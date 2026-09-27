package creds

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadFileFallback(t *testing.T) {
	ForceFileFallback(true)
	t.Cleanup(func() { ForceFileFallback(false) })

	dir := t.TempDir()
	t.Setenv("HOME", dir)
	// Also cover credentialsPath using UserHomeDir
	_ = os.MkdirAll(filepath.Join(dir, ".revkeen"), 0700)

	want := Secrets{AccessToken: "tok", RefreshToken: "ref", APIKey: "rk_x"}
	if err := SaveSecrets(want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadSecrets()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %+v want %+v", got, want)
	}
	info, err := os.Stat(filepath.Join(dir, ".revkeen", "credentials.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0077 != 0 {
		t.Fatalf("credentials.toml perms too open: %v", info.Mode())
	}
	if err := ClearSecrets(); err != nil {
		t.Fatal(err)
	}
}
