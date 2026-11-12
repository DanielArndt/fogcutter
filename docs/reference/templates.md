# Templates reference

Action fields that accept templates: `title`, `body`, `inputs` values, `command`, and `env` values. Templates use Go's [`text/template`](https://pkg.go.dev/text/template) syntax.

## Variables

| Variable | Type | Description |
| :--- | :--- | :--- |
| `{{.ID}}` | string | Blocker ID. |
| `{{.Description}}` | string | Blocker description; may be empty. |
| `{{.Location}}` | string | `file:line` or `owner/repo#n`; empty for declared blockers unless `location:` is set. |
| `{{.Resolution}}` | string | Human-readable result. For issues and releases it is a Markdown link. |
| `{{.SourceSummary}}` | string | Short source identifier, e.g. `ory/kratos#364`, `snap:go/1.26/stable (>=1.26.9)`, `github-release:openfga/openfga (>=v1.8.0)`. |
| `{{.Source}}` | object | The full source config. Fields use Go names: `.Source.Type`, `.Repo`, `.Issue`, `.Version`, `.Snap`, `.Channel`, `.Package`, `.Manifest`, `.Ref`, `.URL`, `.Command`, etc. |

## Source summary formats

| Type | `SourceSummary` |
| :--- | :--- |
| `github-issue` | `owner/repo#123` |
| `github-release` | `github-release:owner/repo (>=v1.8.0)` |
| `snapcraft` | `snap:name/channel (target)` |
| `http` | `http:<url>` |
| `command` | `cmd:<command>` |
| `dependency` | `dep:owner/repo@ref:manifest:package>=version` (`<file>` shown for `ref_file`) |

## Errors

A template that fails to parse or execute makes the action fail with `rendering … template`. Accessing an unknown field on `.Source` is also an execution error. Not templated: `repo`, `labels`, `assignees`, `workflow`, `ref`, `event_type`.

## Defaults

When `title` or `body` is omitted fogcutter uses the defaults listed in [Actions](actions.md).
