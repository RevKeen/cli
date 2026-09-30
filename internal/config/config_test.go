package config_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/revkeen/cli/internal/config"
	"github.com/revkeen/cli/internal/testfixture"
	"github.com/spf13/cobra"
)

func TestResolveBaseURLAppendsV2(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", "https://api.revkeen.com/v2"},
		{"https://api.revkeen.com", "https://api.revkeen.com/v2"},
		{"https://api.revkeen.com/", "https://api.revkeen.com/v2"},
		{"https://api.revkeen.com/v2", "https://api.revkeen.com/v2"},
		{"https://staging-api.revkeen.com", "https://staging-api.revkeen.com/v2"},
		{"https://staging-api.revkeen.com/v2/", "https://staging-api.revkeen.com/v2"},
	}
	for _, tc := range cases {
		cfg := config.DefaultConfig()
		if tc.in != "" {
			cfg.Settings.BaseURL = tc.in
		}
		got := config.ResolveBaseURL(cfg)
		if got != tc.want {
			t.Fatalf("ResolveBaseURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestOriginBaseURLStripsV2(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Settings.BaseURL = "https://api.revkeen.com/v2"
	if got := config.OriginBaseURL(cfg); got != "https://api.revkeen.com" {
		t.Fatalf("got %s", got)
	}
}

func TestResolveAPIKeyFromCmdFlagBeatsEnv(t *testing.T) {
	t.Setenv("REVKEEN_API_KEY", "rk_env_key")
	cfg := config.DefaultConfig()
	cfg.Auth.Mode = "api_key"
	cfg.Auth.APIKey = "rk_config_key"

	root := &cobra.Command{Use: "revkeen"}
	root.PersistentFlags().String("api-key", "", "")
	child := &cobra.Command{Use: "customers"}
	root.AddCommand(child)
	_ = root.PersistentFlags().Set("api-key", "rk_flag_key")

	got := config.ResolveAPIKeyFromCmd(child, cfg)
	if got != "rk_flag_key" {
		t.Fatalf("got %q, want flag key", got)
	}
}

func TestResolveAPIKeyEnvBeatsConfig(t *testing.T) {
	t.Setenv("REVKEEN_API_KEY", "rk_env_key")
	cfg := config.DefaultConfig()
	cfg.Auth.Mode = "api_key"
	cfg.Auth.APIKey = "rk_config_key"
	got := config.ResolveAPIKey(cfg)
	if got != "rk_env_key" {
		t.Fatalf("got %q, want env key", got)
	}
}

func TestMaskAPIKey(t *testing.T) {
	got := config.MaskAPIKey(testfixture.LiveKeyPrefix + "abcdefghijklmnop")
	if got == testfixture.LiveKeyPrefix+"abcdefghijklmnop" || !strings.Contains(got, "...") {
		t.Fatalf("expected masked key, got %q", got)
	}
	if config.MaskAPIKey("short") != "****" {
		t.Fatal("expected **** for short keys")
	}
}

// REV-7138 — CLI credentials are API keys or OAuth tokens. Engine resolves
// member.role / key scopes. There is no merchant:cli (or any CLI) role.
func TestAuthConfigHasNoCliRole(t *testing.T) {
	typ := reflect.TypeOf(config.AuthConfig{})
	if _, ok := typ.FieldByName("Role"); ok {
		t.Fatal("AuthConfig must not carry a Role field; Engine resolves member.role or key scopes")
	}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		name := strings.ToLower(field.Name)
		tag := strings.ToLower(field.Tag.Get("toml"))
		if name == "role" || tag == "role" || strings.Contains(tag, "merchant:cli") {
			t.Fatalf("CLI auth must not persist a role; found %s toml:%q", field.Name, field.Tag)
		}
	}
	cfg := config.DefaultConfig()
	if cfg.Auth.Mode != "" && cfg.Auth.Mode != "oauth" && cfg.Auth.Mode != "api_key" {
		t.Fatalf("Auth.Mode = %q; want empty, oauth, or api_key — never a role string", cfg.Auth.Mode)
	}
}

func TestCLISourcesDoNotMintMerchantCliRole(t *testing.T) {
	root := filepath.Join("..", "..")
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "vendor", ".git":
				return fs.SkipDir
			}
			return nil
		}
		switch filepath.Ext(path) {
		case ".go", ".md":
		default:
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(data), "merchant:cli") {
			t.Errorf("%s mentions merchant:cli", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk CLI tree: %v", err)
	}
}
