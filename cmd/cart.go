package cmd

// Cart setup/status commands (REV-5152). Hand-written Cobra commands that call
// the generated Go SDK — never the raw-HTTP generic
// handler. Commands are idempotent where the API allows: re-adding an origin
// or re-running webhooks setup returns the existing state instead of failing.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	revkeen "github.com/revkeen/sdk-go"
	"github.com/spf13/cobra"

	"github.com/revkeen/cli/internal/config"
	"github.com/revkeen/cli/internal/output"
	"github.com/revkeen/cli/internal/ui"
)

// newCartSDKClient builds the generated SDK client from the CLI's shared
// auth/base-URL resolution (flag > env > config). Overridable in tests.
var newCartSDKClient = func(cmd *cobra.Command) (*revkeen.APIClient, error) {
	cfg := config.Load()

	apiKey := config.ResolveAPIKeyFromCmd(cmd, cfg)
	if apiKey == "" {
		return nil, fmt.Errorf("no API key configured — run `revkeen auth login` or set REVKEEN_API_KEY")
	}

	baseURL := config.ResolveBaseURL(cfg)
	return revkeen.NewClientWithCustomBaseURL(apiKey, baseURL)
}

// apiErrorCode extracts the machine-readable code from a RequestError and
// normalises it to the canonical lower_snake form.
//
// Two envelope shapes are live at once. REV-6760 made the canonical public
// shape `{"error":{"type":...,"code":"cart_disabled","message":...}}`, which the
// generated parser reads into RequestError.Code. Pre-REV-6760 routes still emit
// the flat `{"error":"CART_DISABLED","message":...}`, which the generated parser
// leaves as `unknown_error` — hence the raw-body fallback. Lower-casing collapses
// the two spellings of the same condition onto one switch (REV-6759: before this,
// only the SCREAMING_SNAKE legacy spelling matched, so a canonical 403
// `cart_disabled` fell through to the generic "wrong key kind" advice).
func apiErrorCode(apiErr *revkeen.RequestError) string {
	if apiErr.Code != "" && apiErr.Code != "unknown_error" {
		return strings.ToLower(apiErr.Code)
	}
	var envelope struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(apiErr.Body(), &envelope) == nil && envelope.Error != "" {
		return strings.ToLower(envelope.Error)
	}
	return ""
}

// cartActionableError rewrites known API failures into next-action guidance.
func cartActionableError(err error) error {
	var apiErr *revkeen.RequestError
	if !errors.As(err, &apiErr) {
		return err
	}
	switch apiErrorCode(apiErr) {
	case "cart_disabled":
		return fmt.Errorf(
			"cart is not enabled for this merchant — install the RevKeen Cart app from the Marketplace, then re-run this command (%s)",
			apiErr.Error(),
		)
	case "origin_not_allowed":
		return fmt.Errorf(
			"this origin is not registered — run `revkeen cart origins add <url>` first (%s)",
			apiErr.Error(),
		)
	}
	if apiErr.StatusCode == 403 {
		return fmt.Errorf(
			"forbidden — this command needs a SECRET Cart key with the apps:write scope; publishable keys cannot manage Cart setup (%s)",
			apiErr.Error(),
		)
	}
	return err
}

func newCartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cart",
		Short: "Set up and inspect RevKeen Cart for headless storefronts",
	}

	cmd.AddCommand(newCartStatusCmd())
	cmd.AddCommand(newCartKeysCmd())
	cmd.AddCommand(newCartOriginsCmd())
	cmd.AddCommand(newCartWebhooksCmd())

	return cmd
}

func newCartStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show Cart integration readiness (activation, keys, origins, webhooks, products)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ui.ApplyNoColor(cmd)
			interactive := ui.Interactive(cmd)

			var res *revkeen.StorefrontStatusResponse
			_, err := ui.WithSpinnerResult(interactive, "Fetching Cart status…", func() (interface{}, error) {
				client, err := newCartSDKClient(cmd)
				if err != nil {
					return nil, err
				}
				out, _, err := client.StorefrontAPI.StorefrontStatusGet(context.Background()).Execute()
				if err != nil {
					return nil, cartActionableError(err)
				}
				res = out
				return out, nil
			})
			if err != nil {
				return err
			}

			if output.ResolveFormat(cmd) != output.FormatTable {
				return output.Print(cmd, res)
			}

			data := res.Data
			rows := make([]ui.ChecklistRow, 0, len(data.Checks))
			for _, check := range data.Checks {
				row := ui.ChecklistRow{
					ID: string(check.Id), Status: string(check.Status), Message: check.Message,
				}
				if check.NextAction != nil {
					row.NextAction = *check.NextAction
				}
				rows = append(rows, row)
			}
			ui.PrintChecklist(cmd.OutOrStdout(), "Cart integration", data.Ready, rows)
			return nil
		},
	}
}

