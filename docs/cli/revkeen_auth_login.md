## revkeen auth login

Authenticate with RevKeen

### Synopsis

Authenticate with RevKeen using OAuth (default) or an API key.

By default, opens a browser for the OAuth device flow. Use --api-key
to enter an API key directly, or --no-browser to get a URL for manual
authentication in headless environments.

```
revkeen auth login [flags]
```

### Examples

```
  # Interactive OAuth login (opens browser)
  revkeen auth login

  # API key login (prompts for key)
  revkeen auth login --api-key

  # Headless OAuth login (prints URL)
  revkeen auth login --no-browser
```

### Options

```
      --api-key      Log in with an API key instead of OAuth
  -h, --help         help for login
      --no-browser   Print the login URL instead of opening a browser
```

### Options inherited from parent commands

```
      --agent           Machine-readable mode: compact JSON, no color, errors as JSON to stderr
      --json            Shorthand for --output json (pretty-printed)
      --no-color        Disable color output
  -o, --output string   Output format: table, json, yaml, csv (default "table")
      --table           Shorthand for --output table
```

### SEE ALSO

* [revkeen auth](revkeen_auth.md)	 - Authenticate with RevKeen

