package deviceauth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRequestCodeAndPollSuccess(t *testing.T) {
	var polls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/api/auth/device/code"):
			_ = r.ParseForm()
			if r.Form.Get("client_id") != ClientID {
				t.Errorf("expected client_id %s, got %s", ClientID, r.Form.Get("client_id"))
			}
			_ = json.NewEncoder(w).Encode(CodeResponse{
				DeviceCode:      "dev-1",
				UserCode:        "ABCD-EFGH",
				VerificationURI: "https://app.example/device",
				ExpiresIn:       60,
				Interval:        1,
			})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/api/auth/device/token"):
			n := polls.Add(1)
			if n < 2 {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(TokenError{
					Code:             "authorization_pending",
					ErrorDescription: "waiting",
				})
				return
			}
			_ = json.NewEncoder(w).Encode(TokenResponse{
				AccessToken: "session-token-xyz",
				TokenType:   "Bearer",
				ExpiresIn:   3600,
				Scope:       "openid",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	c := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
	code, err := c.RequestCode()
	if err != nil {
		t.Fatalf("RequestCode: %v", err)
	}
	if code.UserCode != "ABCD-EFGH" {
		t.Fatalf("user code: %s", code.UserCode)
	}

	tok, err := c.PollToken(code.DeviceCode, 1, 30)
	if err != nil {
		t.Fatalf("PollToken: %v", err)
	}
	if tok.AccessToken != "session-token-xyz" {
		t.Fatalf("access token: %q", tok.AccessToken)
	}
	if polls.Load() < 2 {
		t.Fatalf("expected pending then success, polls=%d", polls.Load())
	}
}

func TestRequestCodeNeverSucceedsWithoutTokens(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"device_code":"","user_code":""}`))
	}))
	t.Cleanup(srv.Close)

	c := &Client{BaseURL: srv.URL + "/v2", HTTP: srv.Client()}
	_, err := c.RequestCode()
	if err == nil {
		t.Fatal("expected error for empty codes")
	}
}

func TestPollAccessDenied(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(TokenError{Code: "access_denied"})
	}))
	t.Cleanup(srv.Close)

	c := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
	_, err := c.PollToken("dev", 1, 5)
	te, ok := err.(*TokenError)
	if !ok || te.Code != "access_denied" {
		t.Fatalf("expected access_denied, got %v", err)
	}
}

func TestOriginStripsV2(t *testing.T) {
	c := &Client{BaseURL: "https://api.revkeen.com/v2"}
	if got := c.origin(); got != "https://api.revkeen.com" {
		t.Fatalf("origin=%s", got)
	}
}

func TestPollRespectsSlowDown(t *testing.T) {
	var polls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := polls.Add(1)
		if n == 1 {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(TokenError{Code: "slow_down"})
			return
		}
		_ = json.NewEncoder(w).Encode(TokenResponse{
			AccessToken: "tok",
			TokenType:   "Bearer",
			ExpiresIn:   60,
		})
	}))
	t.Cleanup(srv.Close)

	start := time.Now()
	c := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
	tok, err := c.PollToken("dev", 1, 60)
	if err != nil {
		t.Fatalf("PollToken: %v", err)
	}
	if tok.AccessToken != "tok" {
		t.Fatalf("token=%q", tok.AccessToken)
	}
	if time.Since(start) < time.Second {
		t.Fatalf("expected slow_down backoff to wait")
	}
}
