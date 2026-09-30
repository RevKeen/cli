# RevKeen CLI

[![npm version](https://img.shields.io/npm/v/@revkeen/cli-binary.svg)](https://www.npmjs.com/package/@revkeen/cli-binary)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Manage invoices, customers, subscriptions, and billing from your terminal.

Resource commands (`customers`, `invoices`, …) are generated from the OpenAPI
contract via Climate/Cobra. Operator commands (`auth`, `cart`, `doctor`,
`init`, `dashboard`, …) are hand-written. Cart and doctor call the generated
[`sdk-go`](https://github.com/revkeen/sdk-go) client.

## Installation

### npm (recommended)

```bash
npm install -g @revkeen/cli-binary
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
# Authenticate (OAuth PKCE by default; --device for SSH; --api-key / REVKEEN_API_KEY for CI)
revkeen login
revkeen login --device --no-browser
revkeen login --api-key

# Interactive terminal dashboard
revkeen dashboard

# List recent invoices
revkeen invoices list --limit 10

# JSON output for scripting / agents
revkeen invoices list --json
revkeen customers list --agent
```

## Authentication

```bash
# PKCE browser login (default) or device / API key
revkeen login
revkeen login --device
revkeen login --api-key

# Non-interactive / CI
export REVKEEN_API_KEY=rk_live_your_api_key
revkeen invoices list

# Per-invocation override
revkeen --api-key rk_live_... customers list

revkeen auth status
```

Secrets prefer the OS keyring, with `~/.revkeen/credentials.toml` (0600) as fallback.
Non-secret settings live in `~/.revkeen/config.toml`.

```bash
revkeen config set environment staging   # https://staging-api.revkeen.com
revkeen config set api-key rk_live_...
```

## Operator commands

| Command | Description |
|---------|-------------|
| `revkeen dashboard` | Interactive Bubble Tea dashboard |
| `revkeen login` / `revkeen auth login` | OAuth PKCE (default), `--device`, or `--api-key` |
| `revkeen cart …` | Cart keys, origins, webhooks, status |
| `revkeen doctor` | Cart health check |
| `revkeen api GET /v2/…` | Raw HTTP escape hatch |

## Output Formats

| Flag | Behaviour |
|------|-----------|
| _(TTY default)_ | Styled Lip Gloss table |
| `--json` | Pretty JSON |
| `--agent` | Compact JSON (no TUI chrome) |
| `--output yaml\|csv` | YAML / CSV |

## Platforms

| Platform | Architecture | Download |
|----------|-------------|----------|
| macOS | Apple Silicon (arm64) | `revkeen_darwin_arm64.tar.gz` |
| macOS | Intel (amd64) | `revkeen_darwin_amd64.tar.gz` |
| Linux | x86_64 | `revkeen_linux_amd64.tar.gz` |
| Linux | arm64 | `revkeen_linux_arm64.tar.gz` |
| Windows | x86_64 | `revkeen_windows_amd64.zip` |

## Links

- [CLI docs](https://docs.revkeen.com/docs/cli)
- [API Reference](https://docs.revkeen.com/api-reference/openapi)
- [Go SDK](https://github.com/revkeen/sdk-go)
