# Tutorial: automate follow-up with GitHub Actions

In this tutorial you will move from running fogcutter by hand to running it every day in GitHub Actions. When a blocker clears, fogcutter will open an issue in your repository, and it will not open a second one the next day.

You need:

- a GitHub repository you can push to,
- the [first tutorial](01-your-first-blocker.md) completed, or at least familiarity with annotations,
- about fifteen minutes.

## 1. Add a blocker to the repository

In your repository, add a workaround with an annotation. Any source file works. For example, in a `Dockerfile`:

```dockerfile
# BLOCKER(github-release:openfga/openfga>=v1.8.0): switch to the native batch-check API
RUN echo "using the old client-side batching"
```

Commit it.

## 2. Add a configuration file

Create `.github/blockers.yaml`. This tells fogcutter where to scan and what to do by default when a blocker resolves:

```yaml
scan:
  paths: ["."]
  ignore: ["docs", "*_test.go"]

default_actions:
  - type: create_issue
    title: "Unblocked: {{.SourceSummary}}"
    body: |
      External blocker **{{.ID}}** is now resolved.

      - **Source:** `{{.SourceSummary}}`
      - **Resolution:** {{.Resolution}}
      - **Location:** `{{.Location}}`
      - **Context:** {{.Description}}
    labels: ["unblocked"]
```

Notes:

- `repository:` is omitted because inside GitHub Actions fogcutter reads `$GITHUB_REPOSITORY`.
- `ignore` stops fogcutter from picking up example annotations in documentation or tests.
- The `{{...}}` parts are [templates](../reference/templates.md) filled in when the issue is created.

## 3. Preview locally

Before automating, preview it:

```bash
fogcutter check -repo you/your-repo -dry-run
```

fogcutter reads `.github/blockers.yaml` by default. You should see the blocker and, if it is resolved, a `[DRY-RUN]` line for `create_issue`.

## 4. Add the workflow

Create `.github/workflows/fogcutter.yml`:

```yaml
name: Check External Blockers

on:
  schedule:
    - cron: "0 8 * * *"
  workflow_dispatch:
    inputs:
      dry_run:
        description: "Preview follow-up actions without executing them"
        type: boolean
        default: false

permissions:
  contents: read
  issues: write

jobs:
  fogcutter:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: DanielArndt/fogcutter-action@v1
        with:
          config: .github/blockers.yaml
          dry_run: ${{ inputs.dry_run || false }}
          github_token: ${{ secrets.GITHUB_TOKEN }}
```

> In your own repositories, pin actions to a full commit SHA with a version comment, as this project does. See [`.github/workflows/fogcutter.yml`](../../.github/workflows/fogcutter.yml).

Commit and push to your default branch.

## 5. Run it once

In the GitHub UI open **Actions → Check External Blockers → Run workflow**, tick **dry_run**, and run it. Open the run and look at the **job summary**. fogcutter writes a table of every blocker, whether it is resolved, and what each follow-up would do.

Run it again with **dry_run** unticked. If the blocker is resolved, an issue titled *Unblocked: …* appears in your repository. The issue body ends with a hidden marker:

```html
<!-- fogcutter:id=code:Dockerfile:1:github-release:openfga/openfga>=v1.8.0 -->
```

## 6. Run it again

Trigger the workflow a second time. No second issue is created; the job summary shows the action as `skipped-duplicate`. fogcutter found its own marker on the existing issue and left it alone. That is what makes it safe to run on a schedule.

## What you learned

- Annotations plus `default_actions` give you zero-per-blocker configuration.
- The workflow only needs `issues: write` for `create_issue` (add `actions: write` if you use `workflow_dispatch`).
- A hidden marker makes repeated runs idempotent.

## Next steps

- [Notify a blocked issue when it unblocks](../how-to/notify-a-blocked-issue.md)
- [Launch a coding agent when a blocker resolves](../how-to/launch-a-coding-agent.md)
- [Idempotency and deduplication](../explanation/idempotency.md)
- All action inputs and outputs: the [fogcutter-action README](https://github.com/DanielArndt/fogcutter-action#readme)
