## revkeen cart keys issue

Ensure the managed publishable + secret Cart keys exist (idempotent)

### Synopsis

Provisions the managed Cart API keys if they do not exist yet.
Key material is printed ONCE, at first issuance only — store it immediately.
Re-running reports existing key status without any secret material.

Requires signing in with `revkeen login`: your role in the merchant decides
whether you can issue keys. Issuing with an API key is being retired.

```
revkeen cart keys issue [flags]
```

### Options

```
  -h, --help   help for issue
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

* [revkeen cart keys](revkeen_cart_keys.md)	 - Manage managed Cart API keys

