## revkeen usage-events list

Query usage events

```
revkeen usage-events list [flags]
```

### Options

```
      -- string                       
      --customer_id string            Filter by customer ID
      --end_time string               End of time range (ISO 8601)
      --external_customer_id string   Filter by external customer ID
  -h, --help                          help for list
      --limit string                  Max results (default 100, max 100)
      --meter_id string               Filter by meter ID
      --start_time string             Start of time range (ISO 8601)
      --subscription_id string        Filter by subscription ID
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

* [revkeen usage-events](revkeen_usage-events.md)	 - Operations on usage-events

