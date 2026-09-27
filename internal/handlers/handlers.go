package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/lispyclouds/climate"
	"github.com/pb33f/libopenapi"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/revkeen/cli/internal/apierr"
	"github.com/revkeen/cli/internal/config"
	"github.com/revkeen/cli/internal/output"
	"github.com/spf13/cobra"
)

const apiVersion = "2026-05-01"

// HTTPClient is the shared client for climate-bootstrapped commands.
var HTTPClient = &http.Client{Timeout: 30 * time.Second}

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
	base := config.ResolveBaseURL(cfg)
	path := data.Path
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	rawURL := base + path
	agent := output.IsAgent(cmd)

	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid request URL: %w", err)
	}
	q := u.Query()
	for _, qp := range data.QueryParams {
		val, set := getFlagValue(cmd, qp)
		if set {
			q.Set(qp.Name, val)
		}
	}
	u.RawQuery = q.Encode()

	var body io.Reader
	if data.RequestBodyParam != nil {
		bodyStr, _ := cmd.Flags().GetString(data.RequestBodyParam.Name)
		if bodyStr != "" {
			body = strings.NewReader(bodyStr)
		}
	}

	req, err := http.NewRequest(strings.ToUpper(data.Method), u.String(), body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("RevKeen-Version", apiVersion)

	for _, hp := range data.HeaderParams {
		val, set := getFlagValue(cmd, hp)
		if set {
			req.Header.Set(hp.Name, val)
		}
	}

	apiKey := config.ResolveAPIKeyFromCmd(cmd, cfg)
	if apiKey != "" {
		req.Header.Set("x-api-key", apiKey)
	} else if cfg.Auth.OAuth.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.Auth.OAuth.AccessToken)
	}

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		// REV-6759: decode the public auth-error contract (REV-6760) so 401,
		// 403 and 429 are distinguishable and 429 surfaces Retry-After. Only
		// server-supplied fields are rendered — the presented credential is
		// never echoed back.
		apiError := apierr.Parse(resp.StatusCode, resp.Header, respBody)

		if agent {
			// Agent mode stays a raw passthrough: machine consumers parse the
			// canonical envelope themselves.
			output.PrintRaw(respBody)
			return apiError
		}

		message := fmt.Sprintf("HTTP %d\n%s", resp.StatusCode, string(respBody))
		if guidance := apiError.Guidance(); guidance != "" {
			message = message + "\n\n" + guidance
		}
		output.PrintError(cmd, resp.StatusCode, message)
		return apiError
	}

	if agent {
		output.PrintRaw(respBody)
		return nil
	}

	var result interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		fmt.Println(string(respBody))
		return nil
	}

	return output.Print(cmd, result)
}

// getFlagValue retrieves a flag value as a string based on its OpenAPI type.
// The second return value is false when the flag was not explicitly changed
// (or cannot be read), so zero values are still sent when the user set them.
func getFlagValue(cmd *cobra.Command, param climate.ParamMeta) (string, bool) {
	flags := cmd.Flags()
	if !flags.Changed(param.Name) {
		return "", false
	}

	switch param.Type {
	case climate.String:
		val, err := flags.GetString(param.Name)
		if err != nil {
			return "", false
		}
		return val, true
	case climate.Integer:
		val, err := flags.GetInt(param.Name)
		if err != nil {
			return "", false
		}
		return strconv.Itoa(val), true
	case climate.Number:
		val, err := flags.GetFloat64(param.Name)
		if err != nil {
			return "", false
		}
		return strconv.FormatFloat(val, 'g', -1, 64), true
	case climate.Boolean:
		val, err := flags.GetBool(param.Name)
		if err != nil {
			return "", false
		}
		return strconv.FormatBool(val), true
	default:
		return "", false
	}
}
