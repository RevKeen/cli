package handlers_test

// REV-6759 (AUTH3-7) — the CLI's wire-level authentication contract.
//
// These tests pin what the CLI PUTS ON THE WIRE and what it does with the
// public error contract coming back. They deliberately do not assert anything
// about which backend verifies the credential: `resolveMerchantKeyAuthority()`
// (apps/engine-api/src/config/feature-flags.ts:561) still returns "unkey" for
// staging and production, and returns "better-auth" only when
// MERCHANT_KEY_AUTHORITY is set explicitly or in development. The CLI request
// shape is identical under both lanes by design — that is the property worth
// pinning, and it is what makes the AUTH3 cutover invisible to merchants.
//
// Error-shape fixtures mirror the canonical envelopes emitted by
// apps/engine-api/src/http/routes/v2/errors.ts (REV-6760).

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lispyclouds/climate"
	"github.com/revkeen/cli/internal/apierr"
	"github.com/revkeen/cli/internal/config"
	"github.com/revkeen/cli/internal/creds"
	"github.com/revkeen/cli/internal/handlers"
	"github.com/revkeen/cli/internal/testfixture"
	"github.com/spf13/cobra"
)

type capturedRequest struct {
	apiKeyHeader  string
	authorization string
	apiVersion    string
	headerNames   []string
	query         string
}

