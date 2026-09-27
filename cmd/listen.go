package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/revkeen/cli/internal/config"
	"github.com/revkeen/cli/internal/listen"
	"github.com/spf13/cobra"
)

func newListenCmd() *cobra.Command {
	var forwardTo string
	var events []string
	var skipVerify bool

	cmd := &cobra.Command{
		Use:   "listen",
		Short: "Forward RevKeen webhook events to a local URL",
		Long: `Connect to RevKeen and forward webhook events to your local server.

Requires authentication (revkeen login or REVKEEN_API_KEY). Prints a
session signing secret you can use to verify X-Revkeen-Signature locally.

Also available as: revkeen webhooks listen`,
		Example: `  revkeen listen --forward-to http://localhost:3000/webhooks
  revkeen listen --forward-to http://127.0.0.1:4242/hooks --events invoice.paid,payment.succeeded
  revkeen trigger payment.succeeded`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runListen(cmd, forwardTo, events, skipVerify)
		},
	}

	cmd.Flags().StringVar(&forwardTo, "forward-to", "", "Local URL to forward events to (required)")
	cmd.Flags().StringSliceVar(&events, "events", nil, "Comma-separated event types to receive (default: all)")
	cmd.Flags().BoolVar(&skipVerify, "skip-verify", false, "Reserved for future TLS options (no-op)")
	_ = cmd.MarkFlagRequired("forward-to")
	_ = skipVerify

	return cmd
}

func newTriggerCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "trigger [event_type]",
		Short: "Send a test webhook event to active CLI listen sessions",
		Long: `Publish a fixture event to merchants with an active revkeen listen session.

Does not create billing side effects — fixtures go to CLI listeners only.
Examples: payment.succeeded, invoice.paid, customer.created`,
		Example: `  revkeen trigger payment.succeeded
  revkeen trigger invoice.paid`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load()
			auth, err := listenAuth(cmd, cfg)
			if err != nil {
				return err
			}
			origin := config.OriginBaseURL(cfg)
			raw, err := listen.Trigger(cmd.Context(), nil, origin, auth, args[0], nil)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(raw))
			return nil
		},
	}
}

func listenAuth(cmd *cobra.Command, cfg *config.Config) (listen.Auth, error) {
	apiKey := config.ResolveAPIKeyFromCmd(cmd, cfg)
	token := ""
	if cfg != nil {
		token = cfg.Auth.OAuth.AccessToken
	}
	if apiKey == "" && token == "" {
		return listen.Auth{}, fmt.Errorf("not authenticated — run `revkeen login` or set REVKEEN_API_KEY")
	}
	return listen.Auth{APIKey: apiKey, AccessToken: token}, nil
}

func runListen(cmd *cobra.Command, forwardTo string, events []string, _ bool) error {
	forwardTo = strings.TrimSpace(forwardTo)
	if forwardTo == "" {
		return fmt.Errorf("--forward-to is required")
	}
	if !strings.HasPrefix(forwardTo, "http://") && !strings.HasPrefix(forwardTo, "https://") {
		forwardTo = "http://" + forwardTo
	}

	cfg := config.Load()
	auth, err := listenAuth(cmd, cfg)
	if err != nil {
		return err
	}
	origin := config.OriginBaseURL(cfg)
	listenURL := listen.ListenURL(origin, events)

	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "> Connecting to %s …\n", origin)

	var secret string
	return listen.RunSSE(ctx, nil, listenURL, auth,
		func(c listen.ConnectedPayload) error {
			secret = c.SigningSecret
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "> Ready! Your webhook signing secret is %s (^C to quit)\n", secret)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "> Forwarding to %s\n", forwardTo)
			return nil
		},
		func(ev listen.WebhookEvent) error {
			code, err := listen.Forward(context.WithoutCancel(ctx), nil, forwardTo, secret, ev)
			ts := time.Now().Format("15:04:05")
			if err != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "[%s] error forwarding %s → %s: %v\n", ts, ev.Type, forwardTo, err)
				return nil // keep listening
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s → %s [%d]\n", ts, ev.Type, forwardTo, code)
			return nil
		},
	)
}
