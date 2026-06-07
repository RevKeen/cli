package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/lispyclouds/climate"
	"github.com/pb33f/libopenapi"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/revkeen/cli/internal/config"
	"github.com/revkeen/cli/internal/output"
	"github.com/spf13/cobra"
)

const apiVersion = "2026-05-01"

// BuildHandlerMap iterates over every operation in the OpenAPI model
// and registers GenericHandler for each operationId. Climate skips
// operations without a handler, so we must register all of them.
func BuildHandlerMap(model *libopenapi.DocumentModel[v3.Document]) map[string]climate.HandlerCobra {
	handlers := make(map[string]climate.HandlerCobra)

	for _, item := range model.Model.Paths.PathItems.FromOldest() {
		for _, op := range item.GetOperations().FromOldest() {
			if op.OperationId != "" {
				handlers[op.OperationId] = GenericHandler
			}
		}
	}

	return handlers
}

// GenericHandler handles any climate-bootstrapped command by making an
// authenticated HTTP request and formatting the response.
func GenericHandler(cmd *cobra.Command, args []string, data climate.HandlerData) error {
	cfg := config.Load()
	url := cfg.Settings.BaseURL + data.Path
	agent := output.IsAgent(cmd)

	// Build query string from query params
	var queryParts []string
	for _, qp := range data.QueryParams {
		val := getFlagValue(cmd, qp)
		if val != "" {
			queryParts = append(queryParts, qp.Name+"="+val)
		}
	}
	if len(queryParts) > 0 {
		url += "?" + strings.Join(queryParts, "&")
	}

	// Get request body from the climate-data flag (if present)
	var body io.Reader
	if data.RequestBodyParam != nil {
		bodyStr, _ := cmd.Flags().GetString(data.RequestBodyParam.Name)
		if bodyStr != "" {
			body = strings.NewReader(bodyStr)
		}
	}

	req, err := http.NewRequest(data.Method, url, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("RevKeen-Version", apiVersion)

	// Set header params from flags
	for _, hp := range data.HeaderParams {
		val := getFlagValue(cmd, hp)
		if val != "" {
			req.Header.Set(hp.Name, val)
		}
	}

	// Authenticate
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

	if resp.StatusCode >= 400 {
		if agent {
			output.PrintRaw(respBody)
			return fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		output.PrintError(cmd, resp.StatusCode, fmt.Sprintf("HTTP %d\n%s", resp.StatusCode, string(respBody)))
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// Agent mode: raw passthrough (no unmarshal/remarshal)
	if agent {
		output.PrintRaw(respBody)
		return nil
	}

	// Normal mode: parse and format
	var result interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		fmt.Println(string(respBody))
		return nil
	}

	return output.Print(cmd, result)
}

// getFlagValue retrieves a flag value as a string based on its OpenAPI type.
func getFlagValue(cmd *cobra.Command, param climate.ParamMeta) string {
	flags := cmd.Flags()

	switch param.Type {
	case climate.String:
		val, _ := flags.GetString(param.Name)
		return val
	case climate.Integer:
		val, err := flags.GetInt(param.Name)
		if err != nil || val == 0 {
			return ""
		}
		return strconv.Itoa(val)
	case climate.Number:
		val, err := flags.GetFloat64(param.Name)
		if err != nil || val == 0 {
			return ""
		}
		return strconv.FormatFloat(val, 'g', -1, 64)
	case climate.Boolean:
		val, _ := flags.GetBool(param.Name)
		if !val {
			return ""
		}
		return "true"
	default:
		return ""
	}
}
