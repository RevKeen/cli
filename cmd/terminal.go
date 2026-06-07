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
		Use:   "pair",
		Short: "Pair a POS terminal device",
		Long:  "Initiate the device pairing flow for a PAX A920 Pro terminal.",
		Example: `  revkeen terminal pair --serial PAX123456`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if serial == "" {
				return fmt.Errorf("--serial is required")
			}

			fmt.Printf("Pairing terminal %s...\n", serial)
			// TODO: Call POST /v1/connectors/register with linking code flow
			fmt.Println("Enter the pairing code shown on the terminal:")
			var code string
			_, _ = fmt.Scanln(&code)
			fmt.Printf("Pairing terminal %s with code %s...\n", serial, code)
			return nil
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
			// TODO: Call GET /v1/connectors/devices and display
			fmt.Println("Connected terminals:")
			fmt.Println("  (fetching...)")
			return nil
		},
	}
}

func newTerminalPayCmd() *cobra.Command {
	var terminalID string

	cmd := &cobra.Command{
		Use:   "pay [amount]",
		Short: "Initiate a payment on a terminal",
		Args:  cobra.ExactArgs(1),
		Example: `  revkeen terminal pay 50.00 --terminal PAX123456`,
		RunE: func(cmd *cobra.Command, args []string) error {
			amount := args[0]
			if terminalID == "" {
				return fmt.Errorf("--terminal is required")
			}
			fmt.Printf("Initiating $%s payment on terminal %s...\n", amount, terminalID)
			// TODO: Send payment command via WebSocket to terminal
			return nil
		},
	}

	cmd.Flags().StringVar(&terminalID, "terminal", "", "Terminal ID or serial number (required)")
	_ = cmd.MarkFlagRequired("terminal")

	return cmd
}
