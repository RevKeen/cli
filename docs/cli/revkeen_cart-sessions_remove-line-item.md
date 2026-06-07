## revkeen cart-sessions remove-line-item

Remove a line item from a cart session

```
revkeen cart-sessions remove-line-item [flags]
```

### Options

```
      -- string     
  -h, --help        help for remove-line-item
      --id string   
      --lineId id   Cart line item id (the id field returned on `cart_session.line_items[]`).
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

* [revkeen cart-sessions](revkeen_cart-sessions.md)	 - Operations on cart-sessions

