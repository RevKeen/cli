## revkeen usage balance-get

Get current usage balance

```
revkeen usage balance-get [flags]
```

### Options

```
      -- string                       
      --customer_id string            Filter to a specific customer by RevKeen ID
      --external_customer_id string   Filter to a customer by your external identifier
  -h, --help                          help for balance-get
      --meter_id string               Filter to a single meter
      --subscription_id string        Filter to a specific subscription
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

* [revkeen usage](revkeen_usage.md)	 - Operations on usage

