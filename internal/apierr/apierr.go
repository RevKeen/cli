// Package apierr decodes the RevKeen public API error contract so the CLI can
// tell authentication (401), authorization (403) and rate limiting (429) apart
// and render an actionable next step for each.
//
// The contract is owned by Engine API and is provider-neutral by construction
// (REV-6760, apps/engine-api/src/http/routes/v2/errors.ts):
//
//	401 -> {"error":{"type":"authentication_error","code":<auth code>,"message":...}}
//	       + WWW-Authenticate: Bearer realm="revkeen-api"
//	403 -> {"error":{"type":"authorization_error","code":<authz code>,"message":...}}
//	429 -> {"error":{"type":"rate_limit_error","code":"rate_limit_exceeded",
//	        "message":...,"details":{"retry_after":<seconds>}}}
//	       + Retry-After: <seconds>  (+ optional X-RateLimit-* trio)
//
// Two invariants shape this decoder:
//
//   - Class is derived from the HTTP STATUS, never from the body. A truncated,
//     empty, HTML or otherwise unparseable body must still be classified, and a
//     body that disagrees with the status must not be able to downgrade a 403
//     into something the CLI treats as retryable.
//
//   - Nothing here reads the credential the CLI presented. The rendered text is
//     assembled only from server-supplied fields, so a secret key or bearer
//     token cannot reach stdout/stderr through this path.
//
// The legacy flat envelope ({"error":"SOME_CODE","message":"..."}) is still
// emitted by some pre-REV-6760 routes, so it is accepted and normalised to the
// same shape. Codes are lower-cased: the canonical contract is lower_snake and
// the legacy envelope is SCREAMING_SNAKE for the same conditions.
package apierr

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// Class is the transport-independent classification the CLI branches on.
type Class string

const (
	// ClassAuthentication is HTTP 401 — who is calling could not be established.
	ClassAuthentication Class = "authentication"
	// ClassAuthorization is HTTP 403 — the caller is known but not permitted.
	ClassAuthorization Class = "authorization"
	// ClassRateLimit is HTTP 429 — the caller is known and permitted, but too fast.
	ClassRateLimit Class = "rate_limit"
	// ClassOther is every other >= 400 status.
	ClassOther Class = "other"
)

// Canonical error.type values (REV-6760).
const (
	TypeAuthentication = "authentication_error"
	TypeAuthorization  = "authorization_error"
	TypeRateLimit      = "rate_limit_error"
)

// Error is a decoded public API error response.
type Error struct {
	StatusCode int
	Class      Class

	// Type/Code/Message/RequestID come from the response body when present.
	Type      string
	Code      string
	Message   string
	RequestID string

	// Challenge is the RFC 6750 WWW-Authenticate header sent with every 401.
	Challenge string

	// RetryAfter is whole seconds, from the Retry-After header when present and
	// from details.retry_after otherwise. Zero means "the server did not say".
	RetryAfter int

	// Limit/Remaining/Reset mirror the optional X-RateLimit-* trio. Reset is in
	// UNIX SECONDS per the contract. Empty string means the header was absent.
	Limit     string
	Remaining string
	Reset     string
}

// Parse decodes a >= 400 response. header may be nil (e.g. when only a body is
// available, as with the generated SDK's RequestError).
func Parse(statusCode int, header http.Header, body []byte) *Error {
	e := &Error{
		StatusCode: statusCode,
		Class:      classify(statusCode),
	}

	if header != nil {
		e.Challenge = header.Get("WWW-Authenticate")
		e.Limit = header.Get("X-RateLimit-Limit")
		e.Remaining = header.Get("X-RateLimit-Remaining")
		e.Reset = header.Get("X-RateLimit-Reset")
		if v := strings.TrimSpace(header.Get("Retry-After")); v != "" {
			if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
				e.RetryAfter = secs
			}
		}
	}

	decodeBody(e, body)
	return e
}

// classify maps status to class. Deliberately body-independent: an unparseable
// or lying body must not change how the CLI treats the failure.
func classify(statusCode int) Class {
	switch statusCode {
	case http.StatusUnauthorized:
		return ClassAuthentication
	case http.StatusForbidden:
		return ClassAuthorization
	case http.StatusTooManyRequests:
		return ClassRateLimit
	default:
		return ClassOther
	}
}

func decodeBody(e *Error, body []byte) {
	if len(strings.TrimSpace(string(body))) == 0 {
		return
	}
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return
	}

	// Legacy flat envelope: {"error":"CART_DISABLED","message":"..."}.
	if s, ok := root["error"].(string); ok {
		e.Code = normalizeCode(s)
		e.Message = stringField(root, "message")
		e.RequestID = stringField(root, "request_id")
		return
	}

	nested, ok := root["error"].(map[string]any)
	if !ok {
		return
	}
	e.Type = stringField(nested, "type")
	e.Code = normalizeCode(stringField(nested, "code"))
	e.Message = stringField(nested, "message")
	e.RequestID = stringField(nested, "request_id")

	// details.retry_after is the body-side mirror of the Retry-After header.
	// The header wins when both are present, because it is the one the contract
	// requires and the one an HTTP intermediary would honour.
	if e.RetryAfter == 0 {
		if details, ok := nested["details"].(map[string]any); ok {
			if secs, ok := details["retry_after"].(float64); ok && secs > 0 {
				e.RetryAfter = int(secs)
			}
		}
	}
}

func stringField(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// normalizeCode lower-cases so the canonical lower_snake contract and the
// legacy SCREAMING_SNAKE envelope compare equal for the same condition.
func normalizeCode(code string) string {
	return strings.ToLower(strings.TrimSpace(code))
}

// IsAuth reports whether this is an authentication or authorization failure.
func (e *Error) IsAuth() bool {
	if e == nil {
		return false
	}
	return e.Class == ClassAuthentication || e.Class == ClassAuthorization
}

// Guidance returns a credential-free next action for the caller. It is built
// only from the status class and server-supplied fields.
func (e *Error) Guidance() string {
	if e == nil {
		return ""
	}
	switch e.Class {
	case ClassAuthentication:
		return "authentication failed — run `revkeen login` (or set REVKEEN_API_KEY) and retry"
	case ClassAuthorization:
		return "authorization denied — the credential is recognised but is not permitted to perform this operation; check its scopes"
	case ClassRateLimit:
		if e.RetryAfter > 0 {
			return fmt.Sprintf("rate limited — retry after %ds", e.RetryAfter)
		}
		return "rate limited — retry later"
	default:
		return ""
	}
}

// Summary renders a single credential-free line: status, code, message and
// request id when the server supplied them.
func (e *Error) Summary() string {
	if e == nil {
		return ""
	}
	parts := []string{fmt.Sprintf("HTTP %d", e.StatusCode)}
	if e.Code != "" {
		parts = append(parts, e.Code)
	}
	if e.Message != "" {
		parts = append(parts, e.Message)
	}
	if e.RequestID != "" {
		parts = append(parts, "request_id="+e.RequestID)
	}
	return strings.Join(parts, ": ")
}

// Error implements the error interface so callers can return *Error directly.
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if g := e.Guidance(); g != "" {
		return e.Summary() + " — " + g
	}
	return e.Summary()
}
