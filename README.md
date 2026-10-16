# fogcutter

`fogcutter` is a single-binary Go CLI and composite GitHub Action that periodically checks whether external blockers are resolved and triggers follow-up actions when they unblock.

## Features

- **Flexible Blocker Discovery**:
  - **Declarative YAML config** (`.github/blockers.yaml`)
  - **Source code scanning** (`--scan .`) for inline comments:
    - `// BLOCKER(ory/kratos#364): remove DSN workaround`
    - `# TODO(blocked-by:snap:lxd:5.21/stable>=5.21.2): upgrade LXD`
    - `// BLOCKER(github-release:openfga/openfga>=v1.8.0): adopt batch check`
  - **GitHub Issue scanning** (`--scan-issues`) for `Blocked by owner/repo#123` or `BLOCKER(...)` in open issues.
- **Built-in Blocker Sources**:
  - `github-issue`: Checks if an external GitHub issue or PR is `closed` (with optional `reason: completed` or `merged`).
  - `snapcraft`: Queries the Snapcraft Store v2 API (`https://api.snapcraft.io/v2/snaps/info/<snap>`) for a given `channel` (e.g. `latest/stable`, `16/stable`) and `architecture` (`amd64`), comparing `version` (`>=16.4`) and/or `revision` (`>=29000`).
  - `github-release`: Checks if a repository has published a release or tag matching a version constraint (`>=v1.8.0`).
  - `http`: Queries any HTTP/JSON endpoint and checks status code, dot-path JSON fields (`json_path`), `equals`, `regex`, or `version`.
  - `command`: Runs a custom shell command and checks exit code `0` (plus optional stdout version/regex match).
- **Automated Follow-Up Actions (`on_resolved`)**:
  - `create_issue`: Opens a GitHub issue in your repository with templated title/body/labels/assignees.
  - `comment_issue`: Comments on an existing blocked issue or PR (e.g. to notify maintainers or mention `@claude` / `@copilot`).
  - `workflow_dispatch`: Triggers a GitHub Actions workflow with templated inputs.
  - `repository_dispatch`: Emits a `repository_dispatch` event with blocker metadata in `client_payload`.
  - `command`: Runs a custom shell command or launches an autonomous coding agent CLI with `FOGCUTTER_*` environment variables.
- **Idempotent Deduplication**:
  - Created issues and comments automatically include a hidden HTML marker (`<!-- fogcutter:id=<id> -->`). On subsequent cron runs, `fogcutter` checks for the marker so it never opens duplicate issues or posts duplicate comments.
  - Also supports an optional JSON `--state-file` for tracking triggered non-issue actions (`workflow_dispatch`, `command`, etc.).

## Quick Start

### CLI

```bash
# Build
go build -o fogcutter ./cmd/fogcutter

# Preview blockers and actions in dry-run mode
./fogcutter check -config .github/blockers.example.yaml -dry-run

# Scan source directory for BLOCKER(...) or TODO(blocked-by:...) comments
./fogcutter check -scan . -dry-run

# List all discovered blockers without querying upstream APIs
./fogcutter list -config .github/blockers.example.yaml -scan .
```

### GitHub Action

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
      - uses: DanielArndt/fogcutter@main
        with:
          config: .github/blockers.yaml
          scan: "."
          scan_issues: "true"
          github_token: ${{ secrets.GITHUB_TOKEN }}
```
