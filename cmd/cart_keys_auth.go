package cmd

// REV-8228: `revkeen cart keys issue` authenticates as the signed-in user.
//
// Issuing a Cart key is a person's action: Engine accepts it from the CLI's
// OAuth login (client cli_revkeen, scope cart_keys:write) and decides by the
// member's current role. A secret API key minting new keys lets one leaked
// credential create more, so API-key issuance is being retired; until Engine
// enforces that, the CLI still falls back to an API key, with a warning.

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	revkeen "github.com/revkeen/sdk-go"
	"github.com/spf13/cobra"

	"github.com/revkeen/cli/internal/config"
)

type cartKeysCredential string

const (
	cartKeysCredentialOAuth  cartKeysCredential = "oauth"
	cartKeysCredentialAPIKey cartKeysCredential = "api_key"

	cartKeysSignInMessage = "issuing Cart keys requires signing in — run `revkeen login`"
	cartKeysAPIKeyWarning = "Warning: issuing Cart keys with an API key is being retired. Sign in with `revkeen login` instead."
)

// resolveAccessToken returns the stored OAuth access token when the CLI is
// signed in with OAuth, or "".
func resolveAccessToken(cfg *config.Config) string {
	if cfg == nil || cfg.Auth.Mode != "oauth" {
		return ""
	}
	return strings.TrimSpace(cfg.Auth.OAuth.AccessToken)
}

// newOAuthSDKClient builds the generated client authenticated with an OAuth
// access token (Authorization: Bearer). The SDK's own constructors accept
// only rk_ API keys and send them as x-api-key.
func newOAuthSDKClient(accessToken, baseURL string) (*revkeen.APIClient, error) {
	if accessToken == "" {
		return nil, errors.New(cartKeysSignInMessage)
	}
	sdkCfg := revkeen.NewConfiguration()
	if len(sdkCfg.Servers) == 0 {
		return nil, errors.New("revkeen: generated configuration has no server entry")
	}
	sdkCfg.Servers[0].URL = strings.TrimRight(baseURL, "/")
	sdkCfg.AddDefaultHeader("Authorization", "Bearer "+accessToken)
	sdkCfg.HTTPClient = &http.Client{Timeout: 60 * time.Second}
	return revkeen.NewAPIClient(sdkCfg), nil
}

// newCartKeysSDKClient prefers the signed-in user's OAuth token and falls back
// to an API key. Overridable in tests.
var newCartKeysSDKClient = func(cmd *cobra.Command) (*revkeen.APIClient, cartKeysCredential, error) {
	cfg := config.Load()
	baseURL := config.ResolveBaseURL(cfg)

	if token := resolveAccessToken(cfg); token != "" {
		client, err := newOAuthSDKClient(token, baseURL)
		return client, cartKeysCredentialOAuth, err
	}
	if apiKey := config.ResolveAPIKeyFromCmd(cmd, cfg); apiKey != "" {
		client, err := revkeen.NewClientWithCustomBaseURL(apiKey, baseURL)
		return client, cartKeysCredentialAPIKey, err
	}
	return nil, "", errors.New(cartKeysSignInMessage)
}

// cartKeysActionableError explains a failed issuance for the credential used.
func cartKeysActionableError(err error, credential cartKeysCredential) error {
	var apiErr *revkeen.RequestError
	if !errors.As(err, &apiErr) {
		return err
	}
	switch apiErr.StatusCode {
	case http.StatusUnauthorized:
		if credential == cartKeysCredentialOAuth {
			return fmt.Errorf("your sign-in has expired — run `revkeen login` (%s)", apiErr.Error())
		}
	case http.StatusForbidden:
		if credential == cartKeysCredentialAPIKey {
			return fmt.Errorf("API keys can no longer issue Cart keys — %s (%s)", cartKeysSignInMessage, apiErr.Error())
		}
		switch forbiddenReason(apiErr) {
		case "scope_insufficient":
			return fmt.Errorf("this sign-in cannot issue Cart keys yet — run `revkeen login` again (%s)", apiErr.Error())
		case "role_denied":
			return fmt.Errorf("your role in this merchant cannot issue Cart keys — ask an owner, admin or developer (%s)", apiErr.Error())
		}
	}
	return cartActionableError(err)
}

// forbiddenReason reads error.details.reason from the canonical 403 envelope.
func forbiddenReason(apiErr *revkeen.RequestError) string {
	var envelope struct {
		Error struct {
			Details struct {
				Reason string `json:"reason"`
			} `json:"details"`
		} `json:"error"`
	}
	if json.Unmarshal(apiErr.Body(), &envelope) != nil {
		return ""
	}
	return envelope.Error.Details.Reason
}
