package apierr_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/revkeen/cli/internal/apierr"
	"github.com/revkeen/cli/internal/testfixture"
)

// The fixtures below are the canonical envelopes produced by
// apps/engine-api/src/http/routes/v2/errors.ts (REV-6760):
// respondAuthenticationError, respondAuthorizationError, respondRateLimitError.
// They are transcribed rather than imported because the CLI is a separate Go
// module that ships to merchants independently of Engine API — the point of
// these tests is that the wire contract, not a shared type, is what binds them.

const (
	canonical401 = `{"error":{"type":"authentication_error","code":"invalid_api_key",` +
		`"message":"Authentication required","request_id":"req_abc123"}}`
	canonical403 = `{"error":{"type":"authorization_error","code":"insufficient_permissions",` +
		`"message":"Forbidden","request_id":"req_def456"}}`
	canonical429 = `{"error":{"type":"rate_limit_error","code":"rate_limit_exceeded",` +
		`"message":"Rate limit exceeded. Retry after the period in Retry-After.",` +
		`"request_id":"req_ghi789","details":{"retry_after":60}}}`
)

func TestCanonical401IsAuthenticationAndCarriesTheBearerChallenge(t *testing.T) {
	h := http.Header{}
	h.Set("WWW-Authenticate", `Bearer realm="revkeen-api"`)

	e := apierr.Parse(http.StatusUnauthorized, h, []byte(canonical401))

	if e.Class != apierr.ClassAuthentication {
		t.Fatalf("class = %q, want %q", e.Class, apierr.ClassAuthentication)
	}
	if e.Type != apierr.TypeAuthentication {
		t.Fatalf("type = %q, want %q", e.Type, apierr.TypeAuthentication)
	}
	if e.Code != "invalid_api_key" {
		t.Fatalf("code = %q, want invalid_api_key", e.Code)
	}
	if e.RequestID != "req_abc123" {
		t.Fatalf("request_id = %q", e.RequestID)
	}
	if e.Challenge != `Bearer realm="revkeen-api"` {
		t.Fatalf("challenge = %q", e.Challenge)
	}
	if !e.IsAuth() {
		t.Fatal("401 must report IsAuth")
	}
}

func TestCanonical403IsAuthorizationNotAuthentication(t *testing.T) {
	// REV-6760 retyped 403 from authentication_error to authorization_error so a
	// consumer can branch on 401-vs-403 from the body alone. The CLI must not
	// collapse them back together: a 403 means "log in again" is wrong advice.
	e := apierr.Parse(http.StatusForbidden, http.Header{}, []byte(canonical403))

	if e.Class != apierr.ClassAuthorization {
		t.Fatalf("class = %q, want %q", e.Class, apierr.ClassAuthorization)
	}
	if e.Type != apierr.TypeAuthorization {
		t.Fatalf("type = %q, want %q", e.Type, apierr.TypeAuthorization)
	}
	if e.Code != "insufficient_permissions" {
		t.Fatalf("code = %q", e.Code)
	}
	if strings.Contains(e.Guidance(), "revkeen login") {
		t.Fatalf("403 guidance must not tell the user to re-authenticate: %q", e.Guidance())
	}
}

func TestCanonical429ReadsRetryAfterHeaderAndRateLimitTrio(t *testing.T) {
	h := http.Header{}
	h.Set("Retry-After", "60")
	h.Set("X-RateLimit-Limit", "1000")
	h.Set("X-RateLimit-Remaining", "0")
	// Contract note: Reset is UNIX SECONDS, not epoch milliseconds.
	h.Set("X-RateLimit-Reset", "1787000000")

	e := apierr.Parse(http.StatusTooManyRequests, h, []byte(canonical429))

	if e.Class != apierr.ClassRateLimit {
		t.Fatalf("class = %q, want %q", e.Class, apierr.ClassRateLimit)
	}
	if e.Code != "rate_limit_exceeded" {
		t.Fatalf("code = %q", e.Code)
	}
	if e.RetryAfter != 60 {
		t.Fatalf("retry_after = %d, want 60", e.RetryAfter)
	}
	if e.Limit != "1000" || e.Remaining != "0" || e.Reset != "1787000000" {
		t.Fatalf("rate-limit trio = %q/%q/%q", e.Limit, e.Remaining, e.Reset)
	}
	if !strings.Contains(e.Guidance(), "60s") {
		t.Fatalf("429 guidance must surface the retry delay: %q", e.Guidance())
	}
}

func TestRetryAfterFallsBackToDetailsWhenHeaderIsAbsent(t *testing.T) {
	e := apierr.Parse(http.StatusTooManyRequests, http.Header{}, []byte(canonical429))
	if e.RetryAfter != 60 {
		t.Fatalf("retry_after = %d, want 60 from details.retry_after", e.RetryAfter)
	}
}

func TestRetryAfterHeaderWinsOverDetails(t *testing.T) {
	// An intermediary honours the header, so the header is what the CLI must
	// report when the two disagree.
	h := http.Header{}
	h.Set("Retry-After", "5")
	e := apierr.Parse(http.StatusTooManyRequests, h, []byte(canonical429))
	if e.RetryAfter != 5 {
		t.Fatalf("retry_after = %d, want 5 (header wins over details.retry_after=60)", e.RetryAfter)
	}
}

func TestClassIsDerivedFromStatusNotFromTheBody(t *testing.T) {
	// A body that disagrees with the status must not be able to relabel the
	// failure — otherwise a mislabelled 403 would be treated as retryable.
	e := apierr.Parse(http.StatusForbidden, http.Header{}, []byte(canonical429))
	if e.Class != apierr.ClassAuthorization {
		t.Fatalf("class = %q, want %q for HTTP 403 regardless of body type", e.Class, apierr.ClassAuthorization)
	}
}

