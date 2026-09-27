package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newTerminalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "terminal",
		Short: "Manage POS terminal devices",
	}

	cmd.AddCommand(newTerminalPairCmd())
	cmd.AddCommand(newTerminalStatusCmd())
	cmd.AddCommand(newTerminalPayCmd())

	return cmd
}

func newTerminalPairCmd() *cobra.Command {
	var serial string

	cmd := &cobra.Command{
		Use:     "pair",
		Short:   "Pair a POS terminal device",
		Long:    "Initiate the device pairing flow for a PAX A920 Pro terminal.",
		Example: `  revkeen terminal pair --serial PAX123456`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if serial == "" {
				return fmt.Errorf("--serial is required")
			}
			return fmt.Errorf("terminal pair is not implemented yet (serial=%s) — use the dashboard POS connector flow", serial)
		},
	}

	cmd.Flags().StringVar(&serial, "serial", "", "Terminal serial number (required)")
	_ = cmd.MarkFlagRequired("serial")

	return cmd
}

func newTerminalStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show status of connected terminals",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("terminal status is not implemented yet — use the dashboard POS devices view")
		},
	}
}

func newTerminalPayCmd() *cobra.Command {
	var terminalID string

	cmd := &cobra.Command{
		Use:     "pay [amount]",
		Short:   "Initiate a payment on a terminal",
		Args:    cobra.ExactArgs(1),
		Example: `  revkeen terminal pay 50.00 --terminal PAX123456`,
		RunE: func(cmd *cobra.Command, args []string) error {
			amount := args[0]
			if terminalID == "" {
				return fmt.Errorf("--terminal is required")
			}
			return fmt.Errorf("terminal pay is not implemented yet (amount=%s terminal=%s)", amount, terminalID)
		},
	}

	cmd.Flags().StringVar(&terminalID, "terminal", "", "Terminal ID or serial number (required)")
	_ = cmd.MarkFlagRequired("terminal")

	return cmd
}
