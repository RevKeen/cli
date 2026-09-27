// Package pkceauth implements OAuth 2.1 authorization-code + PKCE (S256)
// against Better Auth's /api/auth/oauth2/* endpoints (REV-2974).
package pkceauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// ClientID is the first-party public CLI client seeded in oauth_client.
	ClientID = "cli_revkeen"

	// CallbackPath is the path registered for loopback redirects.
	CallbackPath = "/callback"

	// cart_keys:write (REV-8228) lets `revkeen cart keys issue` issue Cart keys
	// as the signed-in user; Engine still decides by the member's role. The
	// scope must be registered on cli_revkeen (migration 20260927090000) before
	// a CLI requesting it is released, or login fails with invalid_scope.
	defaultScope = "openid profile email offline_access invoices:read customers:read payments:read orders:read subscriptions:read products:read prices:read analytics:read cart_keys:write"

	authorizePath = "/api/auth/oauth2/authorize"
	tokenPath     = "/api/auth/oauth2/token"
)

// TokenResponse is returned by the token endpoint.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	RefreshToken string `json:"refresh_token"`
}

// TokenError is an OAuth error payload.
type TokenError struct {
	Code             string `json:"error"`
	ErrorDescription string `json:"error_description"`
	HTTPStatus       int    `json:"-"`
}

func (e *TokenError) Error() string {
	if e.ErrorDescription != "" {
		return fmt.Sprintf("%s: %s", e.Code, e.ErrorDescription)
	}
	return e.Code
}

// Client talks to Better Auth OAuth endpoints on the API origin (no /v2).
type Client struct {
	HTTP     *http.Client
	BaseURL  string
	ClientID string
	Scope    string
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (c *Client) clientID() string {
	if c.ClientID != "" {
		return c.ClientID
	}
	return ClientID
}

func (c *Client) scope() string {
	if c.Scope != "" {
		return c.Scope
	}
	return defaultScope
}

func (c *Client) origin() string {
	base := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	for strings.HasSuffix(base, "/v2") {
		base = strings.TrimSuffix(base, "/v2")
		base = strings.TrimRight(base, "/")
	}
	return base
}

// PKCE holds generated verifier/challenge material.
type PKCE struct {
	Verifier        string
	Challenge       string
	ChallengeMethod string
	State           string
}

// GeneratePKCE creates an S256 code_challenge and opaque state.
func GeneratePKCE() (*PKCE, error) {
	verifier, err := randomURLSafe(32)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	state, err := randomURLSafe(16)
	if err != nil {
		return nil, err
	}
	return &PKCE{
		Verifier:        verifier,
		Challenge:       challenge,
		ChallengeMethod: "S256",
		State:           state,
	}, nil
}

func randomURLSafe(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// AuthorizeURL builds the browser authorization URL.
func (c *Client) AuthorizeURL(redirectURI string, pkce *PKCE) (string, error) {
	if pkce == nil || pkce.Challenge == "" || pkce.State == "" {
		return "", fmt.Errorf("pkce challenge and state are required")
	}
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", c.clientID())
	q.Set("redirect_uri", redirectURI)
	q.Set("scope", c.scope())
	q.Set("state", pkce.State)
	q.Set("code_challenge", pkce.Challenge)
	q.Set("code_challenge_method", pkce.ChallengeMethod)
	return c.origin() + authorizePath + "?" + q.Encode(), nil
}

// ExchangeCode swaps an authorization code for tokens.
func (c *Client) ExchangeCode(code, redirectURI, codeVerifier string) (*TokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("client_id", c.clientID())
	form.Set("code_verifier", codeVerifier)

	req, err := http.NewRequest(http.MethodPost, c.origin()+tokenPath, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		var te TokenError
		_ = json.Unmarshal(body, &te)
		te.HTTPStatus = resp.StatusCode
		if te.Code == "" {
			te.Code = "invalid_grant"
			te.ErrorDescription = strings.TrimSpace(string(body))
		}
		return nil, &te
	}

	var out TokenResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("invalid token response: %w", err)
	}
	if out.AccessToken == "" {
		return nil, fmt.Errorf("token response missing access_token")
	}
	return &out, nil
}

// LoginResult is returned by Login.
type LoginResult struct {
	Token       *TokenResponse
	AuthorizeURL string
	RedirectURI string
}

// Login runs the full PKCE browser flow: listen → authorize URL → exchange.
// onAuthorizeURL is always invoked with the authorize URL once the loopback
// listener is ready (callers print and optionally open a browser).
func (c *Client) Login(ctx context.Context, onAuthorizeURL func(string)) (*LoginResult, error) {
	pkce, err := GeneratePKCE()
	if err != nil {
		return nil, err
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to bind loopback callback listener: %w", err)
	}
	addr := ln.Addr().(*net.TCPAddr)
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d%s", addr.Port, CallbackPath)

	authURL, err := c.AuthorizeURL(redirectURI, pkce)
	if err != nil {
		_ = ln.Close()
		return nil, err
	}

	type listenResult struct {
		code string
		err  error
	}
	listenCh := make(chan listenResult, 1)

	mux := http.NewServeMux()
	mux.HandleFunc(CallbackPath, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if errParam := q.Get("error"); errParam != "" {
			desc := q.Get("error_description")
			_, _ = w.Write([]byte("Authentication failed. You can close this window."))
			listenCh <- listenResult{err: &TokenError{Code: errParam, ErrorDescription: desc}}
			return
		}
		if q.Get("state") != pkce.State {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			listenCh <- listenResult{err: fmt.Errorf("OAuth state mismatch")}
			return
		}
		code := q.Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			listenCh <- listenResult{err: fmt.Errorf("OAuth callback missing code")}
			return
		}
		_, _ = w.Write([]byte("Authentication successful. You can close this window and return to the CLI."))
		listenCh <- listenResult{code: code}
	})

	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	if onAuthorizeURL != nil {
		onAuthorizeURL(authURL)
	}

	select {
	case <-ctx.Done():
		return &LoginResult{AuthorizeURL: authURL, RedirectURI: redirectURI}, ctx.Err()
	case res := <-listenCh:
		if res.err != nil {
			return &LoginResult{AuthorizeURL: authURL, RedirectURI: redirectURI}, res.err
		}
		tok, err := c.ExchangeCode(res.code, redirectURI, pkce.Verifier)
		if err != nil {
			return &LoginResult{AuthorizeURL: authURL, RedirectURI: redirectURI}, err
		}
		return &LoginResult{Token: tok, AuthorizeURL: authURL, RedirectURI: redirectURI}, nil
	}
}
