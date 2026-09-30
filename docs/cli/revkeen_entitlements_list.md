## revkeen entitlements list

List entitlements

```
revkeen entitlements list [flags]
```

### Options

```
      -- string               
      --benefit_type string   Filter by benefit type
      --category string       Filter by category
      --customer_id string    Customer UUID (required)
  -h, --help                  help for list
      --include_expired       Include expired entitlements
      --limit int             Maximum results (1-100)
      --offset int            Results to skip
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

* [revkeen entitlements](revkeen_entitlements.md)	 - Operations on entitlements

