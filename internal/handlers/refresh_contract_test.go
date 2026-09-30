package handlers_test

// REV-6759 (AUTH3-7) — stored-token refresh behaviour on the request path.
//
// FINDING (REV-6759 F3): the CLI acquires refresh material and stores it, but
// nothing ever exchanges it. `RefreshToken` and `ExpiresAt` are written by every
// login path (apps/cli/cmd/auth.go), round-tripped by the credential store
// (apps/cli/internal/config/config.go, apps/cli/internal/creds/creds.go) and
// printed by `auth status` — but there is no `grant_type=refresh_token` call
// anywhere in the CLI, and neither GenericHandler nor `revkeen api` consults
// `expires_at` before presenting the stored access token. An expired token is
// therefore sent as-is and the operator sees a 401 they must fix by logging in
// again.
//
// The two tests below split that into the part that is a GAP and the part that
// is an INVARIANT:
//
//   - the gap test characterises today's behaviour so the change is deliberate:
//     when refresh lands it fails, and must be rewritten rather than quietly
//     deleted;
//   - the invariant test pins a property that must hold before AND after refresh
//     lands — a refresh token is a token-endpoint credential and must never be
//     presented as the API request credential.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lispyclouds/climate"
	"github.com/revkeen/cli/internal/config"
	"github.com/revkeen/cli/internal/handlers"
	"github.com/revkeen/cli/internal/testfixture"
)

type refreshProbe struct {
	hits        atomic.Int32
	paths       []string
	credentials []string
}

// refreshProbeServer records every path the CLI touches, so a token-endpoint
// call would be visible rather than inferred.
func refreshProbeServer(t *testing.T) (*httptest.Server, *refreshProbe) {
	t.Helper()
	probe := &refreshProbe{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		probe.hits.Add(1)
		probe.paths = append(probe.paths, r.URL.Path)
		probe.credentials = append(probe.credentials,
			r.Header.Get("Authorization")+"|"+r.Header.Get("x-api-key"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[]}`))
	}))
	t.Cleanup(srv.Close)
	return srv, probe
}

func TestExpiredStoredTokenIsPresentedUnrefreshed_KnownGapREV6759(t *testing.T) {
	srv, probe := refreshProbeServer(t)
	_, child, _ := authContractRoot(t, srv.URL, func(c *config.Config) {
		c.Auth.Mode = "oauth"
		c.Auth.OAuth.AccessToken = testfixture.AccessTokenPrefix + "expiredaccesstoken"
		c.Auth.OAuth.RefreshToken = testfixture.RefreshTokenPrefix + "storedrefreshtoken"
		c.Auth.OAuth.ExpiresAt = time.Now().Add(-2 * time.Hour).Format(time.RFC3339)
		c.Auth.OAuth.ClientID = "cli_revkeen"
	})

	handlers.HTTPClient = srv.Client()
	t.Cleanup(func() { handlers.HTTPClient = http.DefaultClient })
	if err := handlers.GenericHandler(child, nil, climate.HandlerData{Method: "GET", Path: "/customers"}); err != nil {
		t.Fatalf("handler: %v", err)
	}

	if got := probe.hits.Load(); got != 1 {
		t.Fatalf("request count = %d, want 1 — the CLI performs no refresh exchange today", got)
	}
	for _, p := range probe.paths {
		if p == "/api/auth/oauth2/token" {
			t.Fatalf("a token-endpoint call appeared: refresh has landed, so replace this "+
				"characterisation test with a real refresh assertion (paths=%v)", probe.paths)
		}
	}
	if probe.paths[0] != "/v2/customers" {
		t.Fatalf("request path = %q, want /v2/customers", probe.paths[0])
	}
}

func TestStoredRefreshTokenIsNeverPresentedAsTheRequestCredential(t *testing.T) {
	// Invariant, not a gap: a refresh token authenticates to the token endpoint
	// only. Presenting it as the API credential would put a longer-lived secret
	// on every call. This must keep holding after refresh support lands.
	const refresh = testfixture.RefreshTokenPrefix + "MUSTNEVERBESENTASACREDENTIAL"
	srv, probe := refreshProbeServer(t)
	_, child, _ := authContractRoot(t, srv.URL, func(c *config.Config) {
		c.Auth.Mode = "oauth"
		c.Auth.OAuth.AccessToken = testfixture.AccessTokenPrefix + "accesstoken"
		c.Auth.OAuth.RefreshToken = refresh
		c.Auth.OAuth.ExpiresAt = time.Now().Add(time.Hour).Format(time.RFC3339)
	})

	handlers.HTTPClient = srv.Client()
	t.Cleanup(func() { handlers.HTTPClient = http.DefaultClient })
	if err := handlers.GenericHandler(child, nil, climate.HandlerData{Method: "GET", Path: "/customers"}); err != nil {
		t.Fatalf("handler: %v", err)
	}

	for _, presented := range probe.credentials {
		if strings.Contains(presented, refresh) {
			t.Fatalf("the refresh token was presented as a request credential: %q", presented)
		}
	}
}
