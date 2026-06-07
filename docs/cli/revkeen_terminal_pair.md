## revkeen terminal pair

Pair a POS terminal device

### Synopsis

Initiate the device pairing flow for a PAX A920 Pro terminal.

```
revkeen terminal pair [flags]
```

### Examples

```
  revkeen terminal pair --serial PAX123456
```

### Options

```
  -h, --help            help for pair
      --serial string   Terminal serial number (required)
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

