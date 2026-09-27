## revkeen customer-meters list

List a customer's meter usage

```
revkeen customer-meters list [flags]
```

### Options

```
      -- string              
      --customer_id string   Customer UUID. Required — customer-meters are not queryable merchant-wide via this endpoint. Use /v2/meters for merchant-level meter definitions.
  -h, --help                 help for list
      --meter_id string      Optional single-meter filter. When provided, the response contains at most one entry.
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

* [revkeen customer-meters](revkeen_customer-meters.md)	 - Operations on customer-meters

