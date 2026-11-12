# Command line reference

```text
fogcutter [command] [flags]
```

If the first argument is missing or starts with `-`, the command is `check`. Flags use Go's `flag` syntax: `-flag value`, `-flag=value`, or `--flag`.

## Commands

| Command | Purpose |
| :--- | :--- |
| `check` | Evaluate blockers and run follow-up actions. The default. |
| `list` | Print discovered blockers. Does not query upstream sources. |
| `add` | Append a blocker to the config file, or print it. |
| `version`, `--version`, `-v` | Print `fogcutter v<version>`. |
| `help`, `--help`, `-h` | Print usage. |

An unknown command prints usage to stderr and exits `2`.

## `fogcutter check` and `fogcutter list`

Both accept the same flags.

| Flag | Default | Description |
| :--- | :--- | :--- |
| `-config <path>` | `.github/blockers.yaml` | Config file. A missing default file is ignored; a missing explicit path is an error. |
| `-spec <spec>` | none | Ad-hoc blocker. See [spec grammar](annotations.md#spec-grammar). Has the ID equal to the spec text and **no follow-up actions**. |
| `-scan <paths>` | none | Comma-separated files/directories to scan for [annotations](annotations.md). Added to `scan.paths` from config. |
| `-scan-issues` | `false` | Also scan open issues in the target repository. Also enabled by `scan.issues: true`. |
| `-repo <owner/name>` | config `repository`, else `$GITHUB_REPOSITORY` | Target repository for actions and issue scanning. Overrides config. |
| `-state-file <path>` | config `state_file` | JSON state file. Overrides config. |
| `-dry-run` | `false` | Evaluate and preview actions without executing them. `check` only. |
| `-format <fmt>` | `text` | `text`, `json`, or `github`. See [Output formats](output.md). |
| `-fail-on-resolved` | `false` | Exit `1` if any blocker is resolved. `check` only. |

### `check` behaviour

1. Load the config, apply `-repo` and `-state-file`.
2. Collect blockers: config blockers, then `-spec`, then file scan, then issue scan.
3. Evaluate each blocker. Blockers with an evaluation error are reported and skipped.
4. For each resolved blocker run its `on_resolved` actions.
5. Save the state file (unless `-dry-run`).
6. Write `$GITHUB_OUTPUT` / `$GITHUB_STEP_SUMMARY` if applicable, then print the report.

### `list` behaviour

Prints each blocker as `- <id> (<source summary>) [<location>]: <description>`, or `No blockers configured or discovered.`. With `-format json` prints the array of blockers (including `on_resolved`). `-scan-issues` does call GitHub.

## `fogcutter add`

Adds a blocker to the config file (default `.github/blockers.yaml`), creating the file and its directory if needed.

| Flag | Default | Description |
| :--- | :--- | :--- |
| `-spec <spec>` | **required** | Blocker spec. |
| `-config <path>` | `.github/blockers.yaml` | Config file to modify. |
| `-id <id>` | derived from spec | Blocker ID. The spec with `: / > < = #` replaced by `-`. |
| `-desc <text>` | empty | Description. |
| `-issue <ref>` | none | Add a `comment_issue` action. Accepts `983`, `#983`, `owner/repo#983`, or an issue URL. |
| `-body <text>` | empty | Body template for the `comment_issue` action, or for the default `create_issue` action. |
| `-workflow <file>` | none | Add a `workflow_dispatch` action on ref `main`. |
| `-command <cmd>` | none | Add a `command` action. |
| `-print` | `false` | Print the YAML entry to stdout instead of writing. |

If none of `-issue`, `-workflow`, `-command` is given, a `create_issue` action titled `Unblocked: {{.SourceSummary}}` is added. The file is re-serialised, so **existing comments and formatting are lost**.

## Exit codes

| Code | Meaning |
| :--- | :--- |
| `0` | Success. |
| `1` | Any of: config could not be loaded; a source returned an error; an action failed; the state file could not be loaded or saved; scanning failed; or `-fail-on-resolved` was set and at least one blocker is resolved. |
| `2` | Usage error: unknown command, flag parse failure, or `add` without `-spec`. |

A blocker that is simply not resolved yet is not an error.

## Environment variables

| Variable | Used for |
| :--- | :--- |
| `GITHUB_TOKEN`, `GH_TOKEN` | Token for the GitHub API (`GITHUB_TOKEN` wins). Optional for reading public data, required for writing. |
| `GITHUB_REPOSITORY` | Default target repository. |
| `GITHUB_OUTPUT` | If set, step outputs are appended to this file. |
| `GITHUB_STEP_SUMMARY` | If set, a Markdown report is appended to this file. |

The Snap Store is queried without credentials. Network timeouts: 20 seconds per source request, 30 seconds per action request.
