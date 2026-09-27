## revkeen invoice-line-items list-usage-events

List usage events for an invoice line

```
revkeen invoice-line-items list-usage-events [flags]
```

### Options

```
      -- string                   
  -h, --help                      help for list-usage-events
      --id string                 Invoice line item ID
      --include_excluded string   When true, also return excluded-but-considered decisions for the line period
      --limit string              Max billed entries to return (default 50, max 100)
      --starting_after string     Cursor: return entries with id greater than this value
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

* [revkeen invoice-line-items](revkeen_invoice-line-items.md)	 - Operations on invoice-line-items

