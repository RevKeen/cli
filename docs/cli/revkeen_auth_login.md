## revkeen auth login

Authenticate with RevKeen

### Synopsis

Authenticate with RevKeen.

Default (interactive): OAuth authorization-code + PKCE with a local
127.0.0.1 callback — opens the browser for sign-in and consent.

Alternatives:
  --device       OAuth device grant (SSH / Codespaces / headless)
  --api-key      Secret API key (rk_live_* / rk_sandbox_*)
  --with-token   Read a bearer token or API key from stdin / REVKEEN_ACCESS_TOKEN

API keys and environment variables remain fully supported.

```
revkeen auth login [flags]
```

### Examples

```
  # PKCE browser login (default)
  revkeen login
  revkeen auth login

  # Device flow (SSH / no browser automation)
  revkeen login --device
  revkeen login --device --no-browser

  # API key
  revkeen login --api-key

  # Token from env or stdin
  REVKEEN_ACCESS_TOKEN=rkoa_... revkeen login --with-token
  echo 'rk_live_...' | revkeen login --with-token
```

### Options

```
      --api-key      Log in with an API key
      --device       Use OAuth device authorization grant (RFC 8628)
  -h, --help         help for login
      --no-browser   Print the authorize/verification URL instead of opening a browser
      --with-token   Read bearer token or API key from REVKEEN_ACCESS_TOKEN or stdin
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

