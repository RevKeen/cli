# RevKeen CLI

The official RevKeen command-line interface — manage payments, subscriptions,
invoices and billing from your terminal.

> **Know What You Keep, Not Just What You Earn.**

A single self-contained binary (Go + [Cobra](https://cobra.dev)), with commands
generated from the RevKeen OpenAPI contract.

## Install

```bash
# Homebrew (macOS / Linux)
brew install revkeen/tap/revkeen

# npm
npm install -g @revkeen/cli-binary

# Shell script (macOS / Linux)
curl -fsSL https://cli.revkeen.com/install.sh | sh
```

## Authentication

```bash
revkeen auth login          # OAuth device flow, or
revkeen config set api-key rk_live_...   # API key (rk_live_* / rk_sandbox_*)
```

Configuration is stored in `~/.revkeen/config.toml`. The `REVKEEN_API_KEY`
environment variable overrides the stored key.

Target staging with `revkeen config set environment staging`.

## Usage

```bash
revkeen customers list
revkeen invoices list
revkeen subscriptions get sub_123
revkeen analytics revenue-get-mrr-summary

# Escape hatch for any endpoint
revkeen api GET /v2/customers
revkeen api POST /v2/invoices -d '{"customerId":"cus_xxx"}'
```

Global flags: `--output table|json|yaml|csv`, `--json`, `--agent` (compact JSON
for tools/LLMs), `--no-color`. Run `revkeen --help` for the full command list.

## License

[MIT](./LICENSE)
