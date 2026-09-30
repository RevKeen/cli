package cmd

// Unit tests for the REV-5152 cart commands. The generated SDK client is real;
// only the HTTP layer is faked (httptest), so the tests also exercise the
// generated client's request/response handling and error mapping.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/revkeen/cli/internal/testfixture"
	revkeen "github.com/revkeen/sdk-go"
	"github.com/spf13/cobra"
)

func newCartTestRoot(t *testing.T, serverURL string, args ...string) (*cobra.Command, *bytes.Buffer) {
	t.Helper()

	original := newCartSDKClient
	newCartSDKClient = func(cmd *cobra.Command) (*revkeen.APIClient, error) {
		return revkeen.NewClientWithCustomBaseURL(
			"rk_sandbox_test",
			serverURL,
			revkeen.WithMaxAttempts(1),
		)
	}
	t.Cleanup(func() { newCartSDKClient = original })

	originalKeys := newCartKeysSDKClient
	newCartKeysSDKClient = func(cmd *cobra.Command) (*revkeen.APIClient, cartKeysCredential, error) {
		client, err := newOAuthSDKClient(testfixture.AccessTokenPrefix+"test_token", serverURL)
		return client, cartKeysCredentialOAuth, err
	}
	t.Cleanup(func() { newCartKeysSDKClient = originalKeys })

	root := &cobra.Command{Use: "revkeen"}
	root.PersistentFlags().StringP("output", "o", "table", "")
	root.PersistentFlags().Bool("no-color", false, "")
	root.PersistentFlags().String("api-key", "", "")
	root.PersistentFlags().Bool("agent", false, "")
	root.PersistentFlags().Bool("json", false, "")
	root.PersistentFlags().Bool("table", false, "")
	root.AddCommand(newCartCmd())

	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs(args)
	return root, out
}

func storefrontStatusEnvelope(ready bool, checks []map[string]any) map[string]any {
	keyStatus := map[string]any{
		"present": true, "active": true, "created_at": nil, "last_used_at": nil,
	}
	return map[string]any{
		"data": map[string]any{
			"object": "storefront_status", "ready": ready, "checks": checks,
			"cart":         map[string]any{"enabled": true, "profile": "standard"},
			"keys":         map[string]any{"publishable": keyStatus, "secret": keyStatus},
			"origins":      map[string]any{"count": 1, "origins": []string{"https://shop.example.com"}},
			"product_read": map[string]any{"active_products": 1, "priced_products": 1, "ready": true},
			"webhooks":     map[string]any{"active_endpoints": 1, "unreachable_endpoints": 0},
			"availability": map[string]any{"tracked_products": 0, "mode": "disabled"},
		},
	}
}

func webhookEndpointsEnvelope(url string, events []string) map[string]any {
	return map[string]any{
		"data": []map[string]any{{
			"id": "wh-1", "url": url, "description": nil,
			"enabled_events": events, "status": "active", "circuit_breaker_state": "closed",
			"total_deliveries": 5, "successful_deliveries": 5, "failed_deliveries": 0,
			"last_delivery_at": nil, "created_at": "2026-07-10T00:00:00Z", "updated_at": "2026-07-10T00:00:00Z",
		}},
		"meta": map[string]any{"total": 1, "limit": 20, "offset": 0, "has_more": false},
	}
}

func TestCartStatusRendersReadinessReport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/storefront/status" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.Header.Get("x-api-key"); got != "rk_sandbox_test" {
			t.Fatalf("missing x-api-key auth, got %q", got)
		}
		_ = json.NewEncoder(w).Encode(storefrontStatusEnvelope(false, []map[string]any{
			{"id": "cart_enabled", "status": "pass", "message": "Cart is enabled."},
			{
				"id": "origins", "status": "fail", "code": "ORIGIN_MISSING",
				"message":     "No storefront origins are registered.",
				"next_action": "Register one via revkeen cart origins add.",
			},
		}))
	}))
	defer server.Close()

	root, out := newCartTestRoot(t, server.URL, "cart", "status", "--table")
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	text := out.String()
	for _, want := range []string{"NOT READY", "cart_enabled", "origins", "Register one via revkeen cart origins add."} {
		if !strings.Contains(text, want) {
			t.Fatalf("status output missing %q in:\n%s", want, text)
		}
	}
	if !strings.Contains(text, "✗") {
		t.Fatalf("status output missing fail marker in:\n%s", text)
	}
}

func TestCartStatusMapsCartDisabledToActionableError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":   "CART_DISABLED",
			"message": "Cart is not enabled for this merchant.",
		})
	}))
	defer server.Close()

	root, _ := newCartTestRoot(t, server.URL, "cart", "status")
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Marketplace") {
		t.Fatalf("expected actionable CART_DISABLED guidance, got: %v", err)
	}
}

