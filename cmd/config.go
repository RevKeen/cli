package cmd

import (
	"fmt"

	"github.com/revkeen/cli/internal/config"
	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage CLI configuration",
	}

	cmd.AddCommand(newConfigSetCmd())
	cmd.AddCommand(newConfigListCmd())

	return cmd
}

func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set [key] [value]",
		Short: "Set a configuration value",
		Long: `Set a CLI configuration value. Available keys:

  api-key        Your RevKeen API key
  base-url       API base URL (default: https://api.revkeen.com)
  output         Default output format: table, json, yaml, csv
  environment    Target environment: production, staging`,
		Example: `  revkeen config set api-key rk_live_xxx
  revkeen config set environment staging
  revkeen config set output json`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, value := args[0], args[1]
			cfg := config.Load()

			displayValue := value
			switch key {
			case "api-key":
				cfg.Auth.Mode = "api_key"
				cfg.Auth.APIKey = value
				cfg.Auth.OAuth = config.OAuthConfig{}
				displayValue = config.MaskAPIKey(value)
			case "base-url":
				cfg.Settings.BaseURL = value
			case "output":
				cfg.Settings.DefaultOutput = value
			case "environment":
				switch value {
				case "production":
					cfg.Settings.BaseURL = config.DefaultProductionBaseURL
				case "staging":
					cfg.Settings.BaseURL = config.DefaultStagingBaseURL
				default:
					return fmt.Errorf("unknown environment: %s (use 'production' or 'staging')", value)
				}
			default:
				return fmt.Errorf("unknown config key: %s", key)
			}

			if err := config.Save(cfg); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Set %s = %s\n", key, displayValue)
			return nil
		},
	}
}

func newConfigListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Show current configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load()

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Auth mode:      %s\n", cfg.Auth.Mode)
			if cfg.Auth.APIKey != "" {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "API key:        %s\n", config.MaskAPIKey(cfg.Auth.APIKey))
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Base URL:       %s\n", config.OriginBaseURL(cfg))
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "API prefix:     %s\n", config.ResolveBaseURL(cfg))
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Default output: %s\n", cfg.Settings.DefaultOutput)
			if cfg.Settings.MerchantID != "" {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Merchant ID:    %s\n", cfg.Settings.MerchantID)
			}
			return nil
		},
	}
}
