## revkeen refunds list

List refunds

```
revkeen refunds list [flags]
```

### Options

```
      -- string                 
      --created_after string    ISO 8601 date - only refunds created after this date
      --created_before string   ISO 8601 date - only refunds created before this date
      --gateway string          Filter by payment gateway (nmi, stripe, etc.)
  -h, --help                    help for list
      --limit float             Number of results to return (1-100)
      --offset float            Number of results to skip
      --payment_id string       Filter by original payment ID
      --reason string           Filter by refund reason
      --status string           Filter by refund status
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

* [revkeen refunds](revkeen_refunds.md)	 - Operations on refunds

