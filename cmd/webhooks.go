package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newWebhooksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "webhooks",
		Short: "Work with RevKeen webhooks",
	}

	cmd.AddCommand(newWebhooksListenCmd())
	cmd.AddCommand(newWebhooksListCmd())

	return cmd
}

func newWebhooksListenCmd() *cobra.Command {
	var forwardTo string
	var events []string

	cmd := &cobra.Command{
		Use:   "listen",
		Short: "Forward webhook events to a local URL",
		Long: `Listen for RevKeen webhook events and forward them to a local server.
This is useful during development to test webhook handlers without
deploying to a public URL.`,
		Example: `  # Forward all events to localhost
  revkeen webhooks listen --forward-to http://localhost:3000/webhooks

  # Forward specific events only
  revkeen webhooks listen \
    --forward-to http://localhost:3000/webhooks \
    --events subscription.created,payment.completed`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if forwardTo == "" {
				return fmt.Errorf("--forward-to is required")
			}

			fmt.Printf("Listening for webhook events...\n")
			fmt.Printf("Forwarding to: %s\n", forwardTo)
			if len(events) > 0 {
				fmt.Printf("Events filter: %v\n", events)
			}
			fmt.Println("Ready! Waiting for events... (press Ctrl+C to stop)")

			// TODO: Establish SSE connection to RevKeen API,
			// receive events, forward to local URL, display in terminal
			select {} // Block until interrupted
		},
	}

	cmd.Flags().StringVar(&forwardTo, "forward-to", "", "Local URL to forward events to (required)")
	cmd.Flags().StringSliceVar(&events, "events", nil, "Comma-separated list of event types to listen for")
	_ = cmd.MarkFlagRequired("forward-to")

	return cmd
}

func newWebhooksListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List registered webhook endpoints",
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Call GET /v2/webhook-endpoints and display
			fmt.Println("Webhook endpoints:")
			fmt.Println("  (fetching...)")
			return nil
		},
	}
}
