package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config represents the CLI configuration stored at ~/.revkeen/config.toml.
type Config struct {
	Auth     AuthConfig     `toml:"auth"`
	Settings SettingsConfig `toml:"settings"`
}

// AuthConfig holds authentication credentials.
type AuthConfig struct {
	Mode   string      `toml:"mode"`    // "oauth" or "api_key"
	APIKey string      `toml:"api_key"`
	OAuth  OAuthConfig `toml:"oauth"`
}

// OAuthConfig holds OAuth tokens.
type OAuthConfig struct {
	AccessToken  string `toml:"access_token"`
	RefreshToken string `toml:"refresh_token"`
	ExpiresAt    string `toml:"expires_at"`
	ClientID     string `toml:"client_id"`
	ClientSecret string `toml:"client_secret"`
}

// SettingsConfig holds general CLI settings.
type SettingsConfig struct {
	BaseURL       string `toml:"base_url"`
	DefaultOutput string `toml:"default_output"`
	MerchantID    string `toml:"merchant_id"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Settings: SettingsConfig{
			BaseURL:       "https://api.revkeen.com",
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
func Load() *Config {
	cfg := DefaultConfig()

	data, err := os.ReadFile(ConfigPath())
	if err != nil {
		return cfg
	}

	_ = toml.Unmarshal(data, cfg)
	return cfg
}

// Save writes the config to ~/.revkeen/config.toml.
func Save(cfg *Config) error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	f, err := os.OpenFile(ConfigPath(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	encoder := toml.NewEncoder(f)
	return encoder.Encode(cfg)
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
