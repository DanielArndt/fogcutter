# How a run works

This page describes what `fogcutter check` does from start to finish, and the reasons behind the structure. For exact behaviour of each piece see the [reference](../README.md#reference).

## The packages

| Package | Role |
| :--- | :--- |
| `cmd/fogcutter` | Parses flags, orchestrates a run, prints reports. |
| `internal/config` | YAML schema, default IDs, validation. |
| `internal/scanner` | Finds blockers in files and issues; parses compact specs. |
| `internal/checker` | Evaluates a blocker against its source. |
| `internal/manifest` | Reads versions from lockfiles. |
| `internal/version` | Compares version strings. |
| `internal/action` | Executes follow-up actions, deduplicates, keeps state. |

## The pipeline

```text
config file ─┐
-spec ───────┤
-scan files ─┼─▶ list of blockers ─▶ checker ─▶ resolved? ─▶ actions ─▶ report
-scan-issues ┘                          │              │
                                        └ error ───────┘ (reported, run continues)
```

### 1. Discovery

Blockers come from up to four places, merged into one ordered list: the config's `blockers`, the `-spec` flag, the file scan, and the issue scan. They all end up as the same `Blocker` structure, a source plus a list of actions, so everything downstream treats them identically. This is why a spec string, a YAML block and a code comment are interchangeable.

Defaults are attached at discovery time. Config blockers without actions inherit `default_actions`; scanned code blockers inherit them or a built-in `create_issue`; issue-scanned blockers get a `comment_issue` on the issue they were found in.

### 2. Evaluation

Each blocker is validated and handed to the checker, which dispatches on the source type. The result is one of:

- *resolved*, with a resolution message,
- *blocked*, with an explanation of what is still missing,
- *error*, when the check itself could not be performed.

The distinction between blocked and error matters. "Not yet" is normal and exit status `0`. "I could not tell" (HTTP 500, bad config) is a problem the operator should see, and exit status `1`. Errors affect only their own blocker; the rest are still checked.

Blockers are evaluated sequentially, in order.

### 3. Actions

If a blocker is resolved, its actions run in order. Each action renders its templates against the result, checks for duplicates (see below), and then either executes, previews (`-dry-run`), or reports a skip. Action failures are recorded and do not prevent later actions or blockers from running.

### 4. Reporting

The run assembles a single report: counts, per-blocker results, per-action records. It is printed as text or JSON and, in GitHub Actions, exposed as step outputs and a summary so later workflow steps can branch on it.

### 5. Exit status

`0` when everything went well, `1` if anything errored or if `-fail-on-resolved` was requested and something resolved. That makes the same command usable as a scheduled notifier and as a CI gate.

## Network and trust boundaries

- Sources talk to the GitHub REST API, the Snap Store API, and any URL you configure.
- A `GITHUB_TOKEN` is sent only to GitHub endpoints. It is optional for reading public data but raises rate limits and is required for writing.
- `command` sources and actions run shell code with your privileges and environment. Treat the config file as code.

## Why sequential and stateless

fogcutter holds no state between runs except what it can rediscover from the target system or an optional JSON file. That keeps it a single binary, simple to reason about in CI, and easy to run on a laptop for a preview. The cost is that every run re-checks every blocker; for the volumes this tool is meant for (tens to hundreds of blockers, run daily) that is fine.
