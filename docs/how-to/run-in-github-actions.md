# How to run fogcutter in GitHub Actions

The supported way is [fogcutter-action](https://github.com/DanielArndt/fogcutter-action). For a lesson, see the [tutorial](../tutorials/02-automate-with-github-actions.md). This page is the checklist.

## Minimal workflow

```yaml
name: Check External Blockers
on:
  schedule:
    - cron: "0 8 * * *"
  workflow_dispatch:

permissions:
  contents: read
  issues: write
  actions: write

jobs:
  fogcutter:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: DanielArndt/fogcutter-action@v1
        with:
          config: .github/blockers.yaml
          scan: "."
          scan_issues: "true"
```

Pin actions to a full commit SHA with a version comment, as this repository does in [`.github/workflows/fogcutter.yml`](../../.github/workflows/fogcutter.yml).

## Permissions per action type

| You use | Token permission needed |
| :--- | :--- |
| `create_issue`, `comment_issue` | `issues: write` (`pull-requests: write` to comment on PRs) |
| `workflow_dispatch` | `actions: write` on the target repository |
| `repository_dispatch` | write access to repository contents on the target |
| `github-issue` / `dependency` against private repos | read access to those repos, via a PAT |
| `scan_issues` | `issues: read` |

The built-in `GITHUB_TOKEN` only covers the current repository. For cross-repo actions use a fine-grained PAT stored as a secret.

## Preview before going live

Add a `dry_run` input to `workflow_dispatch` and pass it through, as in the tutorial. The job summary shows what *would* happen.

## Reading the results

fogcutter writes outputs when `$GITHUB_OUTPUT` is set (always inside Actions): `total_blockers`, `resolved_count`, `has_resolved`, and `resolved_json`. It also appends a table to `$GITHUB_STEP_SUMMARY`. See [Output formats](../reference/output.md).

## Running the binary yourself

If you do not use the action, download a release archive, verify `checksums.txt`, and run:

```bash
fogcutter check -config .github/blockers.yaml -scan . -state-file .fogcutter-state.json
```

with `GITHUB_TOKEN` in the environment and `GITHUB_REPOSITORY` set (Actions sets it for you).
