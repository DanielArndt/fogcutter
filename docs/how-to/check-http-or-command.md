# How to check an HTTP endpoint or a command

When nothing built in matches, use the `http` or `command` source. Both are declared in config; annotations cannot express them.

## An HTTP endpoint returns a status

```yaml
blockers:
  - id: status-page-green
    description: "Vendor API v2 is live"
    source:
      type: http
      url: https://api.example.com/v2/health
      status_code: 200          # default 200
```

If the status matches and no `equals`, `regex` or `version` is given, the blocker is resolved.

## A JSON field has a value

```yaml
source:
  type: http
  url: https://api.example.com/status
  json_path: release.state       # dot path; array indexes are numbers: items.0.name
  equals: "ga"
```

## A JSON field satisfies a version

```yaml
source:
  type: http
  url: https://api.example.com/info
  json_path: version
  version: ">=3.2.0"
```

## Match the body with a regex

```yaml
source:
  type: http
  url: https://example.com/changelog.txt
  regex: 'Release 4\.\d+'
```

Without `json_path` the whole (trimmed) body is the candidate value; bodies are read up to 2 MiB.

## Authenticated or non-GET requests

```yaml
source:
  type: http
  url: https://api.example.com/private
  method: HEAD
  headers:
    Authorization: "Bearer REPLACE_ME"
```

Header values are literal strings: environment variables and templates are *not* expanded in config, and you should not commit secrets. If you need a token, use a `command` source whose script reads the environment.

## A command succeeds

```yaml
source:
  type: command
  command: "test -f /var/lib/vendor/fixed.flag"
```

Exit code `0` is resolved, anything else is blocked (not an error).

## A command prints a value

```yaml
source:
  type: command
  command: "curl -fsS https://example.com/version"
  version: ">=2.0.0"             # or equals: "..." or regex: "..."
```

The trimmed stdout is tested. Exactly one of `equals`, `regex`, `version` is applied, in that order of precedence.

Commands run with `sh -c`, so they need a POSIX shell. Runs have no timeout of their own beyond the process context; keep them quick.

See [Sources reference](../reference/sources.md#http) for every field.
