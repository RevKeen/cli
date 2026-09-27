package pkceauth

import (
	"strings"
	"testing"
)

// REV-8228: the CLI login must ask for cart_keys:write so `cart keys issue`
// can issue Cart keys as the signed-in user.
func TestDefaultScopeRequestsCartKeysWrite(t *testing.T) {
	scope := (&Client{}).scope()
	if !strings.Contains(" "+scope+" ", " cart_keys:write ") {
		t.Fatalf("default scope must include cart_keys:write, got %q", scope)
	}
}
