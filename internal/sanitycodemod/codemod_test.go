package sanitycodemod

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestApplyStandardConfigDryRun(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "pnpm-lock.yaml"), "")
	write(t, filepath.Join(dir, "package.json"), `{"dependencies":{}}`)
	write(t, filepath.Join(dir, "sanity.config.ts"), `import {defineConfig} from "sanity";
import {deskTool} from "sanity/desk";

export default defineConfig({
  name: "default",
  plugins: [deskTool()],
});
`)

	result, err := Apply(Options{ProjectDir: dir, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.PackageManager != PackageManagerPNPM {
		t.Fatalf("package manager = %s", result.PackageManager)
	}
	if result.PackageInstallCommand != "pnpm add @revkeen/sanity-cart" {
		t.Fatalf("install command = %q", result.PackageInstallCommand)
	}
	if result.BackupPath != "" {
		t.Fatalf("dry run wrote backup path %q", result.BackupPath)
	}
	if !strings.Contains(result.Diff, `+import { revkeenSanityCartPlugin } from "@revkeen/sanity-cart";`) {
		t.Fatalf("diff missing import:\n%s", result.Diff)
	}
	if !strings.Contains(result.Diff, "+    revkeenSanityCartPlugin(),") {
		t.Fatalf("diff missing plugin:\n%s", result.Diff)
	}
	assertFileNotContains(t, filepath.Join(dir, "sanity.config.ts"), "revkeenSanityCartPlugin")
}

func TestApplyAlreadyInstalledConfigIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "sanity.config.ts")
	write(t, filepath.Join(dir, "package.json"), `{"dependencies":{"@revkeen/sanity-cart":"^0.1.0"}}`)
	write(t, configPath, `import {defineConfig} from "sanity";
import { revkeenSanityCartPlugin } from "@revkeen/sanity-cart";

export default defineConfig({
  plugins: [revkeenSanityCartPlugin()],
});
`)

	first, err := Apply(Options{ProjectDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Apply(Options{ProjectDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	content := read(t, configPath)
	if strings.Count(content, "revkeenSanityCartPlugin") != 2 {
		t.Fatalf("expected import + one plugin registration, got:\n%s", content)
	}
	if first.Changed || second.Changed {
		t.Fatalf("already-installed config should be unchanged: first=%v second=%v", first.Changed, second.Changed)
	}
}

func TestApplyUnsupportedConfigReturnsManualPatch(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "package.json"), `{}`)
	write(t, filepath.Join(dir, "sanity.config.ts"), `export default {plugins: []}`)

	result, err := Apply(Options{ProjectDir: dir})
	if err == nil {
		t.Fatal("expected unsupported config error")
	}
	if !strings.Contains(result.ManualPatch, "revkeenSanityCartPlugin") {
		t.Fatalf("manual patch missing plugin guidance: %q", result.ManualPatch)
	}
}

func TestApplyInvalidPluginsArrayLeavesProjectUnchanged(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "sanity.config.ts")
	original := `import {defineConfig} from "sanity";
export default defineConfig({
  plugins: "not-array",
});
`
	write(t, filepath.Join(dir, "package.json"), `{}`)
	write(t, configPath, original)

	_, err := Apply(Options{ProjectDir: dir})
	if err == nil {
		t.Fatal("expected validation failure")
	}
	if after := read(t, configPath); after != original {
		t.Fatalf("failed edit changed config:\n%s", after)
	}
}

func TestApplyWritesBackupBeforeReplace(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "sanity.config.ts")
	original := `import {defineConfig} from "sanity";

export default defineConfig({
  plugins: [],
});
`
	write(t, filepath.Join(dir, "package.json"), `{}`)
	write(t, configPath, original)

	result, err := Apply(Options{
		ProjectDir: dir,
		Now: func() time.Time {
			return time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.BackupPath == "" {
		t.Fatal("expected backup path")
	}
	if read(t, result.BackupPath) != original {
		t.Fatal("backup did not preserve original config")
	}
	if !strings.Contains(read(t, configPath), "revkeenSanityCartPlugin()") {
		t.Fatal("config was not patched")
	}
}

func TestApplyCanInstallMissingPackage(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "yarn.lock"), "")
	write(t, filepath.Join(dir, "package.json"), `{"dependencies":{}}`)
	write(t, filepath.Join(dir, "sanity.config.ts"), `import {defineConfig} from "sanity";
export default defineConfig({
  plugins: [],
});
`)

	var ran []string
	_, err := Apply(Options{
		ProjectDir:     dir,
		InstallPackage: true,
		RunCommand: func(projectDir, name string, args ...string) error {
			ran = append([]string{projectDir, name}, args...)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{dir, "yarn", "add", "@revkeen/sanity-cart"}
	if strings.Join(ran, " ") != strings.Join(want, " ") {
		t.Fatalf("installer = %#v, want %#v", ran, want)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(bytes)
}

func assertFileNotContains(t *testing.T, path, needle string) {
	t.Helper()
	if strings.Contains(read(t, path), needle) {
		t.Fatalf("%s unexpectedly contains %q", path, needle)
	}
}
