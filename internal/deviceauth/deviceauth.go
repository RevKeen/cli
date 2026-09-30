// Package deviceauth implements RFC 8628 device authorization against
// Better Auth's /api/auth/device/* endpoints (REV-6697 / REV-6701).
package deviceauth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// ClientID is the first-party public CLI client seeded in oauth_client.
	ClientID = "cli_revkeen"

	// GrantType is the RFC 8628 device_code grant.
	GrantType = "urn:ietf:params:oauth:grant-type:device_code"

	defaultScope = "openid profile email offline_access invoices:read customers:read payments:read orders:read subscriptions:read products:read prices:read analytics:read"
)

// CodeResponse is returned by POST /api/auth/device/code.
type CodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// TokenResponse is returned by POST /api/auth/device/token on success.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	RefreshToken string `json:"refresh_token"`
}

// TokenError is returned while polling or on failure.
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

// Client talks to Better Auth device endpoints on the API origin (no /v2).
type Client struct {
	HTTP    *http.Client
	BaseURL string // e.g. https://api.revkeen.com
	ClientID string
	Scope   string
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

// RequestCode starts the device authorization grant.
func (c *Client) RequestCode() (*CodeResponse, error) {
	form := url.Values{}
	form.Set("client_id", c.clientID())
	form.Set("scope", c.scope())

	req, err := http.NewRequest(http.MethodPost, c.origin()+"/api/auth/device/code", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("device code request failed: %w", err)
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
			te.Code = "invalid_request"
			te.ErrorDescription = strings.TrimSpace(string(body))
		}
		return nil, &te
	}

	var out CodeResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("invalid device code response: %w", err)
	}
	if out.DeviceCode == "" || out.UserCode == "" {
		return nil, fmt.Errorf("device code response missing device_code or user_code")
	}
	if out.Interval <= 0 {
		out.Interval = 5
	}
	if out.ExpiresIn <= 0 {
		out.ExpiresIn = 600
	}
	return &out, nil
}

// PollToken polls until a token is issued or the flow fails permanently.
func (c *Client) PollToken(deviceCode string, intervalSec int, expiresInSec int) (*TokenResponse, error) {
	if intervalSec <= 0 {
		intervalSec = 5
	}
	deadline := time.Now().Add(time.Duration(expiresInSec) * time.Second)
	interval := time.Duration(intervalSec) * time.Second

	for {
		if time.Now().After(deadline) {
			return nil, &TokenError{Code: "expired_token", ErrorDescription: "device code expired before approval"}
		}

		tok, err := c.tokenOnce(deviceCode)
		if err == nil {
			if tok.AccessToken == "" {
				return nil, fmt.Errorf("token response missing access_token")
			}
			return tok, nil
		}

		var te *TokenError
		if ok := asTokenError(err, &te); ok {
			switch te.Code {
			case "authorization_pending":
				time.Sleep(interval)
				continue
			case "slow_down":
				interval += 5 * time.Second
				time.Sleep(interval)
				continue
			default:
				return nil, te
			}
		}
		return nil, err
	}
}

func (c *Client) tokenOnce(deviceCode string) (*TokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", GrantType)
	form.Set("device_code", deviceCode)
	form.Set("client_id", c.clientID())

	req, err := http.NewRequest(http.MethodPost, c.origin()+"/api/auth/device/token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("device token poll failed: %w", err)
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
	return &out, nil
}

func asTokenError(err error, target **TokenError) bool {
	if te, ok := err.(*TokenError); ok {
		*target = te
		return true
	}
	return false
}
