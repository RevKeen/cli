## revkeen config set

Set a configuration value

### Synopsis

Set a CLI configuration value. Available keys:

  api-key        Your RevKeen API key
  base-url       API base URL (default: https://api.revkeen.com)
  output         Default output format: table, json, yaml, csv
  environment    Target environment: production, staging

```
revkeen config set [key] [value] [flags]
```

### Examples

```
  revkeen config set api-key rk_live_xxx
  revkeen config set environment staging
  revkeen config set output json
```

### Options

```
  -h, --help   help for set
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

* [revkeen config](revkeen_config.md)	 - Manage CLI configuration

