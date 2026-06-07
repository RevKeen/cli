# RevKeen CLI

[![npm version](https://img.shields.io/npm/v/@revkeen/cli.svg)](https://www.npmjs.com/package/@revkeen/cli)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Manage invoices, customers, subscriptions, and billing from your terminal. Built in Go with agent mode and MCP support.

## Installation

### npm (recommended)

```bash
npm install -g @revkeen/cli
```

### Homebrew

```bash
brew install revkeen/tap/revkeen
```

### Shell script

```bash
curl -fsSL https://cli.revkeen.com/install.sh | sh
```

### Manual download

Download the latest release for your platform from [GitHub Releases](https://github.com/revkeen/cli/releases).

## Quick Start

```bash
# Authenticate
revkeen auth login

# List recent invoices
revkeen invoices list --limit 10

# Create a customer
revkeen customers create --name "Acme Corp" --email "billing@acme.com"

# Get a specific invoice
revkeen invoices get inv_xxxxxxxx

# JSON output for scripting
revkeen invoices list --json | jq '.data[].id'
```

## Authentication

```bash
# Interactive login (opens browser)
revkeen auth login

# API key (non-interactive / CI)
export REVKEEN_API_KEY=rk_live_your_api_key
revkeen invoices list

# Check current auth status
revkeen auth status
```

## Agent Mode

The CLI includes an AI agent that can reason about your billing data and execute multi-step operations.

```bash
# Interactive agent session
revkeen agent

# Single prompt
revkeen agent "list all overdue invoices and show total outstanding"

# Non-interactive with JSON output
revkeen agent --json "summarize this month's revenue"
```

## Output Formats

```bash
# Human-readable table (default when TTY)
revkeen customers list

# JSON (default when piped, or explicit)
revkeen customers list --json

# Table (explicit)
revkeen customers list --table
```

The CLI auto-detects whether stdout is a TTY. When piping to another command, output defaults to JSON.

## Commands

| Command | Description |
|---------|-------------|
| `revkeen auth login` | Authenticate with RevKeen |
| `revkeen auth status` | Show current auth status |
| `revkeen auth logout` | Remove stored credentials |
| `revkeen customers list` | List customers |
| `revkeen customers create` | Create a customer |
| `revkeen customers get <id>` | Get customer details |
| `revkeen invoices list` | List invoices |
| `revkeen invoices get <id>` | Get invoice details |
| `revkeen subscriptions list` | List subscriptions |
| `revkeen subscriptions get <id>` | Get subscription details |
| `revkeen products list` | List products |
| `revkeen agent` | Start AI agent session |
| `revkeen version` | Show CLI version |

## Configuration

The CLI stores configuration in `~/.config/revkeen/` (Linux/macOS) or `%APPDATA%\revkeen\` (Windows).

```bash
# Override API base URL (e.g. for staging)
export REVKEEN_BASE_URL=https://staging-api.revkeen.com

# Override config directory
export REVKEEN_CONFIG_DIR=/path/to/config
```

## Platforms

| Platform | Architecture | Download |
|----------|-------------|----------|
| macOS | Apple Silicon (arm64) | `revkeen_darwin_arm64.tar.gz` |
| macOS | Intel (amd64) | `revkeen_darwin_amd64.tar.gz` |
| Linux | x86_64 | `revkeen_linux_amd64.tar.gz` |
| Linux | arm64 | `revkeen_linux_arm64.tar.gz` |
| Windows | x86_64 | `revkeen_windows_amd64.zip` |

## Links

- [API Reference](https://docs.revkeen.com/api-reference/openapi)
- [SDK Documentation](https://docs.revkeen.com/docs/developers/sdks)
- [TypeScript SDK](https://github.com/revkeen/sdk-typescript)
- [Go SDK](https://github.com/revkeen/sdk-go)
- [PHP SDK](https://github.com/revkeen/sdk-php)
