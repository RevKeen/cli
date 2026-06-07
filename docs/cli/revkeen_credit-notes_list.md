## revkeen credit-notes list

List credit notes

```
revkeen credit-notes list [flags]
```

### Options

```
      -- string                 
      --created_after string    ISO 8601 date - only credit notes created after this date
      --created_before string   ISO 8601 date - only credit notes created before this date
      --credit_method string    Filter by credit method
      --customer_id string      Filter by customer ID
  -h, --help                    help for list
      --invoice_id string       Filter by invoice ID
      --limit float             Number of results to return (1-100)
      --offset float            Number of results to skip
      --status string           Filter by credit note status
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

* [revkeen credit-notes](revkeen_credit-notes.md)	 - Operations on credit-notes

