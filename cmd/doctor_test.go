package cmd

// Tests for revkeen doctor (REV-5153): remote checks via the real generated
// SDK client against httptest, local env inspection via temp fixtures, JSON
// automation output, and the no-secret-leak guarantee. REV-8396 removed the
// Sanity project checks, and REV-8408 removed the --cms tombstone flag.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/revkeen/cli/internal/testfixture"
	revkeen "github.com/revkeen/sdk-go"
	"github.com/spf13/cobra"
)

func doctorServer(t *testing.T, subscribed bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/storefront/status":
			_ = json.NewEncoder(w).Encode(storefrontStatusEnvelope(true, []map[string]any{
				{"id": "cart_enabled", "status": "pass", "message": "Cart is enabled."},
				{"id": "keys", "status": "pass", "message": "Keys provisioned."},
				{"id": "origins", "status": "pass", "message": "1 origin registered."},
			}))
		case "/webhook-endpoints":
			events := []string{"invoice.paid"}
			if subscribed {
				events = []string{"commerce.checkout.completed"}
			}
			_ = json.NewEncoder(w).Encode(webhookEndpointsEnvelope("https://hooks.example.com/revkeen", events))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
}

func doctorProjectDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env.local"), []byte(
		"NEXT_PUBLIC_REVKEEN_PUBLISHABLE_KEY="+testfixture.PublishableLiveKeyPrefix+"supersecretvalue\nNEXT_PUBLIC_REVKEEN_BASE_URL=https://api.revkeen.com/v2\nREVKEEN_SECRET_KEY="+testfixture.LiveKeyPrefix+"donotleakme\n",
	), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func newDoctorTestRoot(t *testing.T, serverURL string, args ...string) (*cobra.Command, *bytes.Buffer) {
	t.Helper()
	original := newCartSDKClient
	newCartSDKClient = func(cmd *cobra.Command) (*revkeen.APIClient, error) {
		return revkeen.NewClientWithCustomBaseURL(
			"rk_sandbox_test",
			serverURL,
			revkeen.WithMaxAttempts(1),
		)
	}
	t.Cleanup(func() { newCartSDKClient = original })

	root := &cobra.Command{Use: "revkeen"}
	root.PersistentFlags().StringP("output", "o", "table", "")
	root.PersistentFlags().Bool("no-color", false, "")
	root.PersistentFlags().String("api-key", "", "")
	root.PersistentFlags().Bool("agent", false, "")
	root.PersistentFlags().Bool("json", false, "")
	root.PersistentFlags().Bool("table", false, "")
	root.AddCommand(newDoctorCmd())

	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs(args)
	return root, out
}

func TestDoctorReadyPathAndNoSecretLeak(t *testing.T) {
	server := doctorServer(t, true)
	defer server.Close()
	dir := doctorProjectDir(t)

	root, out := newDoctorTestRoot(t, server.URL, "doctor", "--project-path", dir, "--table")
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	text := out.String()
	for _, want := range []string{
		"READY",
		"cart_enabled",
		"commerce.checkout.completed is delivered",
		"NEXT_PUBLIC_REVKEEN_PUBLISHABLE_KEY is set.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
	// Env VALUES must never appear — presence only.
	for _, leak := range []string{testfixture.LiveKeyPrefix + "donotleakme", testfixture.PublishableLiveKeyPrefix + "supersecretvalue"} {
		if strings.Contains(text, leak) {
			t.Fatalf("doctor output leaked an env value %q", leak)
		}
	}
}

func TestDoctorFlagsMissingKeyAndSubscription(t *testing.T) {
	server := doctorServer(t, false)
	defer server.Close()
	dir := doctorProjectDir(t)
	// A missing publishable key is the remaining local "fail" check, so it is
	// what drives NOT READY now that the Sanity project checks are gone.
	if err := os.WriteFile(filepath.Join(dir, ".env.local"), []byte("NEXT_PUBLIC_REVKEEN_BASE_URL=https://api.revkeen.com/v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NEXT_PUBLIC_REVKEEN_PUBLISHABLE_KEY", "")

	root, out := newDoctorTestRoot(t, server.URL, "doctor", "--project-path", dir, "--table")
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	text := out.String()
	for _, want := range []string{
		"NOT READY",
		"NEXT_PUBLIC_REVKEEN_PUBLISHABLE_KEY is not set",
		"No active webhook endpoint subscribes to commerce.checkout.completed",
		"revkeen cart webhooks setup --url",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
}

func TestDoctorJSONOutputPowersAutomation(t *testing.T) {
	server := doctorServer(t, true)
	defer server.Close()
	dir := doctorProjectDir(t)

	root, _ := newDoctorTestRoot(t, server.URL, "doctor", "--project-path", dir, "--json")
	// output.Print writes to os.Stdout; capture it.
	stdout := os.Stdout
	pipeRead, pipeWrite, _ := os.Pipe()
	os.Stdout = pipeWrite
	err := root.Execute()
	if closeErr := pipeWrite.Close(); closeErr != nil {
		os.Stdout = stdout
		t.Fatalf("close stdout pipe: %v", closeErr)
	}
	os.Stdout = stdout
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	raw := new(bytes.Buffer)
	_, _ = raw.ReadFrom(pipeRead)

	var report struct {
		Object string `json:"object"`
		Ready  bool   `json:"ready"`
		Checks []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(raw.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, raw.String())
	}
	if report.Object != "doctor_report" || !report.Ready || len(report.Checks) < 7 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

// REV-8408: the one-release --cms tombstone is gone. The flag is now unknown,
// so cobra rejects it before any check runs (the server URL is unreachable on
// purpose: reaching it would mean a check ran).
func TestDoctorCMSFlagIsRemoved(t *testing.T) {
	root, _ := newDoctorTestRoot(t, "http://127.0.0.1:1", "doctor", "--cms", "sanity")
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "unknown flag: --cms") {
		t.Fatalf("expected an unknown-flag error for --cms, got %v", err)
	}
}

// REV-8408: `revkeen init` (the REV-8396 tombstone) is no longer registered.
func TestInitCommandIsRemoved(t *testing.T) {
	for _, sub := range NewRootCmd().Commands() {
		if sub.Name() == "init" {
			t.Fatal("the removed `init` command is still registered on the root command")
		}
	}
}