func TestUnparseableAndEmptyBodiesAreStillClassified(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"empty", ""},
		{"whitespace", "   \n"},
		{"html", "<html><body>502 Bad Gateway</body></html>"},
		{"truncated", `{"error":{"type":"authentication_er`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := apierr.Parse(http.StatusUnauthorized, http.Header{}, []byte(tc.body))
			if e.Class != apierr.ClassAuthentication {
				t.Fatalf("class = %q, want %q", e.Class, apierr.ClassAuthentication)
			}
			if !strings.Contains(e.Error(), "HTTP 401") {
				t.Fatalf("rendered error must still name the status: %q", e.Error())
			}
		})
	}
}

func TestLegacyFlatEnvelopeIsNormalisedToTheCanonicalCode(t *testing.T) {
	// Pre-REV-6760 routes still emit {"error":"CART_DISABLED","message":...}.
	// Both shapes must yield the same lower_snake code so callers switch once.
	flat := apierr.Parse(http.StatusForbidden, http.Header{},
		[]byte(`{"error":"CART_DISABLED","message":"Cart is not enabled for this merchant."}`))
	nested := apierr.Parse(http.StatusForbidden, http.Header{},
		[]byte(`{"error":{"type":"authorization_error","code":"cart_disabled","message":"Cart disabled"}}`))

	if flat.Code != "cart_disabled" {
		t.Fatalf("legacy flat code = %q, want cart_disabled", flat.Code)
	}
	if flat.Code != nested.Code {
		t.Fatalf("legacy %q and canonical %q must normalise to the same code", flat.Code, nested.Code)
	}
	if flat.Message == "" {
		t.Fatal("legacy envelope message was dropped")
	}
}

func TestParseToleratesNilHeader(t *testing.T) {
	// The generated Go SDK's RequestError exposes a body but no headers.
	e := apierr.Parse(http.StatusUnauthorized, nil, []byte(canonical401))
	if e.Code != "invalid_api_key" {
		t.Fatalf("code = %q", e.Code)
	}
	if e.Challenge != "" {
		t.Fatalf("challenge = %q, want empty with a nil header", e.Challenge)
	}
}

func TestRenderedErrorNeverContainsThePresentedCredential(t *testing.T) {
	// The decoder is given only the server's response. If a rendered string ever
	// contained the caller's key, it could only be because the CLI put it there.
	const secret = testfixture.LiveKeyPrefix + "SUPERSECRETVALUE"
	e := apierr.Parse(http.StatusUnauthorized, http.Header{}, []byte(canonical401))
	for _, s := range []string{e.Error(), e.Summary(), e.Guidance()} {
		if strings.Contains(s, secret) || strings.Contains(s, "rk_live_") || strings.Contains(s, "rkoa_") {
			t.Fatalf("rendered output leaked credential material: %q", s)
		}
	}
}

func TestNon401403429StatusesAreClassOther(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusNotFound, http.StatusConflict, http.StatusInternalServerError} {
		e := apierr.Parse(status, http.Header{}, []byte(`{"error":{"type":"api_error","code":"internal_error","message":"boom"}}`))
		if e.Class != apierr.ClassOther {
			t.Fatalf("status %d class = %q, want %q", status, e.Class, apierr.ClassOther)
		}
		if e.IsAuth() {
			t.Fatalf("status %d must not report IsAuth", status)
		}
		if e.Guidance() != "" {
			t.Fatalf("status %d must not emit auth guidance: %q", status, e.Guidance())
		}
	}
}

func TestOffEnumAuthCodesAreStillClassifiedAndRendered(t *testing.T) {
	// Not every /v2 surface has been converted. apps/engine-api/src/http/
	// middlewares/merchant-api-key-only.ts (mounted on /v2/credit-notes, which
	// the CLI reaches through the generic OpenAPI handler) emits the nested
	// envelope but with codes that are NOT in the REV-6760 enum
	// (`authentication_required`, `merchant_api_key_required`) and sets no
	// WWW-Authenticate. Deriving class from status is what keeps the CLI correct
	// on those surfaces instead of falling back to "unknown error".
	for _, code := range []string{"authentication_required", "merchant_api_key_required"} {
		body := `{"error":{"type":"authentication_error","code":"` + code + `","message":"A merchant API key is required."}}`
		e := apierr.Parse(http.StatusUnauthorized, http.Header{}, []byte(body))
		if e.Class != apierr.ClassAuthentication {
			t.Fatalf("code %q class = %q, want %q", code, e.Class, apierr.ClassAuthentication)
		}
		if e.Code != code {
			t.Fatalf("code = %q, want %q", e.Code, code)
		}
		if e.Challenge != "" {
			t.Fatalf("challenge = %q, want empty (this surface sends none)", e.Challenge)
		}
		if !strings.Contains(e.Guidance(), "revkeen login") {
			t.Fatalf("guidance = %q, want re-authentication advice", e.Guidance())
		}
	}
}

func TestNonPositiveRetryAfterIsIgnored(t *testing.T) {
	for _, raw := range []string{"0", "-5", "soon", ""} {
		h := http.Header{}
		if raw != "" {
			h.Set("Retry-After", raw)
		}
		e := apierr.Parse(http.StatusTooManyRequests, h,
			[]byte(`{"error":{"type":"rate_limit_error","code":"rate_limit_exceeded","message":"slow down"}}`))
		if e.RetryAfter != 0 {
			t.Fatalf("Retry-After %q parsed to %d, want 0", raw, e.RetryAfter)
		}
		if !strings.Contains(e.Guidance(), "retry later") {
			t.Fatalf("guidance without a delay = %q", e.Guidance())
		}
	}
}
