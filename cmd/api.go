package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/revkeen/cli/internal/apierr"
	"github.com/revkeen/cli/internal/config"
	"github.com/revkeen/cli/internal/output"
	"github.com/spf13/cobra"
)

const apiVersionHeader = "2026-05-01"

func newAPICmd() *cobra.Command {
	var data string

	cmd := &cobra.Command{
		Use:   "api [method] [path]",
		Short: "Make raw API requests",
		Long: `Make authenticated HTTP requests to any RevKeen API endpoint.
This is an escape hatch for endpoints not exposed as named CLI commands.`,
		Example: `  # GET request
  revkeen api GET /v2/customers

  # POST request with JSON body
  revkeen api POST /v2/invoices -d '{"customerId":"cus_xxx","amount":5000}'

  # DELETE request
  revkeen api DELETE /v2/webhook-endpoints/we_xxx

  # Agent mode — raw JSON passthrough
  revkeen api GET /v2/customers --agent`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			method := strings.ToUpper(args[0])
			path := args[1]
			agent := output.IsAgent(cmd)

			cfg := config.Load()
			base := strings.TrimRight(config.OriginBaseURL(cfg), "/")
			if !strings.HasPrefix(path, "/") {
				path = "/" + path
			}
			url := base + path

			var body io.Reader
			if data != "" {
				body = strings.NewReader(data)
			}

			req, err := http.NewRequest(method, url, body)
			if err != nil {
				return fmt.Errorf("failed to create request: %w", err)
			}

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json")
			req.Header.Set("RevKeen-Version", apiVersionHeader)

			apiKey := config.ResolveAPIKeyFromCmd(cmd, cfg)
			if apiKey != "" {
				req.Header.Set("x-api-key", apiKey)
			} else if cfg.Auth.OAuth.AccessToken != "" {
				req.Header.Set("Authorization", "Bearer "+cfg.Auth.OAuth.AccessToken)
			}

			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("request failed: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			respBody, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("failed to read response: %w", err)
			}

			// REV-6759: `revkeen api` is a raw escape hatch, so the body is still
			// printed verbatim — but the returned error now carries the decoded
			// REV-6760 auth contract (401 vs 403 vs 429 + Retry-After) so the
			// failure is actionable without re-parsing the envelope by hand.
			var apiError error
			if resp.StatusCode >= 400 {
				apiError = apierr.Parse(resp.StatusCode, resp.Header, respBody)
			}

			if agent {
				output.PrintRaw(respBody)
				return apiError
			}

			var prettyJSON map[string]interface{}
			if json.Unmarshal(respBody, &prettyJSON) == nil {
				formatted, _ := json.MarshalIndent(prettyJSON, "", "  ")
				fmt.Println(string(formatted))
			} else {
				fmt.Println(string(respBody))
			}

			return apiError
		},
	}

	cmd.Flags().StringVarP(&data, "data", "d", "", "Request body (JSON string)")

	return cmd
}
