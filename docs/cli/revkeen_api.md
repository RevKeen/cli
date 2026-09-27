## revkeen api

Make raw API requests

### Synopsis

Make authenticated HTTP requests to any RevKeen API endpoint.
This is an escape hatch for endpoints not exposed as named CLI commands.

```
revkeen api [method] [path] [flags]
```

### Examples

```
  # GET request
  revkeen api GET /v2/customers

  # POST request with JSON body
  revkeen api POST /v2/invoices -d '{"customerId":"cus_xxx","amount":5000}'

  # DELETE request
  revkeen api DELETE /v2/webhook-endpoints/we_xxx

  # Agent mode — raw JSON passthrough
  revkeen api GET /v2/customers --agent
```

### Options

```
  -d, --data string   Request body (JSON string)
  -h, --help          help for api
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

* [revkeen](revkeen.md)	 - RevKeen CLI — manage payments, subscriptions & billing

