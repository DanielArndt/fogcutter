# Limitations and sharp edges

Things that behave in ways you might not expect. They are documented here as facts of the current implementation, not as promises. Check the [CHANGELOG](../../CHANGELOG.md) for changes.

## Version comparison

- Any text after the first `-` or `~` is a pre-release, so snap versions like `6.10.1-f5c750d` sort *below* `6.10.1`.
- PEP 440 pre-/post-releases without a hyphen (`1.0.0rc1`, `1.0.0.post1`) sort *above* `1.0.0`.

Details and workarounds: [Version constraints](../reference/version-constraints.md).

## Discovery

- Annotation specs cannot contain `)`.
- Malformed specs are skipped silently; use `fogcutter list` to confirm discovery.
- `http` and `command` sources cannot be written inline.
- Annotation IDs include the line number, so moving a comment changes its identity.
- Documentation or tests that contain example annotations are scanned like any other file unless ignored.
- Issue scanning reads one page of 100 open issues and, per issue, one page of 100 comments.

## Deduplication

- The `create_issue` marker lookup only covers the 100 most recently created issues (all states); in a very busy repository an old marker can fall out of range.
- The `comment_issue` marker lookup covers the first 100 comments.
- State-file keys are `blocker id` + `action type`, so same-type actions on one blocker collide.
- `command`, `workflow_dispatch` and `repository_dispatch` repeat on every run unless a state file is used.

## Sources

- `github-release` considers only the 30 most recent releases (or tags, if there are no releases at all).
- `github-issue` with `reason` treats `merged` and `completed` the same for PRs.
- `http` headers are literal; there is no secret or environment substitution in config.
- `command` sources and actions require `sh` and have no timeout other than the process context.
- Source requests time out after 20 s, action requests after 30 s. There is no retry.

## Config tooling

- `fogcutter add` re-serialises the YAML and drops comments and custom formatting.
- Unknown YAML keys are ignored, so typos in key names do not raise errors; confirm with `fogcutter list -format json`.
- Config values are not expanded for environment variables.

## Execution

- Blockers are checked sequentially.
- `-dry-run` for `comment_issue`'s closed-target check and for marker lookups needs a token to be exact; without one, those checks are skipped in the preview.
- Labels added or removed by `comment_issue` are best-effort; failures are not reported.
- A failed action, a failed source, or a failed state save makes the whole run exit `1`, even if other actions succeeded.

## Reporting a problem

If you hit behaviour that is not documented here or in the reference, please [open an issue](https://github.com/DanielArndt/fogcutter/issues).
