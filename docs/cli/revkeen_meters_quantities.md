## revkeen meters quantities

Get meter quantities

```
revkeen meters quantities [flags]
```

### Options

```
      -- string                       
      --customer_id string            Filter by customer ID
      --end_timestamp string          End timestamp (ISO 8601, UTC)
      --external_customer_id string   Filter by the merchant's external customer ID
  -h, --help                          help for quantities
      --id string                     Meter ID
      --interval string               Bucket width. UTC date_trunc semantics; week is ISO (Monday).
      --start_timestamp string        Start timestamp (ISO 8601, UTC)
      --subscription_id string        Filter by subscription ID
      --timezone string               Only UTC is supported. Omit or pass UTC.
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

* [revkeen meters](revkeen_meters.md)	 - Operations on meters