func newCartKeysCmd() *cobra.Command {
	keys := &cobra.Command{
		Use:   "keys",
		Short: "Manage managed Cart API keys",
	}

	issue := &cobra.Command{
		Use:   "issue",
		Short: "Ensure the managed publishable + secret Cart keys exist (idempotent)",
		Long: `Provisions the managed Cart API keys if they do not exist yet.
Key material is printed ONCE, at first issuance only — store it immediately.
Re-running reports existing key status without any secret material.

Requires signing in with ` + "`revkeen login`" + `: your role in the merchant decides
whether you can issue keys. Issuing with an API key is being retired.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, credential, err := newCartKeysSDKClient(cmd)
			if err != nil {
				return err
			}
			if credential == cartKeysCredentialAPIKey {
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), cartKeysAPIKeyWarning)
			}
			res, _, err := client.CartAPIKeysAPI.CartApiKeysEnsure(context.Background()).Execute()
			if err != nil {
				return cartKeysActionableError(err, credential)
			}

			if output.ResolveFormat(cmd) != output.FormatTable {
				return output.Print(cmd, res)
			}

			data := res.Data
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Publishable key: present=%t active=%t\n", data.Publishable.Present, data.Publishable.Active); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Secret key:      present=%t active=%t\n", data.Secret.Present, data.Secret.Active); err != nil {
				return err
			}
			if data.PublishableApiKey != nil && *data.PublishableApiKey != "" {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "\nNEW publishable key (shown once): %s\n", *data.PublishableApiKey); err != nil {
					return err
				}
			}
			if data.SecretApiKey != nil && *data.SecretApiKey != "" {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "NEW secret key (shown once, store it now): %s\n", *data.SecretApiKey); err != nil {
					return err
				}
			}
			if (data.PublishableApiKey == nil || *data.PublishableApiKey == "") &&
				(data.SecretApiKey == nil || *data.SecretApiKey == "") {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), "\nKeys already provisioned — no key material is re-shown. Use `revkeen cart keys` rotation via the dashboard if a key leaked."); err != nil {
					return err
				}
			}
			return nil
		},
	}

	keys.AddCommand(issue)
	return keys
}

func newCartOriginsCmd() *cobra.Command {
	origins := &cobra.Command{
		Use:   "origins",
		Short: "Manage storefront origins allowed for publishable-key browser calls",
	}

	list := &cobra.Command{
		Use:   "list",
		Short: "List registered storefront origins",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newCartSDKClient(cmd)
			if err != nil {
				return err
			}
			res, _, err := client.StorefrontAPI.StorefrontOriginsList(context.Background()).Execute()
			if err != nil {
				return cartActionableError(err)
			}
			if output.ResolveFormat(cmd) != output.FormatTable {
				return output.Print(cmd, res)
			}
			if len(res.Data) == 0 {
				_, err := fmt.Fprintln(cmd.OutOrStdout(), "No storefront origins registered — browser publishable-key calls fail closed until one is added.")
				return err
			}
			for _, origin := range res.Data {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s  %s\n", origin.Id, origin.Origin); err != nil {
					return err
				}
			}
			return nil
		},
	}

	add := &cobra.Command{
		Use:   "add <url>",
		Short: "Register a storefront origin (idempotent — re-adding returns the existing entry)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newCartSDKClient(cmd)
			if err != nil {
				return err
			}
			ctx := context.Background()
			res, _, err := client.StorefrontAPI.StorefrontOriginsCreate(ctx).
				StorefrontOriginCreateRequest(revkeen.StorefrontOriginCreateRequest{Origin: args[0]}).
				Execute()
			if err != nil {
				var apiErr *revkeen.RequestError
				if errors.As(err, &apiErr) && apiErr.StatusCode == 409 {
					// Idempotent success: surface the existing registration.
					existing, _, listErr := client.StorefrontAPI.StorefrontOriginsList(ctx).Execute()
					if listErr == nil {
						for _, origin := range existing.Data {
							if strings.EqualFold(strings.TrimRight(origin.Origin, "/"), strings.TrimRight(strings.ToLower(args[0]), "/")) {
								if output.ResolveFormat(cmd) != output.FormatTable {
									return output.Print(cmd, map[string]any{"data": origin, "already_registered": true})
								}
								if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Already registered: %s (%s)\n", origin.Origin, origin.Id); err != nil {
									return err
								}
								return nil
							}
						}
					}
				}
				return cartActionableError(err)
			}
			if output.ResolveFormat(cmd) != output.FormatTable {
				return output.Print(cmd, res)
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Registered: %s (%s)\n", res.Data.Origin, res.Data.Id)
			return err
		},
	}

	remove := &cobra.Command{
		Use:   "remove <url-or-id>",
		Short: "Remove a registered storefront origin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newCartSDKClient(cmd)
			if err != nil {
				return err
			}
			ctx := context.Background()
			target := args[0]

			originID := target
			if strings.Contains(target, "://") {
				existing, _, err := client.StorefrontAPI.StorefrontOriginsList(ctx).Execute()
				if err != nil {
					return cartActionableError(err)
				}
				originID = ""
				for _, origin := range existing.Data {
					if strings.EqualFold(strings.TrimRight(origin.Origin, "/"), strings.TrimRight(strings.ToLower(target), "/")) {
						originID = origin.Id
						break
					}
				}
				if originID == "" {
					return fmt.Errorf("origin %q is not registered", target)
				}
			}

			if _, _, err := client.StorefrontAPI.StorefrontOriginsDelete(ctx, originID).Execute(); err != nil {
				return cartActionableError(err)
			}
			if output.ResolveFormat(cmd) != output.FormatTable {
				return output.Print(cmd, map[string]any{"data": map[string]any{"id": originID, "deleted": true}})
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Removed origin %s\n", originID)
			return err
		},
	}

	origins.AddCommand(add, list, remove)
	return origins
}

func newCartWebhooksCmd() *cobra.Command {
	webhooks := &cobra.Command{
		Use:   "webhooks",
		Short: "Configure Cart webhook endpoints",
	}

	var setupURL string
	setup := &cobra.Command{
		Use:   "setup",
		Short: "Ensure a webhook endpoint exists for the given URL (idempotent)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if setupURL == "" {
				return fmt.Errorf("--url is required")
			}
			client, err := newCartSDKClient(cmd)
			if err != nil {
				return err
			}
			ctx := context.Background()

			existing, _, err := client.WebhookEndpointsAPI.WebhookEndpointsList(ctx).Execute()
			if err != nil {
				return cartActionableError(err)
			}
			if existing != nil {
				for _, endpoint := range existing.Data {
					if strings.EqualFold(strings.TrimRight(endpoint.Url, "/"), strings.TrimRight(setupURL, "/")) {
						if output.ResolveFormat(cmd) != output.FormatTable {
							return output.Print(cmd, map[string]any{"data": endpoint, "already_configured": true})
						}
						if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Already configured: %s (%s)\n", setupURL, endpoint.Id); err != nil {
							return err
						}
						return nil
					}
				}
			}

			res, _, err := client.WebhookEndpointsAPI.WebhookEndpointsCreate(ctx).
				WebhookEndpointsCreateRequest(revkeen.WebhookEndpointsCreateRequest{Url: setupURL}).
				Execute()
			if err != nil {
				return cartActionableError(err)
			}
			if output.ResolveFormat(cmd) != output.FormatTable {
				return output.Print(cmd, res)
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Webhook endpoint created for %s\n", setupURL); err != nil {
				return err
			}
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), "The signing secret (if returned) is shown once — store it now."); err != nil {
				return err
			}
			return nil
		},
	}
	setup.Flags().StringVar(&setupURL, "url", "", "Webhook endpoint URL (required)")
	_ = setup.MarkFlagRequired("url")

	webhooks.AddCommand(setup)
	return webhooks
}
