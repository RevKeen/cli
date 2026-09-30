package cmd

import (
	"embed"
	"fmt"
	"os"

	"github.com/lispyclouds/climate"
	"github.com/mattn/go-isatty"
	"github.com/revkeen/cli/internal/handlers"
	"github.com/revkeen/cli/internal/ui/dashboard"
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
  revkeen login             OAuth PKCE browser login (default)
  revkeen login --device    Device grant (SSH / headless)
  revkeen login --api-key   Interactive API key login
  REVKEEN_API_KEY=rk_...    Environment variable

Quick start:
  revkeen dashboard
  revkeen customers list
  revkeen invoices get inv_xxxxxxxx --json
  revkeen api GET /v2/customers`,
		Version: version,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Bare `revkeen` on a TTY opens the dashboard; otherwise show help.
			if isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd()) {
				agent, _ := cmd.Root().PersistentFlags().GetBool("agent")
				jsonFlag, _ := cmd.Root().PersistentFlags().GetBool("json")
				if !agent && !jsonFlag {
					return dashboard.Run(cmd)
				}
			}
			return cmd.Help()
		},
	}

	rootCmd.DisableAutoGenTag = true

	rootCmd.PersistentFlags().StringP("output", "o", "table", "Output format: table, json, yaml, csv")
	rootCmd.PersistentFlags().Bool("no-color", false, "Disable color output")
	rootCmd.PersistentFlags().String("api-key", "", "Override API key (or set REVKEEN_API_KEY)")
	rootCmd.PersistentFlags().Bool("agent", false, "Machine-readable mode: compact JSON, no color, errors as JSON to stderr")
	rootCmd.PersistentFlags().Bool("json", false, "Shorthand for --output json (pretty-printed)")
	rootCmd.PersistentFlags().Bool("table", false, "Shorthand for --output table")
	rootCmd.MarkFlagsMutuallyExclusive("agent", "json", "table")

	if err := bootstrapClimate(rootCmd); err != nil {
		rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
			return err
		}
	}

	rootCmd.AddCommand(newAuthCmd())
	rootCmd.AddCommand(newLoginCmd()) // top-level alias — REV-2974
	rootCmd.AddCommand(newListenCmd())  // REV-2978 Epic A
	rootCmd.AddCommand(newTriggerCmd()) // REV-2978 Epic A
	rootCmd.AddCommand(newWebhooksCmd())
	rootCmd.AddCommand(newCartCmd())
	rootCmd.AddCommand(newDoctorCmd())
	rootCmd.AddCommand(newTerminalCmd())
	rootCmd.AddCommand(newAPICmd())
	rootCmd.AddCommand(newConfigCmd())
	rootCmd.AddCommand(newDashboardCmd())

	return rootCmd
}

func bootstrapClimate(rootCmd *cobra.Command) error {
	specData, err := specFS.ReadFile("api.json")
	if err != nil {
		return fmt.Errorf("failed to load embedded OpenAPI spec: %w", err)
	}
	model, loadErr := climate.LoadV3(specData)
	if loadErr != nil {
		return fmt.Errorf("failed to parse OpenAPI spec: %w", loadErr)
	}
	handlerMap := handlers.BuildHandlerMap(model)
	if err := climate.BootstrapV3Cobra(rootCmd, *model, handlerMap); err != nil {
		return fmt.Errorf("failed to bootstrap CLI commands from OpenAPI: %w", err)
	}
	return nil
}

func newDashboardCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "dashboard",
		Aliases: []string{"ui", "tui"},
		Short:   "Open the interactive terminal dashboard",
		Long:    "Launch a Bubble Tea dashboard for customers, invoices, subscriptions, cart, and doctor.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return dashboard.Run(cmd)
		},
	}
}

func init() {
	cobra.EnableCommandSorting = false
}

// Root returns a fully initialised root command for use by doc generators
// and external tooling. Follows the Cobra "CLIs for LLMs" pattern.
func Root() *cobra.Command {
	return NewRootCmd()
}
