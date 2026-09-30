## revkeen trigger

Send a test webhook event to active CLI listen sessions

### Synopsis

Publish a fixture event to merchants with an active revkeen listen session.

Does not create billing side effects — fixtures go to CLI listeners only.
Examples: payment.succeeded, invoice.paid, customer.created

```
revkeen trigger [event_type] [flags]
```

### Examples

```
  revkeen trigger payment.succeeded
  revkeen trigger invoice.paid
```

### Options

```
  -h, --help   help for trigger
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

* [revkeen](revkeen.md)	 - RevKeen CLI — manage payments, subscriptions & billing

