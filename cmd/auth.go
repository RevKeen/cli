package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/revkeen/cli/internal/config"
	"github.com/revkeen/cli/internal/deviceauth"
	"github.com/revkeen/cli/internal/pkceauth"
	"github.com/revkeen/cli/internal/ui"
	"github.com/spf13/cobra"
)

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate with RevKeen",
		Long:  "Log in, log out, or check your current authentication status.",
	}

	cmd.AddCommand(newLoginCmd())
	cmd.AddCommand(newAuthLogoutCmd())
	cmd.AddCommand(newAuthStatusCmd())

	return cmd
}

// newLoginCmd is shared by `revkeen login` and `revkeen auth login` (REV-2974).
func newLoginCmd() *cobra.Command {
	var useAPIKey bool
	var useDevice bool
	var withToken bool
	var noBrowser bool

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with RevKeen",
		Long: `Authenticate with RevKeen.

Default (interactive): OAuth authorization-code + PKCE with a local
127.0.0.1 callback — opens the browser for sign-in and consent.

Alternatives:
  --device       OAuth device grant (SSH / Codespaces / headless)
  --api-key      Secret API key (rk_live_* / rk_sandbox_*)
  --with-token   Read a bearer token or API key from stdin / REVKEEN_ACCESS_TOKEN

API keys and environment variables remain fully supported.`,
		Example: `  # PKCE browser login (default)
  revkeen login
  revkeen auth login

  # Device flow (SSH / no browser automation)
  revkeen login --device
  revkeen login --device --no-browser

  # API key
  revkeen login --api-key

  # Token from env or stdin
  REVKEEN_ACCESS_TOKEN=rkoa_... revkeen login --with-token
  echo 'rk_live_...' | revkeen login --with-token`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load()

			selected := 0
			if useAPIKey {
				selected++
			}
			if useDevice {
				selected++
			}
			if withToken {
				selected++
			}
			if selected > 1 {
				return fmt.Errorf("use only one of --api-key, --device, or --with-token")
			}

			switch {
			case useAPIKey:
				return loginWithAPIKeyInteractive(cmd, cfg)
			case withToken:
				return loginWithToken(cmd, cfg)
			case useDevice:
				return loginWithDevice(cmd, cfg, noBrowser)
			default:
				// Default: PKCE. Device is explicit via --device (REV-2974).
				return loginWithPKCE(cmd, cfg, noBrowser)
			}
		},
	}

	cmd.Flags().BoolVar(&useAPIKey, "api-key", false, "Log in with an API key")
	cmd.Flags().BoolVar(&useDevice, "device", false, "Use OAuth device authorization grant (RFC 8628)")
	cmd.Flags().BoolVar(&withToken, "with-token", false, "Read bearer token or API key from REVKEEN_ACCESS_TOKEN or stdin")
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "Print the authorize/verification URL instead of opening a browser")

	return cmd
}

func loginWithAPIKeyInteractive(cmd *cobra.Command, cfg *config.Config) error {
	var apiKey string

	if ui.Interactive(cmd) {
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewNote().
					Title("RevKeen authentication").
					Description("Paste a secret API key from the dashboard (rk_live_* / rk_sandbox_*)."),
				huh.NewInput().
					Title("API key").
					EchoMode(huh.EchoModePassword).
					Value(&apiKey).
					Validate(func(s string) error {
						s = strings.TrimSpace(s)
						if s == "" {
							return fmt.Errorf("API key cannot be empty")
						}
						if !strings.HasPrefix(s, "rk_") {
							return fmt.Errorf("expected a key starting with rk_")
						}
						return nil
					}),
			),
		).WithTheme(huh.ThemeCharm())

		if err := form.Run(); err != nil {
			return err
		}
	} else {
		// REV-6759: read from the command's input stream, not os.Stdin. The
		// previous fmt.Scanln bypassed cmd.InOrStdin(), so a piped key was read
		// only by accident of the two happening to be the same file descriptor —
		// and the branch could not be driven at all under test. `--with-token`
		// already reads through cmd.InOrStdin(); this matches it.
		_, _ = fmt.Fprint(cmd.OutOrStdout(), "Enter your RevKeen API key: ")
		scanner := bufio.NewScanner(cmd.InOrStdin())
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("failed to read API key: %w", err)
			}
			return fmt.Errorf("no API key provided on stdin")
		}
		apiKey = scanner.Text()
	}

	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return fmt.Errorf("API key cannot be empty")
	}
	// REV-6759: the interactive form already refuses a non-`rk_` value. The
	// non-interactive branch must refuse it too — otherwise a pasted password or
	// bearer token is stored as an API key and every later call fails at the
	// server with an opaque 401 instead of here, with a clear one.
	if !strings.HasPrefix(apiKey, "rk_") {
		return fmt.Errorf("expected a secret API key starting with rk_ (rk_live_* / rk_sandbox_*)")
	}

	cfg.Auth.Mode = "api_key"
	cfg.Auth.APIKey = apiKey
	cfg.Auth.OAuth = config.OAuthConfig{}
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "API key saved (%s). You are now authenticated.\n", config.MaskAPIKey(apiKey))
	return nil
}

