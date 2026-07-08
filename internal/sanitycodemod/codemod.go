package sanitycodemod

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const DefaultPackageName = "@revkeen/sanity-cart"

var configCandidates = []string{
	"sanity.config.ts",
	"sanity.config.tsx",
	"sanity.config.js",
	"sanity.config.mjs",
	"sanity.config.cjs",
}

type PackageManager string

const (
	PackageManagerNPM  PackageManager = "npm"
	PackageManagerPNPM PackageManager = "pnpm"
	PackageManagerYarn PackageManager = "yarn"
)

type Options struct {
	ProjectDir     string
	PackageName    string
	DryRun         bool
	InstallPackage bool
	RunCommand     CommandRunner
	Now            func() time.Time
}

type CommandRunner func(projectDir, name string, args ...string) error

type Result struct {
	Changed               bool
	ConfigPath            string
	PackageManager        PackageManager
	PackageAlreadyPresent bool
	PackageInstallCommand string
	BackupPath            string
	Diff                  string
	ManualPatch           string
}

func Apply(opts Options) (Result, error) {
	projectDir := opts.ProjectDir
	if projectDir == "" {
		projectDir = "."
	}
	projectDir, err := filepath.Abs(projectDir)
	if err != nil {
		return Result{}, err
	}

	packageName := opts.PackageName
	if packageName == "" {
		packageName = DefaultPackageName
	}

	result := Result{PackageManager: DetectPackageManager(projectDir)}
	configPath, err := FindConfig(projectDir)
	if err != nil {
		result.ManualPatch = ManualPatch(packageName, result.PackageManager)
		return result, err
	}
	result.ConfigPath = configPath
	result.PackageAlreadyPresent = packageJSONHasPackage(filepath.Join(projectDir, "package.json"), packageName)
	if !result.PackageAlreadyPresent {
		result.PackageInstallCommand = installCommand(result.PackageManager, packageName)
	}
	result.ManualPatch = ManualPatch(packageName, result.PackageManager)

	originalBytes, err := os.ReadFile(configPath)
	if err != nil {
		return result, err
	}
	original := string(originalBytes)
	updated, changed, err := PatchConfig(original, packageName)
	if err != nil {
		return result, err
	}
	result.Changed = changed || !result.PackageAlreadyPresent
	result.Diff = UnifiedDiff(configPath, original, updated)

	if opts.DryRun || !changed {
		return result, nil
	}

	if !result.PackageAlreadyPresent && opts.InstallPackage {
		if err := installPackage(projectDir, result.PackageManager, packageName, opts.RunCommand); err != nil {
			return result, err
		}
	}

	backupPath := backupPathFor(configPath, opts.Now)
	result.BackupPath = backupPath
	if err := os.WriteFile(backupPath, originalBytes, 0o600); err != nil {
		return result, err
	}

	tmpPath := configPath + ".revkeen.tmp"
	if err := os.WriteFile(tmpPath, []byte(updated), fileMode(configPath)); err != nil {
		restore(configPath, backupPath)
		return result, err
	}
	if _, _, err := PatchConfig(updated, packageName); err != nil {
		os.Remove(tmpPath)
		restore(configPath, backupPath)
		return result, fmt.Errorf("patched config failed validation: %w", err)
	}
	if err := os.Rename(tmpPath, configPath); err != nil {
		os.Remove(tmpPath)
		restore(configPath, backupPath)
		return result, err
	}

	return result, nil
}

func DetectPackageManager(projectDir string) PackageManager {
	if exists(filepath.Join(projectDir, "pnpm-lock.yaml")) {
		return PackageManagerPNPM
	}
	if exists(filepath.Join(projectDir, "yarn.lock")) {
		return PackageManagerYarn
	}
	return PackageManagerNPM
}

func FindConfig(projectDir string) (string, error) {
	for _, candidate := range configCandidates {
		path := filepath.Join(projectDir, candidate)
		if exists(path) {
			return path, nil
		}
	}
	return "", errors.New("no supported Sanity config found")
}

func PatchConfig(source, packageName string) (string, bool, error) {
	if !strings.Contains(source, "defineConfig") {
		return "", false, errors.New("unsupported Sanity config: defineConfig(...) call not found")
	}

	updated := source
	changed := false
	if !hasRevKeenImport(updated, packageName) {
		updated = insertImport(updated, packageName)
		changed = true
	}
	if !hasRevKeenPluginRegistration(updated) {
		next, err := insertPlugin(updated)
		if err != nil {
			return "", false, err
		}
		updated = next
		changed = true
	}
	if _, err := findDefineConfigObject(updated); err != nil {
		return "", false, err
	}
	return updated, changed, nil
}

func ManualPatch(packageName string, pm PackageManager) string {
	return fmt.Sprintf(`Install the package:
  %s

Add this import to sanity.config:
  import { revkeenSanityCartPlugin } from %q;

Add the plugin inside defineConfig({ ... }):
  plugins: [
    revkeenSanityCartPlugin(),
    // existing plugins...
  ],`, installCommand(pm, packageName), packageName)
}

func hasRevKeenImport(source, packageName string) bool {
	return strings.Contains(source, packageName) &&
		strings.Contains(source, "revkeenSanityCartPlugin")
}

func hasRevKeenPluginRegistration(source string) bool {
	return strings.Contains(source, "revkeenSanityCartPlugin(")
}

