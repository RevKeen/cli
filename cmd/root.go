package cmd

import (
	"embed"

	"github.com/lispyclouds/climate"
	"github.com/revkeen/cli/internal/handlers"
	"github.com/spf13/cobra"
)

//go:embed api.json
var specFS embed.FS

// version is overridden at release time via GoReleaser ldflags
// (-X github.com/revkeen/cli/cmd.version=...). It MUST be a var, not a const —
// the Go linker's -X flag cannot patch constants, so a const would pin every
// build to its literal value regardless of the release tag.
var version = "dev"

// NewRootCmd creates the root revkeen command with climate-bootstrapped
// resource commands and hand-written custom commands.
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "revkeen",
		Short: "RevKeen CLI — manage payments, subscriptions & billing",
		Long: `The official RevKeen command-line interface.
Manage customers, invoices, products, subscriptions, and more from your terminal.

Install:
  npm install -g @revkeen/cli-binary
  brew install revkeen/tap/revkeen
  curl -fsSL https://cli.revkeen.com/install.sh | sh

Authentication:
  revkeen auth login        Interactive OAuth login
  REVKEEN_API_KEY=rk_...    Environment variable

Quick start:
  revkeen customers list
  revkeen invoices retrieve inv_xxxxxxxx --json
  revkeen api GET /v2/customers`,
		Version: version,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Global setup (output format, verbosity) happens here
		},
	}

	rootCmd.DisableAutoGenTag = true

	// Global flags
	rootCmd.PersistentFlags().StringP("output", "o", "table", "Output format: table, json, yaml, csv")
	rootCmd.PersistentFlags().Bool("no-color", false, "Disable color output")
	rootCmd.PersistentFlags().String("api-key", "", "Override API key (or set REVKEEN_API_KEY)")
	rootCmd.PersistentFlags().Bool("agent", false, "Machine-readable mode: compact JSON, no color, errors as JSON to stderr")
	rootCmd.PersistentFlags().Bool("json", false, "Shorthand for --output json (pretty-printed)")
	rootCmd.PersistentFlags().Bool("table", false, "Shorthand for --output table")
	rootCmd.MarkFlagsMutuallyExclusive("agent", "json", "table")

	// ── 1. Climate-bootstrapped resource commands ────────────────────
	// climate reads the embedded OpenAPI spec and auto-generates Cobra
	// commands for every operation. x-cli-group extensions control how
	// operations are grouped into subcommands (customers, invoices, etc.).
	specData, err := specFS.ReadFile("api.json")
	if err == nil {
		model, loadErr := climate.LoadV3(specData)
		if loadErr == nil {
			handlerMap := handlers.BuildHandlerMap(model)
			_ = climate.BootstrapV3Cobra(rootCmd, *model, handlerMap)
		}
	}

	// ── 2. Hand-written commands ────────────────────────────────────
	rootCmd.AddCommand(newAuthCmd())
	rootCmd.AddCommand(newWebhooksCmd())
	rootCmd.AddCommand(newTerminalCmd())
	rootCmd.AddCommand(newAPICmd())
	rootCmd.AddCommand(newConfigCmd())

	return rootCmd
}

func init() {
	// Silence Cobra's default usage on errors
	cobra.EnableCommandSorting = false
}

// Root returns a fully initialised root command for use by doc generators
// and external tooling. Follows the Cobra "CLIs for LLMs" pattern.
func Root() *cobra.Command {
	return NewRootCmd()
}

