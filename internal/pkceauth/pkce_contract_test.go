package pkceauth

// REV-6759 (AUTH3-7) — the interactive PKCE lane's security contract.
//
// This lane already talks to the OAuth token endpoint (`/api/auth/oauth2/token`)
// that REV-6709 will also move the device lane onto, so the properties pinned
// here are the ones the settled Better Auth token contract must keep:
//
//   - a PUBLIC client (`cli_revkeen`, token_endpoint_auth_method: none) never
//     sends a client secret — PKCE is what replaces it;
//   - the loopback redirect binds to 127.0.0.1 only, never 0.0.0.0 and never a
//     routable interface;
//   - `state` is verified before the code is exchanged, and a mismatch aborts;
//   - `offline_access` is requested so refresh material exists (the consumer for
//     it is REV-6759 finding F3 — the CLI never exchanges it today).

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/revkeen/cli/internal/testfixture"
)

func TestPKCEExchangeNeverSendsAClientSecret(t *testing.T) {
	var gotForm url.Values
	var gotAuthz string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotForm = r.Form
		gotAuthz = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"access_token":"rkoa_x","token_type":"Bearer","expires_in":3600}`))
	}))
	t.Cleanup(srv.Close)

	c := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
	if _, err := c.ExchangeCode("code", "http://127.0.0.1:1/callback", "verifier"); err != nil {
		t.Fatalf("ExchangeCode: %v", err)
	}

	if _, ok := gotForm["client_secret"]; ok {
		t.Fatal("a public client must never send client_secret")
	}
	if gotAuthz != "" {
		// HTTP Basic client authentication is the other way a secret leaks in.
		t.Fatalf("a public client must not authenticate to the token endpoint: %q", gotAuthz)
	}
	if gotForm.Get("code_verifier") == "" {
		t.Fatal("PKCE code_verifier is what replaces the client secret; it must be present")
	}
}

func TestAuthorizeURLRequestsOfflineAccessSoRefreshMaterialIsIssued(t *testing.T) {
	c := &Client{BaseURL: "https://api.revkeen.com"}
	pkce, err := GeneratePKCE()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := c.AuthorizeURL("http://127.0.0.1:1/callback", pkce)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	scope := parsed.Query().Get("scope")
	for _, want := range []string{"openid", "offline_access"} {
		if !hasScope(scope, want) {
			t.Fatalf("scope %q is missing %q", scope, want)
		}
	}
}

// hasScope matches an exact space-delimited scope value.
func hasScope(scope, want string) bool {
	for _, s := range strings.Split(scope, " ") {
		if s == want {
			return true
		}
	}
	return false
}

func TestExchangeCodeCapturesTheRefreshToken(t *testing.T) {
	// The refresh token must survive the exchange, otherwise the material a
	// future refresh implementation needs is dropped at the source.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"rkoa_a","token_type":"Bearer",` +
			`"expires_in":3600,"refresh_token":"rkrt_r","scope":"openid offline_access"}`))
	}))
	t.Cleanup(srv.Close)

	c := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
	tok, err := c.ExchangeCode("code", "http://127.0.0.1:1/callback", "verifier")
	if err != nil {
		t.Fatalf("ExchangeCode: %v", err)
	}
	if tok.RefreshToken != "rkrt_r" {
		t.Fatalf("refresh_token = %q, want rkrt_r", tok.RefreshToken)
	}
	if tok.ExpiresIn != 3600 {
		t.Fatalf("expires_in = %d, want 3600", tok.ExpiresIn)
	}
}

func TestLoopbackRedirectBindsToLocalhostOnly(t *testing.T) {
	// A callback listener on 0.0.0.0 would let anything on the network complete
	// another user's login. RFC 8252 requires the loopback interface.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"rkoa_x","token_type":"Bearer","expires_in":60}`))
	}))
	t.Cleanup(srv.Close)

	c := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	redirectCh := make(chan string, 1)
	go func() {
		res, _ := c.Login(ctx, func(authURL string) {
			parsed, err := url.Parse(authURL)
			if err != nil {
				redirectCh <- ""
				return
			}
			redirectCh <- parsed.Query().Get("redirect_uri")
		})
		_ = res
	}()

	select {
	case redirect := <-redirectCh:
		parsed, err := url.Parse(redirect)
		if err != nil {
			t.Fatalf("redirect_uri %q: %v", redirect, err)
		}
		host, _, err := net.SplitHostPort(parsed.Host)
		if err != nil {
			t.Fatalf("redirect host %q: %v", parsed.Host, err)
		}
		if host != "127.0.0.1" {
			t.Fatalf("loopback redirect bound to %q, want 127.0.0.1", host)
		}
		if parsed.Path != CallbackPath {
			t.Fatalf("callback path = %q, want %q", parsed.Path, CallbackPath)
		}
	case <-ctx.Done():
		t.Fatal("Login never produced an authorize URL")
	}
}

func TestCallbackWithAMismatchedStateIsRejectedWithoutExchangingTheCode(t *testing.T) {
	// State is the CSRF binding between the browser round-trip and this process.
	// A mismatch must abort BEFORE the code reaches the token endpoint.
	var exchanges int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		exchanges++
		_, _ = w.Write([]byte(`{"access_token":"` + testfixture.AccessTokenPrefix + `should_not_be_issued"}`))
	}))
	t.Cleanup(srv.Close)

	c := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	type outcome struct {
		res *LoginResult
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		res, err := c.Login(ctx, func(authURL string) {
			parsed, parseErr := url.Parse(authURL)
			if parseErr != nil {
				return
			}
			cb, parseErr := url.Parse(parsed.Query().Get("redirect_uri"))
			if parseErr != nil {
				return
			}
			q := cb.Query()
			q.Set("code", "attacker-code")
			q.Set("state", "not-the-state-we-generated")
			cb.RawQuery = q.Encode()
			resp, getErr := http.Get(cb.String())
			if getErr == nil {
				_ = resp.Body.Close()
			}
		})
		done <- outcome{res, err}
	}()

	select {
	case got := <-done:
		if got.err == nil {
			t.Fatal("a state mismatch must fail the login")
		}
		if !strings.Contains(got.err.Error(), "state mismatch") {
			t.Fatalf("error = %v, want a state-mismatch error", got.err)
		}
		if got.res != nil && got.res.Token != nil {
			t.Fatal("no token may be produced after a state mismatch")
		}
		if exchanges != 0 {
			t.Fatalf("the code was exchanged %d times despite the state mismatch", exchanges)
		}
	case <-ctx.Done():
		t.Fatal("Login did not return after a state mismatch")
	}
}

func TestProviderErrorOnTheCallbackIsSurfacedNotSwallowed(t *testing.T) {
	c := &Client{BaseURL: "https://api.revkeen.com"}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		_, err := c.Login(ctx, func(authURL string) {
			parsed, parseErr := url.Parse(authURL)
			if parseErr != nil {
				return
			}
			cb, parseErr := url.Parse(parsed.Query().Get("redirect_uri"))
			if parseErr != nil {
				return
			}
			q := cb.Query()
			q.Set("error", "access_denied")
			q.Set("error_description", "The user declined the request.")
			cb.RawQuery = q.Encode()
			resp, getErr := http.Get(cb.String())
			if getErr == nil {
				_ = resp.Body.Close()
			}
		})
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("a provider error on the callback must fail the login")
		}
		te, ok := err.(*TokenError)
		if !ok || te.Code != "access_denied" {
			t.Fatalf("error = %v, want a TokenError with code access_denied", err)
		}
	case <-ctx.Done():
		t.Fatal("Login did not return after a callback error")
	}
}
