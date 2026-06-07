## revkeen terminal pay

Initiate a payment on a terminal

```
revkeen terminal pay [amount] [flags]
```

### Examples

```
  revkeen terminal pay 50.00 --terminal PAX123456
```

### Options

```
  -h, --help              help for pay
      --terminal string   Terminal ID or serial number (required)
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

* [revkeen terminal](revkeen_terminal.md)	 - Manage POS terminal devices

