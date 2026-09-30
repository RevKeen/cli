package pkceauth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestGeneratePKCE_S256(t *testing.T) {
	pkce, err := GeneratePKCE()
	if err != nil {
		t.Fatal(err)
	}
	if pkce.ChallengeMethod != "S256" {
		t.Fatalf("method: %s", pkce.ChallengeMethod)
	}
	sum := sha256.Sum256([]byte(pkce.Verifier))
	want := base64.RawURLEncoding.EncodeToString(sum[:])
	if pkce.Challenge != want {
		t.Fatalf("challenge mismatch")
	}
	if pkce.State == "" || pkce.Verifier == "" {
		t.Fatal("empty state/verifier")
	}
}

func TestAuthorizeURL(t *testing.T) {
	c := &Client{BaseURL: "https://api.revkeen.com/v2"}
	pkce := &PKCE{
		Verifier:        "v",
		Challenge:       "challenge",
		ChallengeMethod: "S256",
		State:           "st",
	}
	u, err := c.AuthorizeURL("http://127.0.0.1:12345/callback", pkce)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(u)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Path != "/api/auth/oauth2/authorize" {
		t.Fatalf("path: %s", parsed.Path)
	}
	q := parsed.Query()
	if q.Get("client_id") != ClientID {
		t.Fatalf("client_id: %s", q.Get("client_id"))
	}
	if q.Get("code_challenge_method") != "S256" {
		t.Fatal("expected S256")
	}
	if q.Get("redirect_uri") != "http://127.0.0.1:12345/callback" {
		t.Fatalf("redirect_uri: %s", q.Get("redirect_uri"))
	}
	if !strings.Contains(q.Get("scope"), "openid") {
		t.Fatal("scope missing openid")
	}
}

func TestExchangeCode(t *testing.T) {
	var gotForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/auth/oauth2/token" {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		gotForm, _ = url.ParseQuery(string(body))
		_, _ = w.Write([]byte(`{"access_token":"rkoa_test","token_type":"Bearer","expires_in":3600,"refresh_token":"rr"}`))
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
	tok, err := c.ExchangeCode("authcode", "http://127.0.0.1:9/callback", "verifier")
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "rkoa_test" {
		t.Fatalf("token: %s", tok.AccessToken)
	}
	if gotForm.Get("grant_type") != "authorization_code" {
		t.Fatalf("grant_type: %s", gotForm.Get("grant_type"))
	}
	if gotForm.Get("code_verifier") != "verifier" {
		t.Fatalf("code_verifier: %s", gotForm.Get("code_verifier"))
	}
}

func TestLogin_RoundTrip(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"rkoa_ok","token_type":"Bearer","expires_in":60}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	authURLCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		select {
		case <-ctx.Done():
			errCh <- ctx.Err()
			return
		case authURL := <-authURLCh:
			u, err := url.Parse(authURL)
			if err != nil {
				errCh <- err
				return
			}
			redir := u.Query().Get("redirect_uri")
			state := u.Query().Get("state")
			cb, err := url.Parse(redir)
			if err != nil {
				errCh <- err
				return
			}
			q := cb.Query()
			q.Set("code", "abc")
			q.Set("state", state)
			cb.RawQuery = q.Encode()
			resp, err := http.Get(cb.String())
			if err != nil {
				errCh <- err
				return
			}
			_ = resp.Body.Close()
			errCh <- nil
		}
	}()

	res, err := c.Login(ctx, func(authURL string) {
		authURLCh <- authURL
	})
	if waitErr := <-errCh; waitErr != nil {
		t.Fatal(waitErr)
	}
	if err != nil {
		t.Fatal(err)
	}
	if res.Token == nil || res.Token.AccessToken != "rkoa_ok" {
		t.Fatalf("unexpected result: %+v", res)
	}
}