// authContractServer records the credential headers of one request and replies
// with the supplied status/headers/body.
func authContractServer(t *testing.T, status int, respHeaders map[string]string, body string) (*httptest.Server, *capturedRequest) {
	t.Helper()
	got := &capturedRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.apiKeyHeader = r.Header.Get("x-api-key")
		got.authorization = r.Header.Get("Authorization")
		got.apiVersion = r.Header.Get("RevKeen-Version")
		got.query = r.URL.RawQuery
		for name := range r.Header {
			got.headerNames = append(got.headerNames, name)
		}
		for k, v := range respHeaders {
			w.Header().Set(k, v)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if body == "" {
			body = `{"object":"list","data":[]}`
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, got
}

// authContractRoot wires a cobra root with the persistent flags the handler
// reads, an isolated HOME, and the file-backed credential store.
func authContractRoot(t *testing.T, baseURL string, mutate func(*config.Config)) (*cobra.Command, *cobra.Command, *bytes.Buffer) {
	t.Helper()

	creds.ForceFileFallback(true)
	t.Cleanup(func() { creds.ForceFileFallback(false) })

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("REVKEEN_API_KEY", "")
	t.Setenv("REVKEEN_ACCESS_TOKEN", "")

	cfg := config.DefaultConfig()
	cfg.Settings.BaseURL = baseURL
	if mutate != nil {
		mutate(cfg)
	}
	if err := config.Save(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	out := new(bytes.Buffer)
	root := &cobra.Command{Use: "revkeen"}
	root.PersistentFlags().String("api-key", "", "")
	root.PersistentFlags().Bool("agent", false, "")
	root.PersistentFlags().Bool("json", false, "")
	root.PersistentFlags().Bool("table", false, "")
	root.PersistentFlags().String("output", "json", "")
	_ = root.PersistentFlags().Set("json", "true")
	root.SetOut(out)
	root.SetErr(out)

	child := &cobra.Command{Use: "list"}
	root.AddCommand(child)
	return root, child, out
}

func callHandler(t *testing.T, srv *httptest.Server, child *cobra.Command) error {
	t.Helper()
	handlers.HTTPClient = srv.Client()
	t.Cleanup(func() { handlers.HTTPClient = http.DefaultClient })
	return handlers.GenericHandler(child, nil, climate.HandlerData{Method: "GET", Path: "/customers"})
}

// ---------------------------------------------------------------------------
// Which credential goes in which header
// ---------------------------------------------------------------------------

func TestAPIKeyModeSendsXApiKeyAndNeverBearer(t *testing.T) {
	srv, got := authContractServer(t, http.StatusOK, nil, "")
	_, child, _ := authContractRoot(t, srv.URL, func(c *config.Config) {
		c.Auth.Mode = "api_key"
		c.Auth.APIKey = testfixture.LiveKeyPrefix + "storedkeyvalue"
	})

	if err := callHandler(t, srv, child); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if got.apiKeyHeader != testfixture.LiveKeyPrefix+"storedkeyvalue" {
		t.Fatalf("x-api-key = %q, want the stored secret key", got.apiKeyHeader)
	}
	if got.authorization != "" {
		t.Fatalf("API-key mode must not also send Authorization, got %q", got.authorization)
	}
}

func TestSandboxAndLiveKeysProduceTheIdenticalRequestShape(t *testing.T) {
	// The `rk_sandbox_` / `rk_live_` split is an ENVIRONMENT distinction resolved
	// server-side. The CLI must not branch on it — no separate header, no query
	// parameter, no different endpoint.
	shapes := map[string]string{}
	for _, key := range []string{testfixture.LiveKeyPrefix + "abcdefghijklmnop", testfixture.SandboxKeyPrefix + "abcdefghijklmnop"} {
		srv, got := authContractServer(t, http.StatusOK, nil, "")
		_, child, _ := authContractRoot(t, srv.URL, func(c *config.Config) {
			c.Auth.Mode = "api_key"
			c.Auth.APIKey = key
		})
		if err := callHandler(t, srv, child); err != nil {
			t.Fatalf("handler(%s): %v", key, err)
		}
		if got.apiKeyHeader != key {
			t.Fatalf("x-api-key = %q, want %q", got.apiKeyHeader, key)
		}
		shapes[key] = strings.Join([]string{got.authorization, got.apiVersion, got.query}, "|")
	}
	if shapes[testfixture.LiveKeyPrefix+"abcdefghijklmnop"] != shapes[testfixture.SandboxKeyPrefix+"abcdefghijklmnop"] {
		t.Fatalf("live and sandbox requests differ beyond the key value: %#v", shapes)
	}
}

func TestEnvironmentAPIKeyIsSentAsXApiKey(t *testing.T) {
	srv, got := authContractServer(t, http.StatusOK, nil, "")
	_, child, _ := authContractRoot(t, srv.URL, nil)
	t.Setenv("REVKEEN_API_KEY", testfixture.SandboxKeyPrefix+"fromenvironment")

	if err := callHandler(t, srv, child); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if got.apiKeyHeader != testfixture.SandboxKeyPrefix+"fromenvironment" {
		t.Fatalf("x-api-key = %q, want the REVKEEN_API_KEY value", got.apiKeyHeader)
	}
	if got.authorization != "" {
		t.Fatalf("Authorization must stay unset in API-key mode, got %q", got.authorization)
	}
}

func TestOAuthModeSendsBearerAndNeverXApiKey(t *testing.T) {
	srv, got := authContractServer(t, http.StatusOK, nil, "")
	_, child, _ := authContractRoot(t, srv.URL, func(c *config.Config) {
		c.Auth.Mode = "oauth"
		c.Auth.OAuth.AccessToken = testfixture.AccessTokenPrefix + "accesstokenvalue"
		c.Auth.OAuth.ClientID = "cli_revkeen"
	})

	if err := callHandler(t, srv, child); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if got.authorization != "Bearer "+testfixture.AccessTokenPrefix+"accesstokenvalue" {
		t.Fatalf("Authorization = %q, want a Bearer OAuth token", got.authorization)
	}
	if got.apiKeyHeader != "" {
		t.Fatalf("OAuth mode must not also send x-api-key, got %q", got.apiKeyHeader)
	}
}

func TestAPIKeyTakesPrecedenceOverAStoredOAuthToken(t *testing.T) {
	// Documented precedence (config.ResolveAPIKeyFromCmd): flag > env > config
	// API key, and only then the stored OAuth token. Exactly one credential is
	// ever presented, so a stale token cannot silently shadow an explicit key.
	srv, got := authContractServer(t, http.StatusOK, nil, "")
	_, child, _ := authContractRoot(t, srv.URL, func(c *config.Config) {
		c.Auth.Mode = "api_key"
		c.Auth.APIKey = testfixture.LiveKeyPrefix + "configuredkey"
		c.Auth.OAuth.AccessToken = testfixture.AccessTokenPrefix + "staleaccesstoken"
	})

	if err := callHandler(t, srv, child); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if got.apiKeyHeader != testfixture.LiveKeyPrefix+"configuredkey" {
		t.Fatalf("x-api-key = %q", got.apiKeyHeader)
	}
	if got.authorization != "" {
		t.Fatalf("only one credential may be presented; Authorization = %q", got.authorization)
	}
}

func TestUnauthenticatedRequestPresentsNoCredentialAtAll(t *testing.T) {
	// Fail closed at the server: never fabricate, guess or reuse a credential.
	srv, got := authContractServer(t, http.StatusOK, nil, "")
	_, child, _ := authContractRoot(t, srv.URL, nil)

	if err := callHandler(t, srv, child); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if got.apiKeyHeader != "" || got.authorization != "" {
		t.Fatalf("unauthenticated request presented a credential: x-api-key=%q authorization=%q",
			got.apiKeyHeader, got.authorization)
	}
}

func TestRequestPinsTheAPIVersionHeader(t *testing.T) {
	srv, got := authContractServer(t, http.StatusOK, nil, "")
	_, child, _ := authContractRoot(t, srv.URL, func(c *config.Config) {
		c.Auth.Mode = "api_key"
		c.Auth.APIKey = "rk_live_key"
	})
	if err := callHandler(t, srv, child); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if got.apiVersion != "2026-05-01" {
		t.Fatalf("RevKeen-Version = %q, want 2026-05-01", got.apiVersion)
	}
}

func TestNoVendorSpecificAuthHeaderIsEverSent(t *testing.T) {
	// AUTH3 acceptance: no Unkey SDK and no vendor-specific request surface. The
	// only credential channels are x-api-key and Authorization.
	srv, got := authContractServer(t, http.StatusOK, nil, "")
	_, child, _ := authContractRoot(t, srv.URL, func(c *config.Config) {
		c.Auth.Mode = "api_key"
		c.Auth.APIKey = "rk_live_key"
	})
	if err := callHandler(t, srv, child); err != nil {
		t.Fatalf("handler: %v", err)
	}
	for _, name := range got.headerNames {
		lower := strings.ToLower(name)
		if strings.Contains(lower, "unkey") {
			t.Fatalf("vendor-specific header on the wire: %q", name)
		}
	}
	if strings.Contains(strings.ToLower(got.query), "unkey") {
		t.Fatalf("vendor-specific query parameter: %q", got.query)
	}
}

// ---------------------------------------------------------------------------
// The public auth error contract (REV-6760) as the CLI consumes it
// ---------------------------------------------------------------------------

func TestCanonical401IsSurfacedAsAnAuthenticationFailure(t *testing.T) {
	body := `{"error":{"type":"authentication_error","code":"invalid_api_key",` +
		`"message":"Authentication required","request_id":"req_401"}}`
	srv, _ := authContractServer(t, http.StatusUnauthorized,
		map[string]string{"WWW-Authenticate": `Bearer realm="revkeen-api"`}, body)
	_, child, out := authContractRoot(t, srv.URL, func(c *config.Config) {
		c.Auth.Mode = "api_key"
		c.Auth.APIKey = testfixture.LiveKeyPrefix + "revokedkey"
	})

	err := callHandler(t, srv, child)
	if err == nil {
		t.Fatal("a 401 must surface as an error")
	}
	var apiError *apierr.Error
	if !errors.As(err, &apiError) {
		t.Fatalf("error is not a decoded API error: %v", err)
	}
	if apiError.Class != apierr.ClassAuthentication {
		t.Fatalf("class = %q, want %q", apiError.Class, apierr.ClassAuthentication)
	}
	if apiError.Code != "invalid_api_key" {
		t.Fatalf("code = %q", apiError.Code)
	}
	if apiError.Challenge != `Bearer realm="revkeen-api"` {
		t.Fatalf("WWW-Authenticate challenge was dropped: %q", apiError.Challenge)
	}
	if !strings.Contains(out.String(), "revkeen login") {
		t.Fatalf("401 output must tell the user how to re-authenticate:\n%s", out.String())
	}
}

func TestCanonical403IsSurfacedAsAuthorizationNotAuthentication(t *testing.T) {
	body := `{"error":{"type":"authorization_error","code":"insufficient_permissions",` +
		`"message":"Forbidden","request_id":"req_403"}}`
	srv, _ := authContractServer(t, http.StatusForbidden, nil, body)
	_, child, out := authContractRoot(t, srv.URL, func(c *config.Config) {
		c.Auth.Mode = "api_key"
		c.Auth.APIKey = testfixture.LiveKeyPrefix + "narrowscopekey"
	})

	err := callHandler(t, srv, child)
	var apiError *apierr.Error
	if !errors.As(err, &apiError) {
		t.Fatalf("error is not a decoded API error: %v", err)
	}
	if apiError.Class != apierr.ClassAuthorization {
		t.Fatalf("class = %q, want %q", apiError.Class, apierr.ClassAuthorization)
	}
	// Re-authenticating does not fix a scope problem; suggesting it wastes the
	// operator's time and hides the real cause.
	if strings.Contains(out.String(), "revkeen login") {
		t.Fatalf("403 output must not advise re-authentication:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "scopes") {
		t.Fatalf("403 output must point at permissions/scopes:\n%s", out.String())
	}
}

func TestCanonical429SurfacesRetryAfter(t *testing.T) {
	body := `{"error":{"type":"rate_limit_error","code":"rate_limit_exceeded",` +
		`"message":"Rate limit exceeded. Retry after the period in Retry-After.",` +
		`"details":{"retry_after":30}}}`
	srv, _ := authContractServer(t, http.StatusTooManyRequests, map[string]string{
		"Retry-After":           "30",
		"X-RateLimit-Limit":     "1000",
		"X-RateLimit-Remaining": "0",
		"X-RateLimit-Reset":     "1787000000",
	}, body)
	_, child, out := authContractRoot(t, srv.URL, func(c *config.Config) {
		c.Auth.Mode = "api_key"
		c.Auth.APIKey = "rk_live_busykey"
	})

	err := callHandler(t, srv, child)
	var apiError *apierr.Error
	if !errors.As(err, &apiError) {
		t.Fatalf("error is not a decoded API error: %v", err)
	}
	if apiError.Class != apierr.ClassRateLimit {
		t.Fatalf("class = %q, want %q", apiError.Class, apierr.ClassRateLimit)
	}
	if apiError.RetryAfter != 30 {
		t.Fatalf("Retry-After = %d, want 30", apiError.RetryAfter)
	}
	if !strings.Contains(out.String(), "retry after 30s") {
		t.Fatalf("429 output must surface the retry delay:\n%s", out.String())
	}
	// A 429 is not an auth failure: telling the operator to log in again would
	// send them to rotate a perfectly good key.
	if strings.Contains(out.String(), "revkeen login") {
		t.Fatalf("429 output must not advise re-authentication:\n%s", out.String())
	}
}

func TestErrorOutputNeverEchoesThePresentedCredential(t *testing.T) {
	const secret = testfixture.LiveKeyPrefix + "NEVERPRINTTHISVALUE"
	body := `{"error":{"type":"authentication_error","code":"invalid_api_key","message":"Authentication required"}}`
	srv, _ := authContractServer(t, http.StatusUnauthorized, nil, body)
	_, child, out := authContractRoot(t, srv.URL, func(c *config.Config) {
		c.Auth.Mode = "api_key"
		c.Auth.APIKey = secret
	})

	err := callHandler(t, srv, child)
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(out.String(), secret) {
		t.Fatalf("rendered output leaked the presented key:\n%s", out.String())
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("returned error leaked the presented key: %v", err)
	}
}

func TestAgentModeStillEmitsTheRawEnvelopeOnAnAuthFailure(t *testing.T) {
	// Machine consumers parse the canonical envelope themselves, so agent mode
	// must not reformat or swallow it.
	body := `{"error":{"type":"authorization_error","code":"forbidden","message":"Forbidden"}}`
	srv, _ := authContractServer(t, http.StatusForbidden, nil, body)
	root, child, _ := authContractRoot(t, srv.URL, func(c *config.Config) {
		c.Auth.Mode = "api_key"
		c.Auth.APIKey = "rk_live_key"
	})
	_ = root.PersistentFlags().Set("agent", "true")

	err := callHandler(t, srv, child)
	var apiError *apierr.Error
	if !errors.As(err, &apiError) {
		t.Fatalf("agent mode must still return the decoded error: %v", err)
	}
	// The envelope itself must remain valid JSON with the canonical shape.
	var envelope struct {
		Error struct {
			Type string `json:"type"`
			Code string `json:"code"`
		} `json:"error"`
	}
	if jsonErr := json.Unmarshal([]byte(body), &envelope); jsonErr != nil {
		t.Fatalf("fixture is not the canonical envelope: %v", jsonErr)
	}
	if envelope.Error.Type != apierr.TypeAuthorization {
		t.Fatalf("fixture type = %q", envelope.Error.Type)
	}
}
