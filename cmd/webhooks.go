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
	// Alias of top-level `revkeen listen` (REV-2978).
	listen := newListenCmd()
	listen.Use = "listen"
	listen.Short = "Forward webhook events to a local URL (alias of revkeen listen)"
	return listen
}

func newWebhooksListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List registered webhook endpoints",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("webhooks list is not implemented yet — use `revkeen webhook-endpoints list`")
		},
	}
}
