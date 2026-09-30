## revkeen storefront products-list

List storefront products

```
revkeen storefront products-list [flags]
```

### Options

```
      -- string             
      --collection string   Only products in this product collection (id or URL handle).
  -h, --help                help for products-list
      --limit int           Maximum products to return (default 50, max 100).
      --offset int          Products to skip, for pagination.
      --search string       Case-insensitive match on product name or description.
      --sort string         featured (collection order, else newest), newest, name, price_asc or price_desc.
      --tag string          Only products carrying this tag.
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

* [revkeen storefront](revkeen_storefront.md)	 - Operations on storefront

