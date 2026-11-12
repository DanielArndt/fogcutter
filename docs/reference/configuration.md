# Configuration reference

fogcutter reads a YAML file, by default `.github/blockers.yaml`. All keys are optional except where noted. An annotated example is [`.github/blockers.example.yaml`](../../.github/blockers.example.yaml).

```yaml
repository: owner/name
state_file: .fogcutter-state.json
scan: {…}
default_actions: [ … ]
blockers: [ … ]
```

## Top-level keys

| Key | Type | Description |
| :--- | :--- | :--- |
| `repository` | string | Default target repository (`owner/name`) for actions and issue scanning. Falls back to `$GITHUB_REPOSITORY`. Overridden by `-repo`. |
| `state_file` | string | Path to the JSON [state file](../how-to/persist-state.md). Overridden by `-state-file`. |
| `scan` | object | Discovery settings. See [`scan`](#scan). |
| `default_actions` | list of [actions](actions.md) | Applied to blockers with no `on_resolved`, including scanned code blockers. |
| `blockers` | list of [blockers](#blockers) | Declared blockers. |

## `scan`

| Key | Type | Description |
| :--- | :--- | :--- |
| `paths` | list of strings | Files or directories to scan for [annotations](annotations.md). Combined with `-scan`. |
| `ignore` | list of strings | Skip patterns. See [ignore patterns](annotations.md#ignore-patterns). |
| `issues` | bool | Also scan open GitHub issues in `repository`. |
| `issue_label` | string | Only scan issues carrying this label. |

## `blockers`

| Key | Type | Description |
| :--- | :--- | :--- |
| `id` | string | Unique, stable identifier; used in markers and state. If omitted it is derived (see below). |
| `description` | string | Free text, available to templates as `{{.Description}}`. |
| `location` | string | Free text such as `internal/db/conn.go:42`; available as `{{.Location}}`. |
| `source` | object, **required** | What to wait for. See [Sources](sources.md). |
| `on_resolved` | list of [actions](actions.md) | What to do when resolved. Empty means `default_actions`. |

### Derived IDs

| Source | ID |
| :--- | :--- |
| `github-issue` | `gh-issue:<repo>#<issue>` |
| `github-release` | `gh-release:<repo>:<version>` |
| `snapcraft` | `snap:<snap>:<channel>:<version><revision>` (channel defaults to `latest/stable`) |
| `dependency` | `dep:<repo>@<ref or ref_file or latest>:<manifest>:<package>:<version>` |
| others | `blocker-<n>`, where n is the 1-based position in the file |

Give blockers an explicit `id` if you rely on state or markers across edits; positional IDs change when you reorder.

### Source type aliases

`snap` is accepted for `snapcraft`, and `dep` for `dependency`. For snapcraft, `snap: go/1.26/stable` with no `channel` is split into `snap: go` and `channel: 1.26/stable`.

## Validation

Each blocker is validated when it is checked; an invalid blocker yields an `ERROR` result with the message and does not stop the other blockers. Required fields per type are listed in [Sources](sources.md). An unknown `source.type` is reported as `unknown source type`. YAML keys fogcutter does not know are ignored.

## Merge rules

When the same run has config blockers, `-spec`, scanned annotations and scanned issues, they are concatenated in that order. Scanned code blockers receive `default_actions` (or a built-in `create_issue`). Issue-scanned blockers always get a `comment_issue` action on their issue. A `-spec` blocker has no actions.
