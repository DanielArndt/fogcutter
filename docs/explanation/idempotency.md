# Idempotency and deduplication

A scheduled tool must be safe to run twice. If a blocker resolves on Monday and fogcutter runs every day, Tuesday's run must not open a second issue.

## Two mechanisms

### 1. Markers in GitHub (self-describing)

When fogcutter creates an issue or posts a comment, it appends a hidden HTML comment:

```html
<!-- fogcutter:id=<blocker id> -->
```

Before creating or commenting, it looks for that marker in the target:

- `create_issue`: in the 100 most recent issues of the repository (all states, so closed ones count),
- `comment_issue`: in the first 100 comments of the target issue.

If found, the action is skipped (`skipped-duplicate`). The state lives in GitHub itself, so any machine can run fogcutter, CI runners can be ephemeral, and nothing needs persisting. Deleting the issue or comment re-arms the action.

### 2. A state file (explicit)

The marker approach cannot work for `command`, `workflow_dispatch` and `repository_dispatch`: they leave no trace on GitHub that fogcutter can query. For these, `state_file` records `<blocker id>::<action type>` and a timestamp after the action runs. Later runs see the record and skip.

The two mechanisms are independent. For issues and comments you generally do not need the state file; for the other three, you generally do.

## Why the key is `blocker id` + `action type`

It is simple and covers the common case of one action of each type per blocker. The trade-off is that two actions of the same type on one blocker share a key, so the second is skipped after the first has run. If you need that, set `deduplicate: false` on the one that should always run, or merge them.

## Why IDs matter

Both mechanisms hinge on the blocker ID. IDs of declared blockers are explicit or derived from the source, so they are stable. IDs of annotations include `path:line`, so moving the comment produces a new blocker as far as fogcutter is concerned, and a new issue can be opened. This is deliberate: fogcutter has no way to know that a moved line is "the same" annotation, and a false merge would silence a real one. If you expect to move annotated lines a lot, declare the blocker in config with an explicit `id`.

## Dry runs

`-dry-run` never writes the state file and never creates anything. If a token is available it still performs the read-only marker lookups so the preview is truthful (it will say `skipped-duplicate` where a real run would).

## Failure modes

| Situation | What happens |
| :--- | :--- |
| Run crashes after creating the issue but before anything else | The marker is in the issue; the next run skips. |
| Command succeeded but the state file was not saved | The command can run again. Save failure is reported and exits `1`. |
| Marker lookup fails (API error) | In a real run the action fails with an error rather than risk a duplicate. |
| Repository has more than 100 newer issues than the one holding the marker | The marker is not seen and a duplicate may be created. Use a state file as well. |
| The issue holding the marker is edited to remove it | Treated as not existing; the action re-fires. |

## See also

- [Persist state between runs](../how-to/persist-state.md)
- [Limitations and sharp edges](limitations.md)
