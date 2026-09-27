package output_test

import (
	"testing"

	"github.com/revkeen/cli/internal/output"
	"github.com/revkeen/cli/internal/ui"
	"github.com/spf13/cobra"
)

func TestAgentModeDisablesInteractiveUI(t *testing.T) {
	root := &cobra.Command{Use: "revkeen"}
	root.PersistentFlags().Bool("agent", false, "")
	root.PersistentFlags().Bool("json", false, "")
	root.PersistentFlags().Bool("table", false, "")
	root.PersistentFlags().String("output", "table", "")
	_ = root.PersistentFlags().Set("agent", "true")

	cmd := &cobra.Command{Use: "list"}
	root.AddCommand(cmd)

	if !output.IsAgent(cmd) {
		t.Fatal("expected agent mode")
	}
	if ui.Interactive(cmd) {
		t.Fatal("interactive UI must be disabled under --agent")
	}
}

func TestJSONFlagDisablesInteractiveUI(t *testing.T) {
	root := &cobra.Command{Use: "revkeen"}
	root.PersistentFlags().Bool("agent", false, "")
	root.PersistentFlags().Bool("json", false, "")
	root.PersistentFlags().Bool("table", false, "")
	root.PersistentFlags().String("output", "table", "")
	_ = root.PersistentFlags().Set("json", "true")

	cmd := &cobra.Command{Use: "list"}
	root.AddCommand(cmd)

	if ui.Interactive(cmd) {
		t.Fatal("interactive UI must be disabled under --json")
	}
	if output.ResolveFormat(cmd) != output.FormatJSON {
		t.Fatalf("format = %s", output.ResolveFormat(cmd))
	}
}
