package cmd_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/revkeen/cli/cmd"
)

func TestAuthLoginDeviceFlagIsNotStub(t *testing.T) {
	// --device must attempt the real device grant — not the old stub.
	root := cmd.NewRootCmd()
	root.SetArgs([]string{"auth", "login", "--device", "--no-browser"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error without a live Engine API device endpoint")
	}
	msg := err.Error()
	if strings.Contains(msg, "not implemented") {
		t.Fatalf("device flow must not advertise not-implemented: %v", err)
	}
	if !strings.Contains(msg, "OAuth device login failed") && !strings.Contains(msg, "device") {
		t.Fatalf("expected device-flow failure, got: %v", err)
	}
}

func TestTopLevelLoginCommandExists(t *testing.T) {
	root := cmd.NewRootCmd()
	found := false
	for _, c := range root.Commands() {
		if c.Name() == "login" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("top-level revkeen login missing (REV-2974)")
	}
}

func TestAuthLoginWithTokenRequiresInput(t *testing.T) {
	t.Setenv("REVKEEN_ACCESS_TOKEN", "")
	t.Setenv("REVKEEN_API_KEY", "")
	root := cmd.NewRootCmd()
	root.SetIn(strings.NewReader(""))
	root.SetArgs([]string{"login", "--with-token"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "no token provided") {
		t.Fatalf("expected no token provided, got %v", err)
	}
}

func TestListenRequiresAuth(t *testing.T) {
	t.Setenv("REVKEEN_API_KEY", "")
	root := cmd.NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"listen", "--forward-to", "http://localhost:3000"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "not authenticated") {
		t.Fatalf("expected not authenticated, got %v", err)
	}
}

func TestWebhooksListenIsAliasNotStub(t *testing.T) {
	t.Setenv("REVKEEN_API_KEY", "")
	root := cmd.NewRootCmd()
	root.SetArgs([]string{"webhooks", "listen", "--forward-to", "http://localhost:3000"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("webhooks listen must not be a stub anymore: %v", err)
	}
	if !strings.Contains(err.Error(), "not authenticated") {
		t.Fatalf("expected auth error from listen alias, got %v", err)
	}
}

func TestTriggerCommandExists(t *testing.T) {
	root := cmd.NewRootCmd()
	found := false
	for _, c := range root.Commands() {
		if c.Name() == "trigger" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("top-level revkeen trigger missing (REV-2978)")
	}
}

func TestTerminalPairIsHonestStub(t *testing.T) {
	root := cmd.NewRootCmd()
	root.SetArgs([]string{"terminal", "pair", "--serial", "PAX1"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("expected not implemented, got %v", err)
	}
}

func TestDashboardCommandExists(t *testing.T) {
	root := cmd.NewRootCmd()
	found := false
	for _, c := range root.Commands() {
		if c.Name() == "dashboard" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("dashboard command missing")
	}
}

func TestRootBootstrapsResourceCommands(t *testing.T) {
	root := cmd.NewRootCmd()
	found := false
	for _, c := range root.Commands() {
		if c.Name() == "customers" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("climate customers group missing — bootstrap may have failed silently")
	}
}
