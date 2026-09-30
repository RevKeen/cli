## revkeen webhooks listen

Forward webhook events to a local URL (alias of revkeen listen)

### Synopsis

Connect to RevKeen and forward webhook events to your local server.

Requires authentication (revkeen login or REVKEEN_API_KEY). Prints a
session signing secret you can use to verify X-Revkeen-Signature locally.

Also available as: revkeen webhooks listen

```
revkeen webhooks listen [flags]
```

### Examples

```
  revkeen listen --forward-to http://localhost:3000/webhooks
  revkeen listen --forward-to http://127.0.0.1:4242/hooks --events invoice.paid,payment.succeeded
  revkeen trigger payment.succeeded
```

### Options

```
      --events strings      Comma-separated event types to receive (default: all)
      --forward-to string   Local URL to forward events to (required)
  -h, --help                help for listen
      --skip-verify         Reserved for future TLS options (no-op)
```

### Options inherited from parent commands

```
      --agent            Machine-readable mode: compact JSON, no color, errors as JSON to stderr
      --api-key string   Override API key (or set REVKEEN_API_KEY)
      --json             Shorthand for --output json (pretty-printed)
      --no-color         Disable color output
  -o, --output string    Output format: table, json, yaml, csv (default "table")
      --table            Shorthand for --output table
```

### SEE ALSO

* [revkeen webhooks](revkeen_webhooks.md)	 - Work with RevKeen webhooks

