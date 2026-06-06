## revkeen customers preferred-rails-get

Get preferred payment rails

```
revkeen customers preferred-rails-get [flags]
```

### Options

```
      -- string             
      --amount int          Optional amount in minor units. Use amount_minor for the explicit alias.
      --amount_minor int    Optional amount in minor units. Overrides invoice total for margin-aware ranking.
  -h, --help                help for preferred-rails-get
      --id string           Customer UUID
      --invoice_id string   Optional invoice context used to apply invoice-level rail restrictions and amount.
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

* [revkeen customers](revkeen_customers.md)	 - Operations on customers

