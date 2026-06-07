## revkeen webhooks listen

Forward webhook events to a local URL

### Synopsis

Listen for RevKeen webhook events and forward them to a local server.
This is useful during development to test webhook handlers without
deploying to a public URL.

```
revkeen webhooks listen [flags]
```

### Examples

```
  # Forward all events to localhost
  revkeen webhooks listen --forward-to http://localhost:3000/webhooks

  # Forward specific events only
  revkeen webhooks listen \
    --forward-to http://localhost:3000/webhooks \
    --events subscription.created,payment.completed
```

### Options

```
      --events strings      Comma-separated list of event types to listen for
      --forward-to string   Local URL to forward events to (required)
  -h, --help                help for listen
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

