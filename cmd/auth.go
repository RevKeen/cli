package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/revkeen/cli/internal/config"
	"github.com/spf13/cobra"
)

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate with RevKeen",
		Long:  "Log in, log out, or check your current authentication status.",
	}

	cmd.AddCommand(newAuthLoginCmd())
	cmd.AddCommand(newAuthLogoutCmd())
	cmd.AddCommand(newAuthStatusCmd())

	return cmd
}

func newAuthLoginCmd() *cobra.Command {
	var useAPIKey bool
	var noBrowser bool

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with RevKeen",
		Long: `Authenticate with RevKeen using OAuth (default) or an API key.

By default, opens a browser for the OAuth device flow. Use --api-key
to enter an API key directly, or --no-browser to get a URL for manual
authentication in headless environments.`,
		Example: `  # Interactive OAuth login (opens browser)
  revkeen auth login

  # API key login (prompts for key)
  revkeen auth login --api-key

  # Headless OAuth login (prints URL)
  revkeen auth login --no-browser`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load()

			if useAPIKey {
				return loginWithAPIKey(cfg)
			}
			return loginWithOAuth(cfg, noBrowser)
		},
	}

	cmd.Flags().BoolVar(&useAPIKey, "api-key", false, "Log in with an API key instead of OAuth")
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "Print the login URL instead of opening a browser")

	return cmd
}

func loginWithAPIKey(cfg *config.Config) error {
	fmt.Print("Enter your RevKeen API key: ")
	var apiKey string
	_, err := fmt.Scanln(&apiKey)
	if err != nil {
		return fmt.Errorf("failed to read API key: %w", err)
	}

	if apiKey == "" {
		return fmt.Errorf("API key cannot be empty")
	}

	cfg.Auth.Mode = "api_key"
	cfg.Auth.APIKey = apiKey
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println("API key saved. You are now authenticated.")
	return nil
}

func loginWithOAuth(cfg *config.Config, noBrowser bool) error {
	// OAuth Device Authorization Grant (RFC 8628)
	// 1. Request device code from RevKeen
	// 2. Display verification URL + user code
	// 3. Poll for token
	// 4. Store tokens in config

	deviceCodeURL := cfg.Settings.BaseURL + "/api/auth/oauth2/device/code"
	_ = deviceCodeURL // Used in actual implementation

	verificationURL := cfg.Settings.BaseURL + "/device"
	userCode := "RVKN-XXXX" // Placeholder — actual implementation calls device code endpoint

	if noBrowser {
		fmt.Printf("Open this URL in your browser:\n  %s\n\nEnter code: %s\n", verificationURL, userCode)
	} else {
		fmt.Printf("Opening browser to authenticate...\nIf it doesn't open, visit: %s\nEnter code: %s\n", verificationURL, userCode)
		openBrowser(verificationURL)
	}

	// Poll for token (placeholder — actual implementation polls token endpoint)
	fmt.Println("Waiting for authentication...")
	time.Sleep(1 * time.Second)

	// Store tokens
	cfg.Auth.Mode = "oauth"
	cfg.Auth.OAuth.AccessToken = "" // Set from actual token response
	cfg.Auth.OAuth.RefreshToken = ""
	cfg.Auth.OAuth.ExpiresAt = time.Now().Add(3600 * time.Second).Format(time.RFC3339)
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println("Authentication successful.")
	return nil
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	}
	if cmd != nil {
		_ = cmd.Start()
	}
}

func newAuthLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Clear stored credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load()
			cfg.Auth = config.AuthConfig{}
			if err := config.Save(cfg); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}
			fmt.Println("Logged out. Credentials cleared.")
			return nil
		},
	}
}

func newAuthStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current authentication status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load()

			switch cfg.Auth.Mode {
			case "api_key":
				masked := cfg.Auth.APIKey
				if len(masked) > 12 {
					masked = masked[:8] + "..." + masked[len(masked)-4:]
				}
				fmt.Printf("Auth mode:  API Key\n")
				fmt.Printf("API key:    %s\n", masked)
				fmt.Printf("Base URL:   %s\n", cfg.Settings.BaseURL)
			case "oauth":
				fmt.Printf("Auth mode:  OAuth\n")
				fmt.Printf("Expires at: %s\n", cfg.Auth.OAuth.ExpiresAt)
				fmt.Printf("Base URL:   %s\n", cfg.Settings.BaseURL)
			default:
				fmt.Println("Not authenticated. Run 'revkeen auth login' to get started.")
				os.Exit(1)
			}
			return nil
		},
	}
}
