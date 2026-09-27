// Package testfixture holds the prefixes of RevKeen credential shapes, split so
// that no line of source contains a literal matching a token pattern.
//
// Tests still need realistic, full-shape values: a redaction test only proves
// something if the value it feeds in looks like a real key. Those values are
// assembled at compile time, e.g. testfixture.LiveKeyPrefix + "abcdefghijklmnop".
// Writing the whole value as one literal instead gets the CLI source rejected by
// GitHub push protection on the public RevKeen/cli mirror (cli/v0.2.0 release,
// run 36336873357), and makes every secret scanner treat test data as a leak.
//
// Import this package from _test.go files only.
package testfixture

const (
	// LiveKeyPrefix is the secret live merchant API key prefix.
	LiveKeyPrefix = "rk_" + "live_"
	// SandboxKeyPrefix is the secret sandbox merchant API key prefix.
	SandboxKeyPrefix = "rk_" + "sandbox_"
	// PublishableLiveKeyPrefix is the publishable live key prefix.
	PublishableLiveKeyPrefix = "rk_" + "pk_" + "live_"
	// AccessTokenPrefix is the OAuth access token prefix.
	AccessTokenPrefix = "rk" + "oa_"
	// RefreshTokenPrefix is the OAuth refresh token prefix.
	RefreshTokenPrefix = "rk" + "rt_"
)
