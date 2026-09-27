## revkeen customer-portal subscriptions-list

List the authenticated customer's subscriptions

```
revkeen customer-portal subscriptions-list [flags]
```

### Options

```
      -- string                 
      --ending_before string    Cursor — return results created after the row with this ID.
  -h, --help                    help for subscriptions-list
      --limit int               Maximum number of results to return (1-100, default 20).
      --starting_after string   Cursor — return results created before the row with this ID.
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

* [revkeen customer-portal](revkeen_customer-portal.md)	 - Operations on customer-portal

