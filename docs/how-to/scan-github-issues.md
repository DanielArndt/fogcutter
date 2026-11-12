# How to scan GitHub issues for blockers

Let people record blockers in the issue tracker instead of in code.

## Write the blocker in an issue

In an issue (or PR) body or comment, put a line starting with `Blocked by` or `Blocked on`, or use the `BLOCKER(...)` form:

```text
Blocked by ory/kratos#364
Blocked on snap:go/1.26/stable>=1.26.9
- Blocked by https://github.com/owner/repo/issues/123
BLOCKER(github-release:openfga/openfga>=v1.8.0): adopt batch check
```

## Scan

```bash
fogcutter check -scan-issues -repo owner/name -dry-run
```

or in config:

```yaml
repository: owner/name
scan:
  issues: true
  issue_label: blocked      # optional: only scan issues with this label
```

Set `GITHUB_TOKEN` (or `GH_TOKEN`) to avoid the unauthenticated rate limit and to see private repositories.

## What happens when one resolves

Each blocker found this way gets a `comment_issue` action on the issue it was found in. The default comment is:

> External blocker `<source>` is now resolved (<resolution>).
>
> <description>

The comment carries a hidden marker, so the same blocker never produces a second comment. `default_actions` from the config are **not** applied to issue-scanned blockers.

## Limits

- Only **open** issues are scanned, and only the first page of 100 results; fogcutter does not paginate.
- For each issue, the body plus the first 100 comments are read. Comments created by fogcutter itself are ignored.
- Pull requests are included because GitHub lists them as issues.
- The blocker ID is `issue:<repo>#<n><-<spec>`, stable across runs.
