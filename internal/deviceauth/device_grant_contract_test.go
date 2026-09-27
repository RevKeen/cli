package deviceauth

// REV-6759 (AUTH3-7) — the device authorization grant as the CLI performs it
// TODAY, pinned so the REV-6709 cutover is a deliberate edit.
//
// REV-6709 is still Backlog. Until it lands, the CLI polls Better Auth's
// FIRST-PARTY device endpoints (`/api/auth/device/code`, `/api/auth/device/token`),
// which mint Better Auth SESSION tokens — not `rkoa_*` OAuth access tokens.
// Engine API says so directly:
//
//	apps/engine-api/src/lib/auth.ts:531-533
//	  "First-party CLI device authorization (RFC 8628) — REV-6698.
//	   Issues Better Auth *session* tokens at /api/auth/device/token (not rkoa_).
//	   Full oauthDeviceAuthorization() (OAuth access tokens) needs BA >= 1.7."
//	apps/engine-api/src/lib/cli-device-auth.ts:3-8
//
// The PKCE lane (apps/cli/internal/pkceauth) already uses the OAuth token
// endpoint `/api/auth/oauth2/token`. REV-6709 moves the device lane onto that
// same endpoint with `grant_type=urn:ietf:params:oauth:grant-type:device_code`.
// When it does, TestDeviceGrantUsesFirstPartyEndpointsUntilREV6709 fails — that
// is the point: the endpoint move must be an intentional, reviewed change and
// not something that drifts in unnoticed.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// recordingDeviceServer captures the path, grant_type and client_id of every
// request so the wire contract can be asserted rather than assumed.
type deviceCall struct {
	path     string
	form     map[string]string
	authzHdr string
}

func recordingDeviceServer(t *testing.T) (*httptest.Server, func() []deviceCall) {
	t.Helper()
	var mu sync.Mutex
	var calls []deviceCall

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		form := map[string]string{}
		for k := range r.Form {
			form[k] = r.Form.Get(k)
		}
		mu.Lock()
		calls = append(calls, deviceCall{
			path:     r.URL.Path,
			form:     form,
			authzHdr: r.Header.Get("Authorization"),
		})
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/auth/device/code":
			_ = json.NewEncoder(w).Encode(CodeResponse{
				DeviceCode:              "dev-code",
				UserCode:                "WXYZ-1234",
				VerificationURI:         "https://app.revkeen.com/device",
				VerificationURIComplete: "https://app.revkeen.com/device?user_code=WXYZ-1234",
				ExpiresIn:               600,
				Interval:                1,
			})
		case "/api/auth/device/token":
			_ = json.NewEncoder(w).Encode(TokenResponse{
				AccessToken: "session-token",
				TokenType:   "Bearer",
				ExpiresIn:   3600,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	return srv, func() []deviceCall {
		mu.Lock()
		defer mu.Unlock()
		out := make([]deviceCall, len(calls))
		copy(out, calls)
		return out
	}
}

func TestDeviceGrantUsesFirstPartyEndpointsUntilREV6709(t *testing.T) {
	srv, calls := recordingDeviceServer(t)
	c := &Client{BaseURL: srv.URL, HTTP: srv.Client()}

	code, err := c.RequestCode()
	if err != nil {
		t.Fatalf("RequestCode: %v", err)
	}
	if _, err := c.PollToken(code.DeviceCode, 1, 30); err != nil {
		t.Fatalf("PollToken: %v", err)
	}

	got := calls()
	if len(got) != 2 {
		t.Fatalf("call count = %d, want 2 (code then token): %+v", len(got), got)
	}
	if got[0].path != "/api/auth/device/code" {
		t.Fatalf("authorization request path = %q; REV-6709 moves this to the OAuth "+
			"device endpoint — update this test deliberately when it does", got[0].path)
	}
	if got[1].path != "/api/auth/device/token" {
		t.Fatalf("token request path = %q; REV-6709 moves this to /api/auth/oauth2/token — "+
			"update this test deliberately when it does", got[1].path)
	}
}

func TestDeviceGrantSendsTheRFC8628GrantTypeAndPublicClientID(t *testing.T) {
	// `cli_revkeen` is a PUBLIC client (token_endpoint_auth_method: none), so the
	// grant carries client_id and no client secret. A CLI shipped to merchants
	// cannot hold a confidential client secret; the RFC 8628 + PKCE design is what
	// replaces it. These stay true across the REV-6709 cutover.
	srv, calls := recordingDeviceServer(t)
	c := &Client{BaseURL: srv.URL, HTTP: srv.Client()}

	code, err := c.RequestCode()
	if err != nil {
		t.Fatalf("RequestCode: %v", err)
	}
	if _, err := c.PollToken(code.DeviceCode, 1, 30); err != nil {
		t.Fatalf("PollToken: %v", err)
	}

	got := calls()
	authorize, token := got[0], got[1]

	if authorize.form["client_id"] != ClientID || token.form["client_id"] != ClientID {
		t.Fatalf("client_id = %q / %q, want %q", authorize.form["client_id"], token.form["client_id"], ClientID)
	}
	if token.form["grant_type"] != GrantType {
		t.Fatalf("grant_type = %q, want %q", token.form["grant_type"], GrantType)
	}
	if GrantType != "urn:ietf:params:oauth:grant-type:device_code" {
		t.Fatalf("GrantType constant drifted from RFC 8628: %q", GrantType)
	}
	for _, call := range got {
		if _, ok := call.form["client_secret"]; ok {
			t.Fatalf("a public client must never send client_secret (%s)", call.path)
		}
		if call.authzHdr != "" {
			t.Fatalf("a public client must not send an Authorization header on %s: %q", call.path, call.authzHdr)
		}
	}
}

func TestDeviceGrantRequestsOfflineAccessSoRefreshMaterialIsIssued(t *testing.T) {
	// The scope must ask for offline_access, otherwise there is nothing for a
	// future refresh implementation (REV-6759 finding F3) to exchange.
	srv, calls := recordingDeviceServer(t)
	c := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
	if _, err := c.RequestCode(); err != nil {
		t.Fatalf("RequestCode: %v", err)
	}

	scope := calls()[0].form["scope"]
	for _, want := range []string{"openid", "offline_access"} {
		if !containsToken(scope, want) {
			t.Fatalf("scope %q is missing %q", scope, want)
		}
	}
}

// containsToken reports whether a space-delimited scope string contains an
// exact scope value (so "invoices:read" cannot satisfy a check for "invoices").
func containsToken(scope, want string) bool {
	start := 0
	for i := 0; i <= len(scope); i++ {
		if i == len(scope) || scope[i] == ' ' {
			if scope[start:i] == want {
				return true
			}
			start = i + 1
		}
	}
	return false
}

func TestDeviceGrantSurfacesTerminalErrorsWithoutRetrying(t *testing.T) {
	// authorization_pending / slow_down are the only retryable codes in RFC 8628.
	// Every other code is terminal — retrying an access_denied or expired_token
	// burns the user's time and can trip server-side rate limits.
	for _, code := range []string{"access_denied", "expired_token", "invalid_grant", "invalid_client"} {
		t.Run(code, func(t *testing.T) {
			var polls int
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				polls++
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(TokenError{Code: code})
			}))
			t.Cleanup(srv.Close)

			c := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
			_, err := c.PollToken("dev", 1, 30)
			te, ok := err.(*TokenError)
			if !ok || te.Code != code {
				t.Fatalf("expected terminal %s, got %v", code, err)
			}
			if polls != 1 {
				t.Fatalf("%s must not be retried, polled %d times", code, polls)
			}
		})
	}
}