func insertImport(source, packageName string) string {
	importLine := fmt.Sprintf("import { revkeenSanityCartPlugin } from %q;\n", packageName)
	lines := strings.SplitAfter(source, "\n")
	insertAt := 0
	importRe := regexp.MustCompile(`^\s*import(\s|{|\*)`)
	for i, line := range lines {
		if importRe.MatchString(line) {
			insertAt = i + 1
		}
	}
	if insertAt == 0 {
		return importLine + source
	}
	withImport := append([]string{}, lines[:insertAt]...)
	withImport = append(withImport, importLine)
	withImport = append(withImport, lines[insertAt:]...)
	return strings.Join(withImport, "")
}

type span struct {
	start int
	end   int
}

func insertPlugin(source string) (string, error) {
	object, err := findDefineConfigObject(source)
	if err != nil {
		return "", err
	}

	plugins, ok, err := findPropertyArray(source, object, "plugins")
	if err != nil {
		return "", err
	}
	if ok {
		insertAt := plugins.start + 1
		prefix := "\n    revkeenSanityCartPlugin(),"
		if strings.TrimSpace(source[plugins.start+1:plugins.end]) == "" {
			prefix = "\n    revkeenSanityCartPlugin(),\n  "
		}
		return source[:insertAt] + prefix + source[insertAt:], nil
	}

	insertAt := object.start + 1
	return source[:insertAt] + "\n  plugins: [revkeenSanityCartPlugin()]," + source[insertAt:], nil
}

func findDefineConfigObject(source string) (span, error) {
	idx := strings.Index(source, "defineConfig")
	if idx < 0 {
		return span{}, errors.New("defineConfig(...) call not found")
	}
	openParen := strings.Index(source[idx:], "(")
	if openParen < 0 {
		return span{}, errors.New("defineConfig call is missing opening parenthesis")
	}
	searchFrom := idx + openParen + 1
	for i := searchFrom; i < len(source); i++ {
		if source[i] == '{' {
			end, err := matchingBracket(source, i, '{', '}')
			if err != nil {
				return span{}, err
			}
			return span{start: i, end: end}, nil
		}
		if !isWhitespace(source[i]) {
			return span{}, errors.New("defineConfig first argument is not an object literal")
		}
	}
	return span{}, errors.New("defineConfig object not found")
}

func findPropertyArray(source string, object span, property string) (span, bool, error) {
	body := source[object.start+1 : object.end]
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(property) + `\s*:`)
	matches := re.FindAllStringIndex(body, -1)
	for _, match := range matches {
		colonAt := object.start + 1 + match[1] - 1
		i := colonAt + 1
		for i < object.end && isWhitespace(source[i]) {
			i++
		}
		if i >= object.end || source[i] != '[' {
			return span{}, false, fmt.Errorf("%s property exists but is not an array", property)
		}
		end, err := matchingBracket(source, i, '[', ']')
		if err != nil {
			return span{}, false, err
		}
		return span{start: i, end: end}, true, nil
	}
	return span{}, false, nil
}

func matchingBracket(source string, start int, open, close byte) (int, error) {
	depth := 0
	var quote byte
	escaped := false
	for i := start; i < len(source); i++ {
		ch := source[i]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == quote {
				quote = 0
			}
			continue
		}
		if ch == '"' || ch == '\'' || ch == '`' {
			quote = ch
			continue
		}
		if ch == open {
			depth++
		}
		if ch == close {
			depth--
			if depth == 0 {
				return i, nil
			}
		}
	}
	return 0, fmt.Errorf("unmatched %q", open)
}

func UnifiedDiff(path, before, after string) string {
	if before == after {
		return ""
	}
	beforeLines := strings.Split(before, "\n")
	afterLines := strings.Split(after, "\n")
	var b strings.Builder
	b.WriteString("--- " + path + "\n")
	b.WriteString("+++ " + path + "\n")
	for _, line := range beforeLines {
		if line != "" {
			b.WriteString("-" + line + "\n")
		}
	}
	for _, line := range afterLines {
		if line != "" {
			b.WriteString("+" + line + "\n")
		}
	}
	return b.String()
}

func packageJSONHasPackage(path, packageName string) bool {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.Contains(string(bytes), `"`+packageName+`"`)
}

func installCommand(pm PackageManager, packageName string) string {
	switch pm {
	case PackageManagerPNPM:
		return "pnpm add " + packageName
	case PackageManagerYarn:
		return "yarn add " + packageName
	default:
		return "npm install " + packageName
	}
}

func installPackage(projectDir string, pm PackageManager, packageName string, runner CommandRunner) error {
	name := "npm"
	args := []string{"install", packageName}
	switch pm {
	case PackageManagerPNPM:
		name = "pnpm"
		args = []string{"add", packageName}
	case PackageManagerYarn:
		name = "yarn"
		args = []string{"add", packageName}
	}
	if runner == nil {
		runner = defaultCommandRunner
	}
	return runner(projectDir, name, args...)
}

func defaultCommandRunner(projectDir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = projectDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func backupPathFor(configPath string, now func() time.Time) string {
	clock := time.Now
	if now != nil {
		clock = now
	}
	return fmt.Sprintf("%s.revkeen-backup-%s", configPath, clock().UTC().Format("20060102T150405Z"))
}

func restore(configPath, backupPath string) {
	if bytes, err := os.ReadFile(backupPath); err == nil {
		_ = os.WriteFile(configPath, bytes, fileMode(configPath))
	}
}

func fileMode(path string) os.FileMode {
	info, err := os.Stat(path)
	if err != nil {
		return 0o600
	}
	return info.Mode()
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func isWhitespace(ch byte) bool {
	return ch == ' ' || ch == '\n' || ch == '\r' || ch == '\t'
}

func SupportedConfigNames() []string {
	out := append([]string{}, configCandidates...)
	sort.Strings(out)
	return out
}