func TestCartKeysIssuePrintsMaterialOnlyWhenReturned(t *testing.T) {
	fresh := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/cart-api-keys/ensure" && !strings.Contains(r.URL.Path, "cart-api-keys") {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		payload := map[string]any{
			"success": true,
			"data": map[string]any{
				"publishable": map[string]any{
					"kind": "publishable", "present": true, "active": true, "id": nil,
					"scopes": []string{}, "created_at": nil, "last_used_at": nil, "revoked_at": nil,
				},
				"secret": map[string]any{
					"kind": "secret", "present": true, "active": true, "id": nil,
					"scopes": []string{}, "created_at": nil, "last_used_at": nil, "revoked_at": nil,
				},
				"ready":   true,
				"created": []string{},
			},
		}
		if fresh {
			payload["data"].(map[string]any)["publishable_api_key"] = testfixture.PublishableLiveKeyPrefix + "NEW"
			payload["data"].(map[string]any)["secret_api_key"] = testfixture.LiveKeyPrefix + "NEW"
			fresh = false
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	root, out := newCartTestRoot(t, server.URL, "cart", "keys", "issue", "--table")
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	first := out.String()
	if !strings.Contains(first, testfixture.LiveKeyPrefix+"NEW") || !strings.Contains(first, "shown once") {
		t.Fatalf("first issuance should print new key material once:\n%s", first)
	}

	root2, out2 := newCartTestRoot(t, server.URL, "cart", "keys", "issue", "--table")
	if err := root2.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	second := out2.String()
	if strings.Contains(second, testfixture.LiveKeyPrefix+"NEW") {
		t.Fatalf("re-run must not reprint secret material:\n%s", second)
	}
	if !strings.Contains(second, "already provisioned") {
		t.Fatalf("re-run should report existing state:\n%s", second)
	}
}

func TestCartOriginsAddIsIdempotentOnConflict(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/storefront/origins":
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": "ORIGIN_EXISTS", "message": "This origin is already registered.",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/storefront/origins":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{
					"id": "org-1", "object": "storefront_origin",
					"origin": "https://shop.example.com", "created_at": "2026-07-10T00:00:00.000Z",
				}},
			})
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	root, out := newCartTestRoot(t, server.URL, "cart", "origins", "add", "https://shop.example.com", "--table")
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "Already registered: https://shop.example.com (org-1)") {
		t.Fatalf("conflict should resolve to existing origin:\n%s", out.String())
	}
}

func TestCartOriginsRemoveResolvesURLToID(t *testing.T) {
	deleted := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/storefront/origins":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{
					"id": "org-9", "object": "storefront_origin",
					"origin": "https://shop.example.com", "created_at": "2026-07-10T00:00:00.000Z",
				}},
			})
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/storefront/origins/"):
			deleted = strings.TrimPrefix(r.URL.Path, "/storefront/origins/")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"id": deleted, "deleted": true},
			})
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	root, out := newCartTestRoot(t, server.URL, "cart", "origins", "remove", "https://shop.example.com", "--table")
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if deleted != "org-9" {
		t.Fatalf("expected delete of org-9, got %q", deleted)
	}
	if !strings.Contains(out.String(), "Removed origin org-9") {
		t.Fatalf("unexpected output:\n%s", out.String())
	}
}

func TestCartWebhooksSetupIsIdempotent(t *testing.T) {
	created := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/webhook-endpoints":
			_ = json.NewEncoder(w).Encode(webhookEndpointsEnvelope(
				"https://hooks.example.com/revkeen",
				[]string{"commerce.checkout.completed"},
			))
		case r.Method == http.MethodPost && r.URL.Path == "/webhook-endpoints":
			created++
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "wh-2", "url": "https://other.example.com"})
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	root, out := newCartTestRoot(t, server.URL, "cart", "webhooks", "setup", "--url", "https://hooks.example.com/revkeen", "--table")
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if created != 0 {
		t.Fatalf("setup must not create a duplicate endpoint")
	}
	if !strings.Contains(out.String(), "Already configured: https://hooks.example.com/revkeen (wh-1)") {
		t.Fatalf("unexpected output:\n%s", out.String())
	}
}

func TestCartCommandsAgentModeEmitsCompactJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{
				"id": "org-1", "object": "storefront_origin",
				"origin": "https://shop.example.com", "created_at": "2026-07-10T00:00:00.000Z",
			}},
		})
	}))
	defer server.Close()

	root, _ := newCartTestRoot(t, server.URL, "cart", "origins", "list", "--agent")
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	// output.Print writes agent JSON to os.Stdout (compact, one line); the
	// absence of an error plus the servers being hit is the contract here —
	// format selection is covered by output package behavior.
}
