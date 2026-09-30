// Package creds stores CLI secrets in the OS keyring when available,
// falling back to ~/.revkeen/credentials.toml (0600) — REV-2974.
package creds

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/zalando/go-keyring"
)

const (
	keyringService = "revkeen-cli"
	keyringUser    = "oauth"
)

// Secrets holds sensitive credential material.
type Secrets struct {
	AccessToken  string `toml:"access_token"`
	RefreshToken string `toml:"refresh_token"`
	APIKey       string `toml:"api_key"`
}

var (
	forceFileFallback bool
	mu                sync.Mutex
)

// ForceFileFallback disables the keyring (tests / CI without a secret service).
func ForceFileFallback(v bool) {
	mu.Lock()
	forceFileFallback = v
	mu.Unlock()
}

func credentialsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".revkeen", "credentials.toml")
	}
	return filepath.Join(home, ".revkeen", "credentials.toml")
}

// SaveSecrets persists secrets to keyring, or credentials.toml on failure.
func SaveSecrets(s Secrets) error {
	mu.Lock()
	fallback := forceFileFallback
	mu.Unlock()

	if !fallback {
		payload := encodePayload(s)
		if err := keyring.Set(keyringService, keyringUser, payload); err == nil {
			_ = os.Remove(credentialsPath())
			return nil
		}
	}
	return saveFile(s)
}

// LoadSecrets reads secrets from keyring or credentials.toml.
func LoadSecrets() (Secrets, error) {
	mu.Lock()
	fallback := forceFileFallback
	mu.Unlock()

	if !fallback {
		raw, err := keyring.Get(keyringService, keyringUser)
		if err == nil && raw != "" {
			return decodePayload(raw), nil
		}
	}
	return loadFile()
}

// ClearSecrets removes keyring + file credentials.
func ClearSecrets() error {
	_ = keyring.Delete(keyringService, keyringUser)
	path := credentialsPath()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func encodePayload(s Secrets) string {
	// Compact TOML-like single blob for keyring storage.
	return fmt.Sprintf(
		"access_token=%s\nrefresh_token=%s\napi_key=%s\n",
		s.AccessToken,
		s.RefreshToken,
		s.APIKey,
	)
}

func decodePayload(raw string) Secrets {
	var s Secrets
	for _, line := range splitLines(raw) {
		key, val, ok := cutOnce(line, "=")
		if !ok {
			continue
		}
		switch key {
		case "access_token":
			s.AccessToken = val
		case "refresh_token":
			s.RefreshToken = val
		case "api_key":
			s.APIKey = val
		}
	}
	return s
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func cutOnce(s, sep string) (string, string, bool) {
	for i := 0; i+len(sep) <= len(s); i++ {
		if s[i:i+len(sep)] == sep {
			return s[:i], s[i+len(sep):], true
		}
	}
	return "", "", false
}

func saveFile(s Secrets) error {
	dir := filepath.Dir(credentialsPath())
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(credentialsPath(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return toml.NewEncoder(f).Encode(s)
}

func loadFile() (Secrets, error) {
	var s Secrets
	data, err := os.ReadFile(credentialsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return s, err
	}
	if err := toml.Unmarshal(data, &s); err != nil {
		return s, err
	}
	return s, nil
}
