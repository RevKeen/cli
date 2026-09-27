package cmd

// REV-8228: `revkeen cart keys issue` authenticates as the signed-in user.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	revkeen "github.com/revkeen/sdk-go"
	"github.com/spf13/cobra"

	"github.com/revkeen/cli/internal/config"
	"github.com/revkeen/cli/internal/testfixture"
)

func ensurePayload() map[string]any {
	status := func(kind string) map[string]any {
		return map[string]any{
			"kind": kind, "present": true, "active": true, "id": nil,
			"scopes": []string{}, "created_at": nil, "last_used_at": nil, "revoked_at": nil,
		}
	}
	return map[string]any{
		"success": true,
		"data": map[string]any{
			"publishable": status("publishable"), "secret": status("secret"),
			"ready": true, "created": []string{},
		},
	}
}

// keysRoot runs `cart keys issue` with the given client factory.
func keysRoot(t *testing.T, factory func(*cobra.Command) (*revkeen.APIClient, cartKeysCredential, error)) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	original := newCartKeysSDKClient
	newCartKeysSDKClient = factory
	t.Cleanup(func() { newCartKeysSDKClient = original })

	root := &cobra.Command{Use: "revkeen"}
	root.PersistentFlags().StringP("output", "o", "table", "")
	root.PersistentFlags().String("api-key", "", "")
	root.PersistentFlags().Bool("json", false, "")
	root.PersistentFlags().Bool("table", false, "")
	root.AddCommand(newCartCmd())
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(errOut)
	root.SetArgs([]string{"cart", "keys", "issue", "--table"})
	return root, out, errOut
}

func oauthFactory(serverURL string) func(*cobra.Command) (*revkeen.APIClient, cartKeysCredential, error) {
	return func(*cobra.Command) (*revkeen.APIClient, cartKeysCredential, error) {
		client, err := newOAuthSDKClient(testfixture.AccessTokenPrefix+"user_token", serverURL)
		return client, cartKeysCredentialOAuth, err
	}
}

func apiKeyFactory(serverURL string) func(*cobra.Command) (*revkeen.APIClient, cartKeysCredential, error) {
	return func(*cobra.Command) (*revkeen.APIClient, cartKeysCredential, error) {
		client, err := revkeen.NewClientWithCustomBaseURL("rk_sandbox_test", serverURL, revkeen.WithMaxAttempts(1))
		return client, cartKeysCredentialAPIKey, err
	}
}

func TestCartKeysIssueSendsTheOAuthTokenAsBearer(t *testing.T) {
	var gotAuth, gotAPIKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotAPIKey = r.Header.Get("Authorization"), r.Header.Get("x-api-key")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ensurePayload())
	}))
	defer server.Close()

	root, _, errOut := keysRoot(t, oauthFactory(server.URL))
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotAuth != "Bearer "+testfixture.AccessTokenPrefix+"user_token" {
		t.Fatalf("Authorization = %q, want the OAuth bearer", gotAuth)
	}
	if gotAPIKey != "" {
		t.Fatalf("x-api-key must not be sent with OAuth, got %q", gotAPIKey)
	}
	if strings.Contains(errOut.String(), "being retired") {
		t.Fatalf("no API-key warning expected when signed in:\n%s", errOut)
	}
}

func TestCartKeysIssueWarnsWhenOnlyAnAPIKeyIsConfigured(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ensurePayload())
	}))
	defer server.Close()

	root, _, errOut := keysRoot(t, apiKeyFactory(server.URL))
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(errOut.String(), "revkeen login") || !strings.Contains(errOut.String(), "being retired") {
		t.Fatalf("expected the API-key retirement warning, got:\n%s", errOut)
	}
}

func forbidden(reason string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"type": "authorization_error", "code": "insufficient_permissions",
				"message": "denied", "details": map[string]any{"reason": reason},
			},
		})
	}
}

func TestCartKeysIssueExplainsEachDenial(t *testing.T) {
	cases := []struct {
		name    string
		factory func(string) func(*cobra.Command) (*revkeen.APIClient, cartKeysCredential, error)
		status  http.HandlerFunc
		want    string
	}{
		{"API key refused", apiKeyFactory, forbidden(""), "API keys can no longer issue Cart keys"},
		{"role denied", oauthFactory, forbidden("role_denied"), "ask an owner, admin or developer"},
		{"scope missing", oauthFactory, forbidden("scope_insufficient"), "run `revkeen login` again"},
		{"token expired", oauthFactory, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"type":"authentication_error","code":"unauthorized","message":"no"}}`))
		}, "sign-in has expired"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(tc.status)
			defer server.Close()
			root, _, _ := keysRoot(t, tc.factory(server.URL))
			err := root.Execute()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestResolveAccessTokenOnlyInOAuthMode(t *testing.T) {
	oauth := &config.Config{Auth: config.AuthConfig{Mode: "oauth", OAuth: config.OAuthConfig{AccessToken: " rkoa_x "}}}
	if got := resolveAccessToken(oauth); got != "rkoa_x" {
		t.Fatalf("oauth mode: got %q", got)
	}
	apiKeyMode := &config.Config{Auth: config.AuthConfig{Mode: "api_key", OAuth: config.OAuthConfig{AccessToken: "rkoa_stale"}}}
	if got := resolveAccessToken(apiKeyMode); got != "" {
		t.Fatalf("api_key mode must not use a stale OAuth token, got %q", got)
	}
	if resolveAccessToken(nil) != "" {
		t.Fatal("nil config")
	}
}

func TestNewOAuthSDKClientRefusesAnEmptyToken(t *testing.T) {
	if _, err := newOAuthSDKClient("", "https://api.example.test/v2"); err == nil || !strings.Contains(err.Error(), "revkeen login") {
		t.Fatalf("want sign-in error, got %v", err)
	}
}
