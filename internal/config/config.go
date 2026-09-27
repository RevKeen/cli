package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/revkeen/cli/internal/creds"
	"github.com/spf13/cobra"
)

// Config represents the CLI configuration stored at ~/.revkeen/config.toml.
type Config struct {
	Auth     AuthConfig     `toml:"auth"`
	Settings SettingsConfig `toml:"settings"`
}

// AuthConfig holds authentication credentials.
type AuthConfig struct {
	Mode   string      `toml:"mode"` // "oauth" or "api_key"
	APIKey string      `toml:"api_key"`
	OAuth  OAuthConfig `toml:"oauth"`
}

// OAuthConfig holds OAuth / device-flow tokens.
type OAuthConfig struct {
	AccessToken  string `toml:"access_token"`
	RefreshToken string `toml:"refresh_token"`
	ExpiresAt    string `toml:"expires_at"`
	ClientID     string `toml:"client_id,omitempty"`
}

// SettingsConfig holds general CLI settings.
type SettingsConfig struct {
	BaseURL       string `toml:"base_url"`
	DefaultOutput string `toml:"default_output"`
	MerchantID    string `toml:"merchant_id"`
}

const (
	DefaultProductionBaseURL = "https://api.revkeen.com"
	DefaultStagingBaseURL    = "https://staging-api.revkeen.com"
)

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Settings: SettingsConfig{
			BaseURL:       DefaultProductionBaseURL,
			DefaultOutput: "table",
		},
	}
}

// ConfigDir returns the path to ~/.revkeen/.
func ConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".revkeen"
	}
	return filepath.Join(home, ".revkeen")
}

// ConfigPath returns the path to ~/.revkeen/config.toml.
func ConfigPath() string {
	return filepath.Join(ConfigDir(), "config.toml")
}

// Load reads the config file, returning defaults if it doesn't exist.
// OAuth / API-key secrets prefer the OS keyring (or credentials.toml) over
// values still present in config.toml.
func Load() *Config {
	cfg := DefaultConfig()

	data, err := os.ReadFile(ConfigPath())
	if err != nil {
		mergeSecrets(cfg)
		return cfg
	}

	_ = toml.Unmarshal(data, cfg)
	mergeSecrets(cfg)
	return cfg
}

func mergeSecrets(cfg *Config) {
	sec, err := creds.LoadSecrets()
	if err != nil {
		return
	}
	if sec.AccessToken != "" {
		cfg.Auth.OAuth.AccessToken = sec.AccessToken
	}
	if sec.RefreshToken != "" {
		cfg.Auth.OAuth.RefreshToken = sec.RefreshToken
	}
	if sec.APIKey != "" && cfg.Auth.APIKey == "" {
		cfg.Auth.APIKey = sec.APIKey
	}
}

// Save writes the config to ~/.revkeen/config.toml and persists secrets via
// keyring / credentials.toml (REV-2974). Secrets are redacted from config.toml
// when the secret store accepts them.
func Save(cfg *Config) error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	toStore := *cfg
	sec := creds.Secrets{
		AccessToken:  cfg.Auth.OAuth.AccessToken,
		RefreshToken: cfg.Auth.OAuth.RefreshToken,
		APIKey:       cfg.Auth.APIKey,
	}
	hasSecrets := sec.AccessToken != "" || sec.RefreshToken != "" || sec.APIKey != ""
	if hasSecrets {
		if err := creds.SaveSecrets(sec); err == nil {
			toStore.Auth.OAuth.AccessToken = ""
			toStore.Auth.OAuth.RefreshToken = ""
			if cfg.Auth.Mode == "api_key" {
				toStore.Auth.APIKey = ""
			}
		}
	}

	f, err := os.OpenFile(ConfigPath(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	encoder := toml.NewEncoder(f)
	return encoder.Encode(&toStore)
}

// ClearAuth removes auth mode metadata and all stored secrets.
func ClearAuth(cfg *Config) error {
	cfg.Auth = AuthConfig{}
	_ = creds.ClearSecrets()
	return Save(cfg)
}

// ResolveAPIKey returns the API key from env var or config, with env taking priority.
func ResolveAPIKey(cfg *Config) string {
	if envKey := os.Getenv("REVKEEN_API_KEY"); envKey != "" {
		return envKey
	}
	if cfg.Auth.Mode == "api_key" {
		return cfg.Auth.APIKey
	}
	return ""
}

// ResolveAPIKeyFromCmd returns flag → env → config precedence.
func ResolveAPIKeyFromCmd(cmd *cobra.Command, cfg *Config) string {
	if cmd != nil {
		if flagKey, err := cmd.Root().PersistentFlags().GetString("api-key"); err == nil && flagKey != "" {
			return flagKey
		}
	}
	return ResolveAPIKey(cfg)
}

// ResolveBaseURL returns the API origin with a trailing /v2 path segment.
// OpenAPI path keys are relative to the /v2 server URL (e.g. /customers).
func ResolveBaseURL(cfg *Config) string {
	base := DefaultProductionBaseURL
	if cfg != nil && cfg.Settings.BaseURL != "" {
		base = cfg.Settings.BaseURL
	}
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	for strings.HasSuffix(base, "/v2") {
		base = strings.TrimSuffix(base, "/v2")
		base = strings.TrimRight(base, "/")
	}
	return base + "/v2"
}

// OriginBaseURL strips a trailing /v2 for display/auth device pages.
func OriginBaseURL(cfg *Config) string {
	base := ResolveBaseURL(cfg)
	return strings.TrimSuffix(base, "/v2")
}

// MaskAPIKey redacts an API key for display (keeps prefix + last 4).
func MaskAPIKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 12 {
		return "****"
	}
	return key[:8] + "..." + key[len(key)-4:]
}
