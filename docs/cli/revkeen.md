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
  revkeen login             OAuth PKCE browser login (default)
  revkeen login --device    Device grant (SSH / headless)
  revkeen login --api-key   Interactive API key login
  REVKEEN_API_KEY=rk_...    Environment variable

Quick start:
  revkeen dashboard
  revkeen customers list
  revkeen invoices get inv_xxxxxxxx --json
  revkeen api GET /v2/customers

```
revkeen [flags]
```

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
* [revkeen billing](revkeen_billing.md)	 - Operations on billing
* [revkeen cart](revkeen_cart.md)	 - Set up and inspect RevKeen Cart for headless storefronts
* [revkeen cart-api-keys](revkeen_cart-api-keys.md)	 - Operations on cart-api-keys
* [revkeen cart-sessions](revkeen_cart-sessions.md)	 - Operations on cart-sessions
* [revkeen checkout-sessions](revkeen_checkout-sessions.md)	 - Operations on checkout-sessions
* [revkeen config](revkeen_config.md)	 - Manage CLI configuration
* [revkeen credit-notes](revkeen_credit-notes.md)	 - Operations on credit-notes
* [revkeen customer-meters](revkeen_customer-meters.md)	 - Operations on customer-meters
* [revkeen customer-portal](revkeen_customer-portal.md)	 - Operations on customer-portal
* [revkeen customers](revkeen_customers.md)	 - Operations on customers
* [revkeen dashboard](revkeen_dashboard.md)	 - Open the interactive terminal dashboard
* [revkeen dd](revkeen_dd.md)	 - Operations on dd
* [revkeen doctor](revkeen_doctor.md)	 - Health-check a RevKeen Cart integration
* [revkeen entitlements](revkeen_entitlements.md)	 - Operations on entitlements
* [revkeen events](revkeen_events.md)	 - Operations on events
* [revkeen integrations](revkeen_integrations.md)	 - Operations on integrations
* [revkeen invoice-line-items](revkeen_invoice-line-items.md)	 - Operations on invoice-line-items
* [revkeen invoices](revkeen_invoices.md)	 - Operations on invoices
* [revkeen listen](revkeen_listen.md)	 - Forward RevKeen webhook events to a local URL
* [revkeen login](revkeen_login.md)	 - Authenticate with RevKeen
* [revkeen mandates](revkeen_mandates.md)	 - Operations on mandates
* [revkeen meters](revkeen_meters.md)	 - Operations on meters
* [revkeen payment-intents](revkeen_payment-intents.md)	 - Operations on payment-intents
* [revkeen payment-links](revkeen_payment-links.md)	 - Operations on payment-links
* [revkeen prices](revkeen_prices.md)	 - Operations on prices
* [revkeen products](revkeen_products.md)	 - Operations on products
* [revkeen refunds](revkeen_refunds.md)	 - Operations on refunds
* [revkeen storefront](revkeen_storefront.md)	 - Operations on storefront
* [revkeen subscriptions](revkeen_subscriptions.md)	 - Operations on subscriptions
* [revkeen terminal](revkeen_terminal.md)	 - Manage POS terminal devices
* [revkeen transactions](revkeen_transactions.md)	 - Operations on transactions
* [revkeen trigger](revkeen_trigger.md)	 - Send a test webhook event to active CLI listen sessions
* [revkeen usage](revkeen_usage.md)	 - Operations on usage
* [revkeen usage-events](revkeen_usage-events.md)	 - Operations on usage-events
* [revkeen webhook-deliveries](revkeen_webhook-deliveries.md)	 - Operations on webhook-deliveries
* [revkeen webhook-endpoints](revkeen_webhook-endpoints.md)	 - Operations on webhook-endpoints
* [revkeen webhooks](revkeen_webhooks.md)	 - Work with RevKeen webhooks

