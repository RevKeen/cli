package cmd

// revkeen doctor (REV-5153). One repeatable, read-only health check for a
// RevKeen Cart integration: remote readiness comes from the
// /v2/storefront/status endpoint plus the webhook-endpoint event list (via
// the generated Go SDK); env checks come from local project inspection only.
// No mutations, no secret material in any output — env checks report
// presence, never values. The Sanity project checks were removed with the
// Sanity integration (REV-8396); --cms now exits with the replacement message.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	revkeen "github.com/revkeen/sdk-go"
	"github.com/spf13/cobra"

	"github.com/revkeen/cli/internal/output"
	"github.com/revkeen/cli/internal/ui"
)

const checkoutCompletedEvent = "commerce.checkout.completed"

type doctorCheck struct {
	ID         string `json:"id"`
	Status     string `json:"status"` // pass | warn | fail
	Message    string `json:"message"`
	NextAction string `json:"next_action,omitempty"`
}

type doctorReport struct {
	Object string        `json:"object"`
	Ready  bool          `json:"ready"`
	Checks []doctorCheck `json:"checks"`
}

func newDoctorCmd() *cobra.Command {
	var cmsTarget string
	var projectPath string

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Health-check a RevKeen Cart integration",
		Example: `  revkeen doctor
  revkeen doctor --project-path ./storefront --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("cms") {
				cmd.SilenceErrors = true
				cmd.SilenceUsage = true
				return errSanityIntegrationReplaced
			}

			ui.ApplyNoColor(cmd)
			interactive := ui.Interactive(cmd)

			var report doctorReport
			_, err := ui.WithSpinnerResult(interactive, "Checking RevKeen Cart integration…", func() (interface{}, error) {
				r := doctorReport{Object: "doctor_report"}
				r.Checks = append(r.Checks, runRemoteDoctorChecks(cmd)...)
				r.Checks = append(r.Checks, envPresenceChecks(projectPath)...)
				r.Ready = true
				for _, check := range r.Checks {
					if check.Status == "fail" {
						r.Ready = false
						break
					}
				}
				report = r
				return r, nil
			})
			if err != nil {
				return err
			}

			if output.ResolveFormat(cmd) != output.FormatTable {
				return output.Print(cmd, report)
			}

			rows := make([]ui.ChecklistRow, 0, len(report.Checks))
			for _, check := range report.Checks {
				rows = append(rows, ui.ChecklistRow{
					ID: check.ID, Status: check.Status, Message: check.Message, NextAction: check.NextAction,
				})
			}
			ui.PrintChecklist(cmd.OutOrStdout(), "RevKeen Cart integration", report.Ready, rows)
			return nil
		},
	}

	cmd.Flags().StringVar(&cmsTarget, "cms", "", "Removed. Any value exits with an error.")
	_ = cmd.Flags().MarkHidden("cms")
	cmd.Flags().StringVar(&projectPath, "project-path", ".", "Project directory to inspect")
	return cmd
}

// runRemoteDoctorChecks maps the integration-status endpoint (and the webhook
// event list) into doctor rows. API failures degrade to a single fail row so
// the local checks still run.
func runRemoteDoctorChecks(cmd *cobra.Command) []doctorCheck {
	client, err := newCartSDKClient(cmd)
	if err != nil {
		return []doctorCheck{{
			ID: "api-auth", Status: "fail", Message: err.Error(),
			NextAction: "revkeen auth login   (or set REVKEEN_API_KEY to your secret Cart key)",
		}}
	}

	ctx := context.Background()
	status, _, err := client.StorefrontAPI.StorefrontStatusGet(ctx).Execute()
	if err != nil {
		return []doctorCheck{{
			ID: "integration-status", Status: "fail",
			Message:    cartActionableError(err).Error(),
			NextAction: "Fix the API error above, then re-run revkeen doctor",
		}}
	}

	var checks []doctorCheck
	for _, check := range status.Data.Checks {
		row := doctorCheck{
			ID:      string(check.Id),
			Status:  string(check.Status),
			Message: check.Message,
		}
		if check.NextAction != nil {
			row.NextAction = *check.NextAction
		}
		checks = append(checks, row)
	}

	checks = append(checks, checkoutCompletedSubscriptionCheck(ctx, client))
	return checks
}

func checkoutCompletedSubscriptionCheck(ctx context.Context, client *revkeen.APIClient) doctorCheck {
	endpoints, _, err := client.WebhookEndpointsAPI.WebhookEndpointsList(ctx).Execute()
	if err != nil || endpoints == nil {
		return doctorCheck{
			ID: "checkout-completed-event", Status: "warn",
			Message: "Could not verify webhook event subscriptions.",
		}
	}
	for _, endpoint := range endpoints.Data {
		if endpoint.Status != "active" {
			continue
		}
		for _, event := range endpoint.EnabledEvents {
			if event == checkoutCompletedEvent || event == "*" {
				return doctorCheck{
					ID: "checkout-completed-event", Status: "pass",
					Message: fmt.Sprintf("%s is delivered to %s.", checkoutCompletedEvent, endpoint.Url),
				}
			}
		}
	}
	return doctorCheck{
		ID: "checkout-completed-event", Status: "warn",
		Message:    fmt.Sprintf("No active webhook endpoint subscribes to %s.", checkoutCompletedEvent),
		NextAction: "revkeen cart webhooks setup --url <your-endpoint>",
	}
}

var envAssignmentPattern = regexp.MustCompile(`(?m)^\s*([A-Z0-9_]+)\s*=`)

// envPresenceChecks reports which expected env vars are assigned in local env
// files or the process environment. Presence only — values are never read
// into the report.
func envPresenceChecks(projectPath string) []doctorCheck {
	defined := map[string]bool{}
	for _, file := range []string{".env", ".env.local", ".env.development"} {
		content, err := os.ReadFile(filepath.Join(projectPath, file))
		if err != nil {
			continue
		}
		for _, match := range envAssignmentPattern.FindAllStringSubmatch(string(content), -1) {
			defined[match[1]] = true
		}
	}

	expected := []struct {
		name     string
		severity string
	}{
		{"NEXT_PUBLIC_REVKEEN_PUBLISHABLE_KEY", "fail"},
		{"NEXT_PUBLIC_REVKEEN_BASE_URL", "warn"},
		{"REVKEEN_SECRET_KEY", "warn"},
	}

	var checks []doctorCheck
	for _, entry := range expected {
		present := defined[entry.name] || os.Getenv(entry.name) != ""
		if present {
			checks = append(checks, doctorCheck{
				ID: "env-" + strings.ToLower(entry.name), Status: "pass",
				Message: entry.name + " is set.",
			})
			continue
		}
		checks = append(checks, doctorCheck{
			ID: "env-" + strings.ToLower(entry.name), Status: entry.severity,
			Message:    entry.name + " is not set in .env files or the environment.",
			NextAction: "Add it to .env.local (Vercel: vercel env add " + entry.name + ")",
		})
	}
	return checks
}
