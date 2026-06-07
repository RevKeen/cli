## revkeen

RevKeen CLI — manage payments, subscriptions & billing

### Synopsis

The official RevKeen command-line interface.
Manage customers, invoices, products, subscriptions, and more from your terminal.

Install:
  npm install -g @revkeen/cli-binary
  brew install revkeen/tap/revkeen
  curl -fsSL https://cli.revkeen.com/install.sh | sh

Authentication:
  revkeen auth login        Interactive OAuth login
  REVKEEN_API_KEY=rk_...    Environment variable

Quick start:
  revkeen customers list
  revkeen invoices retrieve inv_xxxxxxxx --json
  revkeen api GET /v2/customers

### Options

```
      --agent            Machine-readable mode: compact JSON, no color, errors as JSON to stderr
      --api-key string   Override API key (or set REVKEEN_API_KEY)
  -h, --help             help for revkeen
      --json             Shorthand for --output json (pretty-printed)
      --no-color         Disable color output
  -o, --output string    Output format: table, json, yaml, csv (default "table")
      --table            Shorthand for --output table
```

### SEE ALSO

* [revkeen analytics](revkeen_analytics.md)	 - Operations on analytics
* [revkeen api](revkeen_api.md)	 - Make raw API requests
* [revkeen auth](revkeen_auth.md)	 - Authenticate with RevKeen
* [revkeen cart-sessions](revkeen_cart-sessions.md)	 - Operations on cart-sessions
* [revkeen checkout-sessions](revkeen_checkout-sessions.md)	 - Operations on checkout-sessions
* [revkeen config](revkeen_config.md)	 - Manage CLI configuration
* [revkeen credit-notes](revkeen_credit-notes.md)	 - Operations on credit-notes
* [revkeen customer-meters](revkeen_customer-meters.md)	 - Operations on customer-meters
* [revkeen customer-portal](revkeen_customer-portal.md)	 - Operations on customer-portal
* [revkeen customers](revkeen_customers.md)	 - Operations on customers
* [revkeen entitlements](revkeen_entitlements.md)	 - Operations on entitlements
* [revkeen events](revkeen_events.md)	 - Operations on events
* [revkeen invoices](revkeen_invoices.md)	 - Operations on invoices
* [revkeen payment-intents](revkeen_payment-intents.md)	 - Operations on payment-intents
* [revkeen payment-links](revkeen_payment-links.md)	 - Operations on payment-links
* [revkeen prices](revkeen_prices.md)	 - Operations on prices
* [revkeen products](revkeen_products.md)	 - Operations on products
* [revkeen refunds](revkeen_refunds.md)	 - Operations on refunds
* [revkeen subscriptions](revkeen_subscriptions.md)	 - Operations on subscriptions
* [revkeen terminal](revkeen_terminal.md)	 - Manage POS terminal devices
* [revkeen transactions](revkeen_transactions.md)	 - Operations on transactions
* [revkeen webhook-deliveries](revkeen_webhook-deliveries.md)	 - Operations on webhook-deliveries
* [revkeen webhook-endpoints](revkeen_webhook-endpoints.md)	 - Operations on webhook-endpoints
* [revkeen webhooks](revkeen_webhooks.md)	 - Work with RevKeen webhooks

