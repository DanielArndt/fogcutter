# How to notify a blocked issue when it unblocks

Comment on the issue or PR that is waiting, optionally swapping labels, so people (or bots such as `@claude` or `@copilot`) are notified.

## In config

```yaml
blockers:
  - id: login-ui-983-go-1.26.9
    source:
      type: snapcraft
      snap: go/1.26/stable
      version: ">=1.26.9"
    on_resolved:
      - type: comment_issue
        repo: canonical/identity-platform-login-ui   # defaults to the top-level repository
        issue: 983
        labels: ["unblocked"]
        remove_labels: ["blocked"]
        body: |
          `go/1.26/stable` now has Go >= 1.26.9 ({{.Resolution}}).
          This issue is unblocked and ready for a rebuild.
```

## From the command line

```bash
fogcutter add \
  -spec 'snap:go/1.26/stable>=1.26.9' \
  -issue canonical/identity-platform-login-ui#983 \
  -body 'Go 1.26.9 is available: {{.Resolution}}'
```

`-issue` accepts `983`, `#983`, `owner/repo#983` or a full issue URL.

## Behaviour to know

- The comment ends with a hidden marker; if a comment with that marker already exists, nothing is posted (`skipped-duplicate`).
- If the target issue is already closed, the action is skipped (`skipped-closed`). Set `skip_if_closed: false` to comment anyway.
- Labels are added and removed *after* the comment is posted, on a best-effort basis: failures there do not fail the action.
- The token needs `issues: write` (or `pull-requests: write` for PRs) on the target repository. With the default `GITHUB_TOKEN` in Actions that is limited to the current repository; use a token with wider access for cross-repo comments.

## Mention an agent

```yaml
body: |
  Upstream blocker {{.Resolution}}.
  @claude please remove the DSN workaround in `internal/db/conn.go` and open a PR.
```

See also [Launch a coding agent](launch-a-coding-agent.md) for running one locally.
