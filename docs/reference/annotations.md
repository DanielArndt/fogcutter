# Inline annotations and specs

## Comment forms

fogcutter finds annotations with this case-insensitive pattern, anywhere on a line:

| Form | Example |
| :--- | :--- |
| `BLOCKER(<spec>)` | `// BLOCKER(ory/kratos#364): remove DSN workaround` |
| `TODO(blocked-by:<spec>)` | `# TODO(blocked-by:snap:lxd:5.21/stable>=5.21.2): upgrade LXD` |
| `TODO(blocked-on:<spec>)` | `-- TODO(blocked-on:github-release:org/repo>=v2.0.0)` |
| `FIXME(blocked-by:<spec>)`, `FIXME(blocked-on:<spec>)` | |

`blocked-by` may also be written `blocked_by` or `blocked by`, and likewise for `on`. An optional colon follows the closing parenthesis; the rest of the line is the blocker's description, with a trailing `*/`, `-->` or backtick removed.

The spec ends at the first `)`, so it cannot contain one. Specs that fail to parse are silently skipped.

## Spec grammar

| Spec | Source |
| :--- | :--- |
| `owner/repo#123` | `github-issue` |
| `https://github.com/owner/repo/issues/123` (or `/pull/123`) | `github-issue` |
| `github-issue:owner/repo#123` | `github-issue` |
| `github-release:owner/repo>=v1.8.0` (or `…@>=v1.8.0`) | `github-release` |
| `snap:<name>>=1.2` | `snapcraft`, channel `latest/stable` |
| `snap:<name>:<channel>>=1.2` | `snapcraft` |
| `snap:<name>/<channel>>=1.2` (e.g. `snap:go/1.26/stable>=1.26.9`) | `snapcraft` |
| `snapcraft:…` | alias of `snap:…` |
| `dep:owner/repo[@ref]:<manifest>:<package>>=1.2.3` | `dependency` |
| `dependency:…` | alias of `dep:…` |

`github-release` is also accepted as `github-release:owner/repo@v1.8.0` (the part after `@` is the constraint). The constraint is mandatory for `snap` and `dep`. `http` and `command` sources have no spec form.

The same grammar is used by `-spec` on the command line and `fogcutter add`.

### Implicit ref for `dep:`

For a `dep:` annotation without `@ref`, in a file where the annotation sits inside a mapping (craft files such as `rockcraft.yaml`, `snapcraft.yaml`, `charmcraft.yaml`), fogcutter finds the enclosing part by indentation and uses its `source-tag` or `source-commit`, provided the part's `source:` mentions the same repository (a `.git` suffix is ignored). Otherwise the latest release is used. `@latest` forces the latest release.

## Identifiers and locations

| Where found | `id` | `location` |
| :--- | :--- | :--- |
| File | `code:<path>:<line>:<spec>` | `<path>:<line>` |
| Issue | `issue:<repo>#<n><-<spec>` | `<repo>#<n>` |

Duplicate IDs within a scan are collapsed.

## Issue lines

In issue bodies and comments (see [Scan GitHub issues](../how-to/scan-github-issues.md)), besides the forms above, a line is recognised if it starts (after optional `-` or `*`) with `Blocked by` or `Blocked on`, with optional colon, followed by a spec:

```text
Blocked by ory/kratos#364 until the DSN fix lands
- Blocked on snap:go/1.26/stable>=1.26.9
```

Text after the spec becomes the description; if empty it is `Unblocks #<n> (<title>)`.

## Ignore patterns

`scan.ignore` entries (config only) are tested against each directory and file; a match skips it (and, for a directory, everything below it). An entry matches if any of these holds:

1. it matches the entry's base name as a glob (`*_test.go`, `testdata`);
2. it matches the whole path as a glob;
3. the path starts with the entry (trailing `/` removed), so `docs` also skips `docs/…` and `docsite`.

Built-in skips (not for the scan root itself): `.git`, `node_modules`, `vendor`, `dist`, `build`, `.venv`, `venv`, `__pycache__`. Files over 2 MiB, non-regular files, and unreadable files are skipped. fogcutter does not detect binary content beyond the size limit.

> fogcutter's own repository contains example annotations in its README, docs and tests. Ignore them when scanning it.
