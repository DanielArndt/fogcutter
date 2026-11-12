# How to annotate source code with blockers

Mark a workaround where it lives so fogcutter can tell you when it can go.

## Write an annotation

Put one of these forms in any comment, in any text file:

```go
// BLOCKER(ory/kratos#364): remove the DSN workaround
# TODO(blocked-by:snap:lxd:5.21/stable>=5.21.2): upgrade LXD
// FIXME(blocked-on:github-release:openfga/openfga>=v1.8.0): adopt batch check
```

The text in the parentheses is a [spec](../reference/annotations.md#spec-grammar). Everything after the closing `)` and optional colon becomes the blocker's description. fogcutter does not care about the comment syntax, so it works in Go, Python, YAML, Dockerfiles, Markdown, and so on.

## Scan

```bash
fogcutter list -scan .                 # see what was found
fogcutter check -scan . -dry-run -repo owner/name
```

Scan several paths with a comma-separated list: `-scan cmd,internal,Dockerfile`.

To scan on every run without a flag, put the paths in config:

```yaml
scan:
  paths: ["."]
```

## Keep unwanted matches out

fogcutter always skips `.git`, `node_modules`, `vendor`, `dist`, `build`, `.venv`, `venv` and `__pycache__` below the scan root, and files larger than 2 MB. Add your own patterns:

```yaml
scan:
  paths: ["."]
  ignore:
    - "docs"          # directory prefix
    - "testdata"
    - "*_test.go"     # file name glob
    - "README.md"
```

Documentation and tests that show example annotations will otherwise be picked up as real blockers. See [Annotations reference: ignore patterns](../reference/annotations.md#ignore-patterns).

## Choose what happens on resolution

Scanned blockers use `default_actions` from the config. Without any, they get a `create_issue` action titled `Blocker resolved: {{.ID}}`. Set your own defaults:

```yaml
default_actions:
  - type: comment_issue
    issue: 15
    body: "{{.Location}} can be unblocked: {{.Resolution}}"
```

## Notes

- Unrecognised or malformed specs are silently skipped. Run `fogcutter list -scan .` to confirm yours was found.
- Annotations cannot express `http` or `command` sources. Declare those [in a config file](declare-blockers-in-config.md).
- The blocker ID includes the line number, so moving the comment changes the ID. See [Persist state between runs](persist-state.md) for the consequences.
