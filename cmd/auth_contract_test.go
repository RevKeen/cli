package cmd_test

// REV-6759 (AUTH3-7) — stored-credential contract for the CLI.
//
// Scope: how `revkeen login` classifies what it is given, what ends up on disk,
// and what `revkeen auth status` / `revkeen auth logout` do with it. The
// on-the-wire header contract lives in
// apps/cli/internal/handlers/auth_contract_test.go.
//
// None of this depends on which backend verifies the credential:
// `resolveMerchantKeyAuthority()` (apps/engine-api/src/config/feature-flags.ts:561)
// still returns "unkey" for staging and production, and the CLI stores and
// presents merchant keys identically under either lane.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/revkeen/cli/cmd"
	"github.com/revkeen/cli/internal/config"
	"github.com/revkeen/cli/internal/creds"
	"github.com/revkeen/cli/internal/testfixture"
)

// isolatedHome gives one test its own ~/.revkeen and forces the file-backed
// credential store, so results do not depend on a desktop secret service.
func isolatedHome(t *testing.T) string {
	t.Helper()
	creds.ForceFileFallback(true)
	t.Cleanup(func() { creds.ForceFileFallback(false) })

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("REVKEEN_API_KEY", "")
	t.Setenv("REVKEEN_ACCESS_TOKEN", "")
	return home
}

func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := cmd.NewRootCmd()
	out := new(bytes.Buffer)
	root.SetOut(out)
	root.SetErr(out)
	root.SetIn(strings.NewReader(""))
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

