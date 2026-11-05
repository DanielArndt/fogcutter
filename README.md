# fogcutter

[![CI](https://github.com/DanielArndt/fogcutter/actions/workflows/ci.yml/badge.svg)](https://github.com/DanielArndt/fogcutter/actions/workflows/ci.yml)
[![License](https://img.shields.io/github/license/DanielArndt/fogcutter)](LICENSE)

`fogcutter` is a single-binary Go CLI and composite GitHub Action that periodically checks whether external blockers are resolved and triggers follow-up actions when they unblock.

## Features

- **Flexible Blocker Discovery**:
  - **Declarative YAML config** (`.github/blockers.yaml`)
  - **Source code scanning** (`--scan .`) for inline comments:
    - `// BLOCKER(ory/kratos#364): remove DSN workaround`
    - `# TODO(blocked-by:snap:lxd:5.21/stable>=5.21.2): upgrade LXD`
    - `// BLOCKER(github-release:openfga/openfga>=v1.8.0): adopt batch check`
    - `# BLOCKER(dep:goauthentik/authentik:go.mod:golang.org/x/net>=v0.62.0): drop x/net CVE override` (ref from the enclosing part's `source-tag`)
  - **GitHub Issue scanning** (`--scan-issues`) for `Blocked by owner/repo#123` or `BLOCKER(...)` in open issues.
- **Built-in Blocker Sources**:
  - `github-issue`: Checks if an external GitHub issue or PR is `closed` (with optional `reason: completed` or `merged`).
  - `snapcraft`: Queries the Snapcraft Store v2 API (`https://api.snapcraft.io/v2/snaps/info/<snap>`) for a given `channel` (e.g. `latest/stable`, `16/stable`) and `architecture` (`amd64`), comparing `version` (`>=16.4`) and/or `revision` (`>=29000`).
  - `github-release`: Checks if a repository has published a release or tag matching a version constraint (`>=v1.8.0`).
  - `http`: Queries any HTTP/JSON endpoint and checks status code, dot-path JSON fields (`json_path`), `equals`, `regex`, or `version`.
  - `command`: Runs a custom shell command and checks exit code `0` (plus optional stdout version/regex match).
  - `dependency`: Reads a dependency's version from an upstream repo's `go.mod`, `uv.lock`, `poetry.lock`, `pdm.lock`, `Cargo.lock`, `package-lock.json` or `requirements*.txt` at a ref and checks it against a constraint. Every copy in a lockfile must match. The ref can be a literal tag, `latest` (highest non-prerelease release, not the most recently published), or read from a local file with `ref_file` + `ref_regex` so it follows the upstream version you actually build.
- **Automated Follow-Up Actions (`on_resolved`)**:
  - `create_issue`: Opens a GitHub issue in your repository with templated title/body/labels/assignees.
  - `comment_issue`: Comments on an existing blocked issue or PR (e.g. to notify maintainers or mention `@claude` / `@copilot`).
  - `workflow_dispatch`: Triggers a GitHub Actions workflow with templated inputs.
  - `repository_dispatch`: Emits a `repository_dispatch` event with blocker metadata in `client_payload`.
  - `command`: Runs a custom shell command or launches an autonomous coding agent CLI with `FOGCUTTER_*` environment variables.
- **Idempotent Deduplication**:
  - Created issues and comments automatically include a hidden HTML marker (`<!-- fogcutter:id=<id> -->`). On subsequent cron runs, `fogcutter` checks for the marker so it never opens duplicate issues or posts duplicate comments.
  - Also supports an optional JSON `--state-file` for tracking triggered non-issue actions (`workflow_dispatch`, `command`, etc.).

## Recipe: retiring local CVE overrides

Packaging repos often patch a vulnerable transitive dependency on top of the upstream source, e.g. in a `rockcraft.yaml`:

```yaml
override-build: |
  go get golang.org/x/net@v0.62.0   # GO-2026-6860
```

Once upstream ships that version itself, the override is dead weight, but "the new release probably bumped it" is not something to trust. Annotate each override with the floor it guarantees, right next to the pin:

```yaml
parts:
  go-server:
    source: https://github.com/goauthentik/authentik
    source-tag: version/2026.5.8
    override-build: |
      # BLOCKER(dep:goauthentik/authentik:go.mod:golang.org/x/net>=v0.62.0): drop x/net override (GO-2026-6860)
      go get golang.org/x/net@v0.62.0
```

When a `dep:` annotation in a craft YAML file (`rockcraft.yaml`, `snapcraft.yaml`, ...) has no `@ref`, fogcutter uses the `source-tag` (or `source-commit`) of the part that contains it, so the check follows whatever Renovate bumps. The part's `source:` must point at the same repo; otherwise, or when the part has no tag, the latest release is used. Write `@latest` to always check the latest release, or `@<tag>` to pin one.

The same blocker in `.github/blockers.yaml`, for when you'd rather keep it out of the build file:

```yaml
blockers:
  - id: go-x-net
    description: "go get golang.org/x/net (GO-2026-6860)"
    source:
      type: dependency
      repo: goauthentik/authentik
      ref_file: rockcraft.yaml
      ref_regex: 'source-tag: (\S+)'
      manifest: go.mod
      package: golang.org/x/net
      version: ">=v0.62.0"
```

Run it on the upgrade pull request: blockers that resolve tell you which overrides to delete in that same PR. Run it on a schedule with `@latest` / `ref: latest` instead to learn early that an upgrade will retire an override.

Record a pinned version (`go get mod@vX`, `pkg>=X`), not `go get -u mod`: a floating upgrade has no version to compare against, and it silently pulls in unrelated changes.

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
