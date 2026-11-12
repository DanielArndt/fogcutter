# Actions reference

Actions are listed under `on_resolved` (per blocker) or `default_actions` (global). They run, in order, for each blocker that is resolved. Titles, bodies, workflow inputs, commands and `env` values are [templates](templates.md).

Every action produces an execution record with a `status`:

| Status | Meaning |
| :--- | :--- |
| `executed` | Done. |
| `dry-run` | Would have been done (`-dry-run`). |
| `skipped-duplicate` | A GitHub marker or [state file](../how-to/persist-state.md) entry says it was already done. |
| `skipped-closed` | `comment_issue` target is already closed. |
| `error` | Failed; fogcutter will exit `1`. |

## Common fields

| Field | Description |
| :--- | :--- |
| `type` | **Required.** One of the types below. |
| `repo` | Target repository, overriding `repository` / `-repo` / `$GITHUB_REPOSITORY`. |
| `deduplicate` | Default `true`. `false` disables marker lookup and state-file skipping for this action. |

## create_issue

Opens an issue.

| Field | Default | Description |
| :--- | :--- | :--- |
| `title` | `Blocker resolved: {{.SourceSummary}}` | Template. |
| `body` | summary with ID, source, resolution, description and location | Template. A hidden `<!-- fogcutter:id=<id> -->` marker is appended. |
| `labels` | none | Labels to apply. |
| `assignees` | none | Logins to assign. |

Deduplication: before creating, fogcutter lists the 100 most recent issues of the repository (all states) and skips if one contains the marker. In `-dry-run` the lookup only happens when a token is present.

## comment_issue

Comments on an issue or PR.

| Field | Default | Description |
| :--- | :--- | :--- |
| `issue` | **required** | Issue/PR number (positive). |
| `body` | `External blocker **{{.ID}}** (`{{.SourceSummary}}`) is now resolved: {{.Resolution}}` | Template; marker appended. |
| `labels` | none | Labels to add after commenting. Failures are ignored. |
| `remove_labels` | none | Labels to remove after commenting. Failures are ignored. |
| `skip_if_closed` | `true` | Skip when the target is already closed. |

Deduplication: looks for the marker in the first 100 comments of the target.

## workflow_dispatch

Triggers a GitHub Actions workflow (`POST /repos/{repo}/actions/workflows/{workflow}/dispatches`).

| Field | Default | Description |
| :--- | :--- | :--- |
| `workflow` | **required** | File name (`rebuild.yml`) or numeric ID. |
| `ref` | `main` | Branch or tag to run on. |
| `inputs` | none | Map of string inputs; values are templates. |

The workflow must have an `on: workflow_dispatch` trigger declaring those inputs. Not deduplicated on GitHub; use a [state file](../how-to/persist-state.md).

## repository_dispatch

Sends a `repository_dispatch` event.

| Field | Default | Description |
| :--- | :--- | :--- |
| `event_type` | `blocker_resolved` | Event type. |

`client_payload` is always:

```json
{
  "blocker_id": "…",
  "description": "…",
  "location": "…",
  "resolution": "…",
  "source_summary": "…"
}
```

Not deduplicated on GitHub; use a state file.

## command

Runs a shell command with `sh -c`.

| Field | Description |
| :--- | :--- |
| `command` | **Required.** Template. |
| `env` | Map of extra environment variables; values are templates (an unrenderable value is used verbatim). |

The process inherits fogcutter's environment plus:

| Variable | Value |
| :--- | :--- |
| `FOGCUTTER_ID` | blocker ID |
| `FOGCUTTER_DESCRIPTION` | description |
| `FOGCUTTER_LOCATION` | location |
| `FOGCUTTER_RESOLUTION` | resolution text |
| `FOGCUTTER_SOURCE` | source summary |

Stdout and stderr are combined. Exit `0` is `executed` with the output as `detail`; non-zero is `error`. Not deduplicated on GitHub; use a state file.

## Target repository resolution

`action.repo` → `-repo` → config `repository` → `$GITHUB_REPOSITORY`. If none is set, GitHub actions fail with an explicit message.

## Authentication

All GitHub actions send `Authorization: Bearer <token>` using `GITHUB_TOKEN` (or `GH_TOKEN`). Required permissions are in [Run fogcutter in GitHub Actions](../how-to/run-in-github-actions.md#permissions-per-action-type).