func readIfPresent(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// ---------------------------------------------------------------------------
// `login --with-token`: which prefix means which auth mode
// ---------------------------------------------------------------------------

func TestWithTokenRoutesAnRkKeyToAPIKeyMode(t *testing.T) {
	isolatedHome(t)
	t.Setenv("REVKEEN_ACCESS_TOKEN", testfixture.LiveKeyPrefix+"secretkeyvalue")

	out, err := runCLI(t, "login", "--with-token")
	if err != nil {
		t.Fatalf("login --with-token: %v", err)
	}
	if !strings.Contains(out, "API key saved") {
		t.Fatalf("expected API-key mode, got:\n%s", out)
	}
	cfg := config.Load()
	if cfg.Auth.Mode != "api_key" {
		t.Fatalf("auth mode = %q, want api_key", cfg.Auth.Mode)
	}
	if cfg.Auth.OAuth.AccessToken != "" {
		t.Fatalf("API-key login must clear any OAuth token, got %q", cfg.Auth.OAuth.AccessToken)
	}
}

func TestWithTokenRoutesAnRkoaTokenToOAuthMode(t *testing.T) {
	isolatedHome(t)
	t.Setenv("REVKEEN_ACCESS_TOKEN", testfixture.AccessTokenPrefix+"accesstokenvalue")

	out, err := runCLI(t, "login", "--with-token")
	if err != nil {
		t.Fatalf("login --with-token: %v", err)
	}
	if !strings.Contains(out, "Access token saved") {
		t.Fatalf("expected OAuth mode, got:\n%s", out)
	}
	cfg := config.Load()
	if cfg.Auth.Mode != "oauth" {
		t.Fatalf("auth mode = %q, want oauth", cfg.Auth.Mode)
	}
	if cfg.Auth.APIKey != "" {
		t.Fatalf("OAuth login must clear any API key, got %q", cfg.Auth.APIKey)
	}
}

func TestWithTokenPrefersAccessTokenOverAPIKeyEnvVar(t *testing.T) {
	isolatedHome(t)
	t.Setenv("REVKEEN_ACCESS_TOKEN", testfixture.AccessTokenPrefix+"preferred")
	t.Setenv("REVKEEN_API_KEY", "rk_live_ignored")

	if _, err := runCLI(t, "login", "--with-token"); err != nil {
		t.Fatalf("login --with-token: %v", err)
	}
	cfg := config.Load()
	if cfg.Auth.OAuth.AccessToken != testfixture.AccessTokenPrefix+"preferred" {
		t.Fatalf("stored token = %q, want the REVKEEN_ACCESS_TOKEN value", cfg.Auth.OAuth.AccessToken)
	}
}

func TestWithTokenFallsBackToTheAPIKeyEnvVar(t *testing.T) {
	isolatedHome(t)
	t.Setenv("REVKEEN_API_KEY", "rk_sandbox_fromenv")

	if _, err := runCLI(t, "login", "--with-token"); err != nil {
		t.Fatalf("login --with-token: %v", err)
	}
	cfg := config.Load()
	if cfg.Auth.Mode != "api_key" || cfg.Auth.APIKey != "rk_sandbox_fromenv" {
		t.Fatalf("cfg = mode:%q key:%q", cfg.Auth.Mode, cfg.Auth.APIKey)
	}
}

func TestLoginRejectsMoreThanOneMode(t *testing.T) {
	isolatedHome(t)
	_, err := runCLI(t, "login", "--api-key", "--device")
	if err == nil || !strings.Contains(err.Error(), "only one of") {
		t.Fatalf("expected a single-mode error, got %v", err)
	}
}

func TestNonInteractiveAPIKeyLoginRejectsAnUnknownPrefix(t *testing.T) {
	// AUTH3 acceptance: a wrong/unknown key prefix must fail here, with no
	// vendor-specific fallback verification attempt. Before REV-6759 the
	// non-interactive branch skipped the `rk_` check the interactive form
	// already applied, so a pasted password was stored as an API key.
	isolatedHome(t)

	root := cmd.NewRootCmd()
	out := new(bytes.Buffer)
	root.SetOut(out)
	root.SetErr(out)
	root.SetIn(strings.NewReader("hunter2\n"))
	root.SetArgs([]string{"login", "--api-key"})
	err := root.Execute()

	if err == nil {
		t.Fatalf("a non-rk_ value must be refused, output:\n%s", out.String())
	}
	if !strings.Contains(err.Error(), "rk_") {
		t.Fatalf("error must name the expected prefix, got: %v", err)
	}
	if cfg := config.Load(); cfg.Auth.Mode != "" || cfg.Auth.APIKey != "" {
		t.Fatalf("a refused key must not be persisted: mode=%q key=%q", cfg.Auth.Mode, cfg.Auth.APIKey)
	}
}

func TestNonInteractiveAPIKeyLoginReadsFromTheCommandInputStream(t *testing.T) {
	// REV-6759: the non-interactive branch used fmt.Scanln, which reads os.Stdin
	// directly and ignores cmd.InOrStdin(). Piping worked only by accident of the
	// two being the same descriptor, and nothing could exercise the branch.
	isolatedHome(t)

	root := cmd.NewRootCmd()
	out := new(bytes.Buffer)
	root.SetOut(out)
	root.SetErr(out)
	root.SetIn(strings.NewReader(testfixture.LiveKeyPrefix + "pipedkeyvalue\n"))
	root.SetArgs([]string{"login", "--api-key"})
	if err := root.Execute(); err != nil {
		t.Fatalf("login --api-key: %v", err)
	}

	cfg := config.Load()
	if cfg.Auth.Mode != "api_key" || cfg.Auth.APIKey != testfixture.LiveKeyPrefix+"pipedkeyvalue" {
		t.Fatalf("piped key was not stored: mode=%q key=%q", cfg.Auth.Mode, cfg.Auth.APIKey)
	}
	if strings.Contains(out.String(), testfixture.LiveKeyPrefix+"pipedkeyvalue") {
		t.Fatalf("the confirmation must mask the key:\n%s", out.String())
	}
}

func TestNonInteractiveAPIKeyLoginFailsClosedOnEmptyInput(t *testing.T) {
	isolatedHome(t)
	_, err := runCLI(t, "login", "--api-key")
	if err == nil {
		t.Fatal("an empty stdin must not produce an authenticated state")
	}
	if cfg := config.Load(); cfg.Auth.Mode != "" {
		t.Fatalf("auth mode = %q, want unset", cfg.Auth.Mode)
	}
}

// ---------------------------------------------------------------------------
// What lands on disk
// ---------------------------------------------------------------------------

func TestSecretsAreRedactedFromConfigTomlAndStored0600(t *testing.T) {
	home := isolatedHome(t)
	t.Setenv("REVKEEN_ACCESS_TOKEN", testfixture.LiveKeyPrefix+"ondiskcheck")

	if _, err := runCLI(t, "login", "--with-token"); err != nil {
		t.Fatalf("login: %v", err)
	}

	configPath := filepath.Join(home, ".revkeen", "config.toml")
	credsPath := filepath.Join(home, ".revkeen", "credentials.toml")

	if body := readIfPresent(t, configPath); strings.Contains(body, testfixture.LiveKeyPrefix+"ondiskcheck") {
		t.Fatalf("config.toml retained the secret after the credential store accepted it:\n%s", body)
	}
	if body := readIfPresent(t, credsPath); !strings.Contains(body, testfixture.LiveKeyPrefix+"ondiskcheck") {
		t.Fatalf("credentials.toml is missing the secret:\n%s", body)
	}

	for _, path := range []string{configPath, credsPath} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		if info.Mode().Perm()&0o077 != 0 {
			t.Fatalf("%s is group/world readable: %v", path, info.Mode())
		}
	}
}

