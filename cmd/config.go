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

			switch key {
			case "api-key":
				cfg.Auth.Mode = "api_key"
				cfg.Auth.APIKey = value
			case "base-url":
				cfg.Settings.BaseURL = value
			case "output":
				cfg.Settings.DefaultOutput = value
			case "environment":
				switch value {
				case "production":
					cfg.Settings.BaseURL = "https://api.revkeen.com"
				case "staging":
					cfg.Settings.BaseURL = "https://staging-api.revkeen.com"
				default:
					return fmt.Errorf("unknown environment: %s (use 'production' or 'staging')", value)
				}
			default:
				return fmt.Errorf("unknown config key: %s", key)
			}

			if err := config.Save(cfg); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			fmt.Printf("Set %s = %s\n", key, value)
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

			fmt.Printf("Auth mode:      %s\n", cfg.Auth.Mode)
			if cfg.Auth.APIKey != "" {
				masked := cfg.Auth.APIKey
				if len(masked) > 12 {
					masked = masked[:8] + "..." + masked[len(masked)-4:]
				}
				fmt.Printf("API key:        %s\n", masked)
			}
			fmt.Printf("Base URL:       %s\n", cfg.Settings.BaseURL)
			fmt.Printf("Default output: %s\n", cfg.Settings.DefaultOutput)
			if cfg.Settings.MerchantID != "" {
				fmt.Printf("Merchant ID:    %s\n", cfg.Settings.MerchantID)
			}
			return nil
		},
	}
}
