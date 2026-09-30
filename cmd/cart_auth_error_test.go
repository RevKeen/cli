package cmd

// REV-6759 (AUTH3-7) — cart command error mapping against the canonical public
// error contract.
//
// `revkeen cart *` talks to /v2/storefront/* and /v2/cart-api-keys, whose 401
// and 403 responses come from authContextMiddleware and requireScopes and are
// therefore the canonical REV-6760 envelope:
//
//	{"error":{"type":"authorization_error","code":"cart_disabled",...}}
//
// Some pre-REV-6760 surfaces still emit the flat legacy envelope with the code
// in SCREAMING_SNAKE:
//
//	{"error":"CART_DISABLED","message":"..."}
//
// Both spellings name the same condition, so both must produce the same
// actionable guidance. Before REV-6759 only the legacy spelling matched, and a
// canonical `cart_disabled` fell through to the generic "you need a secret key"
// advice — which sends the operator to rotate a perfectly good key instead of
// installing the Cart app.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func cartErrorServer(t *testing.T, status int, body map[string]any) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestCartMapsCanonicalCartDisabledToTheSameGuidanceAsLegacy(t *testing.T) {
	canonical := cartErrorServer(t, http.StatusForbidden, map[string]any{
		"error": map[string]any{
			"type":       "authorization_error",
			"code":       "cart_disabled",
			"message":    "Cart is not enabled for this merchant.",
			"request_id": "req_cart",
		},
	})

	root, _ := newCartTestRoot(t, canonical.URL, "cart", "status")
	err := root.Execute()
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "Marketplace") {
		t.Fatalf("canonical cart_disabled must yield install-the-Cart-app guidance, got: %v", err)
	}
	if strings.Contains(err.Error(), "SECRET Cart key") {
		t.Fatalf("canonical cart_disabled must not be mistaken for a wrong-key-kind 403: %v", err)
	}
}

func TestCartMapsCanonicalOriginNotAllowedToTheOriginsGuidance(t *testing.T) {
	srv := cartErrorServer(t, http.StatusForbidden, map[string]any{
		"error": map[string]any{
			"type":    "authorization_error",
			"code":    "origin_not_allowed",
			"message": "Origin is not registered.",
		},
	})

	root, _ := newCartTestRoot(t, srv.URL, "cart", "status")
	err := root.Execute()
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "cart origins add") {
		t.Fatalf("canonical origin_not_allowed must point at `revkeen cart origins add`, got: %v", err)
	}
}

func TestCartKeepsTheGenericScopeGuidanceForOtherForbiddenCodes(t *testing.T) {
	// insufficient_permissions IS the wrong-key-kind / missing-scope case, so the
	// existing guidance is correct there and must survive the normalisation.
	srv := cartErrorServer(t, http.StatusForbidden, map[string]any{
		"error": map[string]any{
			"type":    "authorization_error",
			"code":    "insufficient_permissions",
			"message": "Missing required scope apps:write.",
		},
	})

	root, _ := newCartTestRoot(t, srv.URL, "cart", "status")
	err := root.Execute()
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "apps:write") {
		t.Fatalf("a scope 403 must name the scope it needs, got: %v", err)
	}
}