func TestLogoutClearsBothStores(t *testing.T) {
	home := isolatedHome(t)
	t.Setenv("REVKEEN_ACCESS_TOKEN", testfixture.AccessTokenPrefix+"tobecleared")

	if _, err := runCLI(t, "login", "--with-token"); err != nil {
		t.Fatalf("login: %v", err)
	}
	if _, err := runCLI(t, "auth", "logout"); err != nil {
		t.Fatalf("logout: %v", err)
	}

	for _, name := range []string{"config.toml", "credentials.toml"} {
		if body := readIfPresent(t, filepath.Join(home, ".revkeen", name)); strings.Contains(body, testfixture.AccessTokenPrefix+"tobecleared") {
			t.Fatalf("%s still holds the revoked token:\n%s", name, body)
		}
	}
	cfg := config.Load()
	if cfg.Auth.Mode != "" || cfg.Auth.OAuth.AccessToken != "" || cfg.Auth.APIKey != "" {
		t.Fatalf("logout left credentials behind: %+v", cfg.Auth)
	}
}

// ---------------------------------------------------------------------------
// `auth status`
// ---------------------------------------------------------------------------

func TestAuthStatusMasksTheStoredAPIKey(t *testing.T) {
	isolatedHome(t)
	t.Setenv("REVKEEN_ACCESS_TOKEN", testfixture.LiveKeyPrefix+"abcdefghijklmnop")
	if _, err := runCLI(t, "login", "--with-token"); err != nil {
		t.Fatalf("login: %v", err)
	}

	out, err := runCLI(t, "auth", "status")
	if err != nil {
		t.Fatalf("auth status: %v", err)
	}
	if strings.Contains(out, testfixture.LiveKeyPrefix+"abcdefghijklmnop") {
		t.Fatalf("auth status printed the full key:\n%s", out)
	}
	if !strings.Contains(out, "...") {
		t.Fatalf("auth status must mask the key:\n%s", out)
	}
}

func TestAuthStatusNeverPrintsTheOAuthAccessToken(t *testing.T) {
	isolatedHome(t)
	t.Setenv("REVKEEN_ACCESS_TOKEN", testfixture.AccessTokenPrefix+"donotprintme")
	if _, err := runCLI(t, "login", "--with-token"); err != nil {
		t.Fatalf("login: %v", err)
	}

	out, err := runCLI(t, "auth", "status")
	if err != nil {
		t.Fatalf("auth status: %v", err)
	}
	if strings.Contains(out, testfixture.AccessTokenPrefix+"donotprintme") {
		t.Fatalf("auth status printed the bearer token:\n%s", out)
	}
	if !strings.Contains(out, "OAuth") {
		t.Fatalf("auth status must report OAuth mode:\n%s", out)
	}
}

func TestAuthStatusFailsWhenNoCredentialIsConfigured(t *testing.T) {
	isolatedHome(t)
	out, err := runCLI(t, "auth", "status")
	if err == nil {
		t.Fatalf("auth status must exit non-zero when unauthenticated:\n%s", out)
	}
	if !strings.Contains(out, "Not authenticated") {
		t.Fatalf("expected an actionable message, got:\n%s", out)
	}
}

func TestAuthStatusReportsTheEnvironmentCredentialWithoutAStoredConfig(t *testing.T) {
	isolatedHome(t)
	t.Setenv("REVKEEN_API_KEY", testfixture.SandboxKeyPrefix+"abcdefghijklmnop")

	out, err := runCLI(t, "auth", "status")
	if err != nil {
		t.Fatalf("auth status: %v", err)
	}
	if !strings.Contains(out, "REVKEEN_API_KEY") {
		t.Fatalf("expected the environment credential to be named:\n%s", out)
	}
	if strings.Contains(out, testfixture.SandboxKeyPrefix+"abcdefghijklmnop") {
		t.Fatalf("auth status printed the full environment key:\n%s", out)
	}
}