func loginWithToken(cmd *cobra.Command, cfg *config.Config) error {
	token := strings.TrimSpace(os.Getenv("REVKEEN_ACCESS_TOKEN"))
	if token == "" {
		token = strings.TrimSpace(os.Getenv("REVKEEN_API_KEY"))
	}
	if token == "" {
		stat, _ := os.Stdin.Stat()
		if stat != nil && (stat.Mode()&os.ModeCharDevice) == 0 {
			data, err := io.ReadAll(bufio.NewReader(cmd.InOrStdin()))
			if err != nil {
				return fmt.Errorf("failed to read token from stdin: %w", err)
			}
			token = strings.TrimSpace(string(data))
		}
	}
	if token == "" {
		return fmt.Errorf("no token provided — set REVKEEN_ACCESS_TOKEN or pipe a token on stdin")
	}

	if strings.HasPrefix(token, "rk_") && !strings.HasPrefix(token, "rkoa_") {
		cfg.Auth.Mode = "api_key"
		cfg.Auth.APIKey = token
		cfg.Auth.OAuth = config.OAuthConfig{}
		if err := config.Save(cfg); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "API key saved (%s). You are now authenticated.\n", config.MaskAPIKey(token))
		return nil
	}

	cfg.Auth.Mode = "oauth"
	cfg.Auth.APIKey = ""
	cfg.Auth.OAuth = config.OAuthConfig{
		AccessToken: token,
		ClientID:    pkceauth.ClientID,
		ExpiresAt:   time.Now().Add(time.Hour).Format(time.RFC3339),
	}
	if err := config.Save(cfg); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Access token saved. You are now authenticated.")
	return nil
}

func loginWithPKCE(cmd *cobra.Command, cfg *config.Config, noBrowser bool) error {
	client := &pkceauth.Client{
		BaseURL:  cfg.Settings.BaseURL,
		ClientID: pkceauth.ClientID,
	}

	out := cmd.OutOrStdout()
	ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
	defer cancel()

	_, _ = fmt.Fprintln(out, "Starting OAuth PKCE login (local callback on 127.0.0.1)...")

	res, err := client.Login(ctx, func(authURL string) {
		_, _ = fmt.Fprintf(out, "Open this URL in your browser:\n  %s\n\n", authURL)
		if !noBrowser {
			openBrowser(authURL)
		}
	})
	if err != nil {
		return fmt.Errorf("OAuth PKCE login failed: %w — use `revkeen login --device` or `revkeen login --api-key`", err)
	}
	if res == nil || res.Token == nil || res.Token.AccessToken == "" {
		return fmt.Errorf("OAuth PKCE login failed: empty access token — credentials were not saved")
	}

	expiresAt := time.Now().Add(time.Duration(res.Token.ExpiresIn) * time.Second).Format(time.RFC3339)
	if res.Token.ExpiresIn <= 0 {
		expiresAt = time.Now().Add(time.Hour).Format(time.RFC3339)
	}

	cfg.Auth.Mode = "oauth"
	cfg.Auth.APIKey = ""
	cfg.Auth.OAuth = config.OAuthConfig{
		AccessToken:  res.Token.AccessToken,
		RefreshToken: res.Token.RefreshToken,
		ExpiresAt:    expiresAt,
		ClientID:     pkceauth.ClientID,
	}
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	_, _ = fmt.Fprintln(out, "Authentication successful. OAuth access token saved.")
	return nil
}

