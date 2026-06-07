package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

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
			url := cfg.Settings.BaseURL + path

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

			// Auth
			apiKey := config.ResolveAPIKey(cfg)
			if apiKey != "" {
				req.Header.Set("x-api-key", apiKey)
			} else if cfg.Auth.OAuth.AccessToken != "" {
				req.Header.Set("Authorization", "Bearer "+cfg.Auth.OAuth.AccessToken)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return fmt.Errorf("request failed: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			respBody, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("failed to read response: %w", err)
			}

			if agent {
				output.PrintRaw(respBody)
				if resp.StatusCode >= 400 {
					return fmt.Errorf("HTTP %d", resp.StatusCode)
				}
				return nil
			}

			// Normal mode: pretty-print JSON
			var prettyJSON map[string]interface{}
			if json.Unmarshal(respBody, &prettyJSON) == nil {
				formatted, _ := json.MarshalIndent(prettyJSON, "", "  ")
				fmt.Println(string(formatted))
			} else {
				fmt.Println(string(respBody))
			}

			if resp.StatusCode >= 400 {
				return fmt.Errorf("HTTP %d", resp.StatusCode)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&data, "data", "d", "", "Request body (JSON string)")

	return cmd
}
