# Output formats

`check` reports through `-format`, and, inside GitHub Actions, through step outputs and the step summary.

## `text` (default)

```text
Checked 2 blocker(s): 1 resolved

[RESOLVED] <id> (<source summary>) @ <location>
  -> <resolution>
[BLOCKED] <id> (<source summary>)
  -> <explanation>
[ERROR] <id> (<source summary>)
  -> <error>

Follow-up Actions:
  - [EXECUTED] <id> (create_issue): created issue [owner/repo#7: title](url)
```

The heading reads `Follow-up Actions (dry-run):` with `-dry-run`. With no blockers: `No blockers found.`.

## `json`

```json
{
  "total_blockers": 1,
  "resolved_count": 1,
  "results": [
    {
      "blocker": { "id": "…", "description": "…", "location": "…", "source": { "type": "…" }, "on_resolved": [] },
      "resolved": true,
      "resolution": "…",
      "source_summary": "…",
      "checked_at": "2026-11-06T13:43:02.879141874Z",
      "error": "only present on failure"
    }
  ],
  "actions": [
    {
      "blocker_id": "…",
      "type": "create_issue",
      "target": "owner/repo",
      "status": "executed",
      "detail": "…",
      "error": "only present on failure"
    }
  ]
}
```

`actions` is omitted when no action ran. `status` values are listed in [Actions](actions.md). Field names inside `blocker.source` and `on_resolved` match the [configuration keys](configuration.md). Characters such as `>` and `<` are escaped as `\u003e` and `\u003c`, which JSON parsers decode normally.

For `list`, `-format json` prints the array of blockers.

## `github`

Behaves like `text` on stdout and additionally writes the GitHub outputs and summary below (which happen anyway if the environment variables are set).

## `$GITHUB_OUTPUT`

If `$GITHUB_OUTPUT` is set (always in Actions) these lines are appended:

| Output | Value |
| :--- | :--- |
| `total_blockers` | number of blockers checked |
| `resolved_count` | number resolved |
| `has_resolved` | `true` or `false` |
| `resolved_json` | JSON array of the resolved results (same shape as `results[]`); `null` if none |

## `$GITHUB_STEP_SUMMARY`

If set, a Markdown report is appended: a heading `Fogcutter Report (<resolved>/<total> resolved)`, a table with columns Status, Blocker ID, Source and Details (⏳ Blocked, ✅ Resolved, ❌ Error), and a *Follow-up Actions* list when any ran.

## Exit status

See [exit codes](cli.md#exit-codes).