func loginWithDevice(cmd *cobra.Command, cfg *config.Config, noBrowser bool) error {
	client := &deviceauth.Client{
		BaseURL:  cfg.Settings.BaseURL,
		ClientID: deviceauth.ClientID,
	}

	code, err := client.RequestCode()
	if err != nil {
		return fmt.Errorf("OAuth device login failed to start: %w — use `revkeen login --api-key` or set REVKEEN_API_KEY", err)
	}

	verifyURL := code.VerificationURIComplete
	if verifyURL == "" {
		verifyURL = code.VerificationURI
		if code.UserCode != "" && !strings.Contains(verifyURL, "user_code=") {
			sep := "?"
			if strings.Contains(verifyURL, "?") {
				sep = "&"
			}
			verifyURL = verifyURL + sep + "user_code=" + code.UserCode
		}
	}

	out := cmd.OutOrStdout()
	_, _ = fmt.Fprintf(out, "Open this URL in your browser:\n  %s\n\n", verifyURL)
	_, _ = fmt.Fprintf(out, "Enter code: %s\n\n", code.UserCode)

	if !noBrowser {
		openBrowser(verifyURL)
	}

	_, _ = fmt.Fprintln(out, "Waiting for authentication...")
	tok, err := client.PollToken(code.DeviceCode, code.Interval, code.ExpiresIn)
	if err != nil {
		return fmt.Errorf("OAuth device login failed: %w", err)
	}
	if tok.AccessToken == "" {
		return fmt.Errorf("OAuth device login failed: empty access token — credentials were not saved")
	}

	expiresAt := time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second).Format(time.RFC3339)
	if tok.ExpiresIn <= 0 {
		expiresAt = time.Now().Add(time.Hour).Format(time.RFC3339)
	}

	cfg.Auth.Mode = "oauth"
	cfg.Auth.APIKey = ""
	cfg.Auth.OAuth = config.OAuthConfig{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		ExpiresAt:    expiresAt,
		ClientID:     deviceauth.ClientID,
	}
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	_, _ = fmt.Fprintln(out, "Authentication successful. Session token saved.")
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
			if err := config.ClearAuth(cfg); err != nil {
				return fmt.Errorf("failed to clear credentials: %w", err)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Logged out. Credentials cleared.")
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
			base := config.OriginBaseURL(cfg)

			switch cfg.Auth.Mode {
			case "api_key":
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Auth mode:  API Key\n")
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "API key:    %s\n", config.MaskAPIKey(cfg.Auth.APIKey))
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Base URL:   %s\n", base)
				return nil
			case "oauth":
				if cfg.Auth.OAuth.AccessToken == "" {
					return fmt.Errorf("OAuth mode is set but no access token is stored — run `revkeen login`")
				}
				label := "OAuth (device session)"
				if strings.HasPrefix(cfg.Auth.OAuth.AccessToken, "rkoa_") {
					label = "OAuth (PKCE access token)"
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Auth mode:  %s\n", label)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Client ID:  %s\n", cfg.Auth.OAuth.ClientID)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Expires at: %s\n", cfg.Auth.OAuth.ExpiresAt)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Base URL:   %s\n", base)
				return nil
			default:
				if key := config.ResolveAPIKey(cfg); key != "" {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Auth mode:  Environment (REVKEEN_API_KEY)\n")
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "API key:    %s\n", config.MaskAPIKey(key))
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Base URL:   %s\n", base)
					return nil
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Not authenticated. Run 'revkeen login' to get started.")
				return fmt.Errorf("not authenticated")
			}
		},
	}
}
