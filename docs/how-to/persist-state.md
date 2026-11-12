# How to persist state between runs

A state file records which actions fogcutter already ran, so non-issue actions (commands, workflow dispatches) are not repeated on every run.

## Enable it

```bash
fogcutter check -state-file .fogcutter-state.json
```

or in config:

```yaml
state_file: .fogcutter-state.json
```

The flag overrides the config value. The file is created on the first non-dry-run, and looks like:

```json
{
  "triggered": {
    "openfga-batch-check::command": "2026-11-06T13:43:02.879141874Z"
  }
}
```

## Keep it between CI runs

State is only useful if the file survives from one run to the next. In GitHub Actions use the cache or an artifact:

```yaml
- uses: actions/cache@v4
  with:
    path: .fogcutter-state.json
    key: fogcutter-state-${{ github.run_id }}
    restore-keys: fogcutter-state-
- uses: DanielArndt/fogcutter-action@v1
  with:
    config: .github/blockers.yaml
    state_file: .fogcutter-state.json
```

The action's `state_file` input maps to `-state-file`; setting `state_file:` in the config works too. Alternatively commit the file to a dedicated branch.

## Rules

- A record is keyed by `<blocker id>::<action type>` and written only when the action's status is `executed`.
- A recorded key makes later runs report `skipped-duplicate`, for every action type, unless that action sets `deduplicate: false`.
- Two actions of the **same type** on one blocker share a key: after the first runs, the second is skipped on the next run, and in the same run too. Combine them or use `deduplicate: false`.
- Dry runs read the file (so you see `skipped-duplicate` previews) but never write it.
- To make an action fire again, delete its entry (or the file).
- Annotation IDs include `path:line`. Moving a commented line changes its ID, so state recorded for the old ID no longer applies. Declared blockers with an explicit `id:` do not have this problem.

For `create_issue` and `comment_issue` the GitHub marker already prevents duplicates; the state file is mainly for the other three action types. See [Idempotency and deduplication](../explanation/idempotency.md).
