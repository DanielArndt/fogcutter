# AGENTS.md

Guidelines for AI coding agents (and humans) working in this repository. See [CONTRIBUTIONS.md](CONTRIBUTIONS.md) for the full contributing guide.

## Project overview

`fogcutter` is a single-binary Go CLI that checks whether external blockers are resolved and triggers follow-up actions. The module is `github.com/DanielArndt/fogcutter`; the only direct dependency is `gopkg.in/yaml.v3`.

| Path | Purpose |
| :--- | :--- |
| `cmd/fogcutter` | CLI entry point |
| `internal/checker` | Blocker evaluation and the built-in sources |
| `internal/scanner` | Source code and GitHub issue scanning |
| `internal/action` | `on_resolved` follow-up actions |
| `internal/config` | YAML config schema |
| `internal/manifest` | Dependency manifest and lockfile parsing |
| `internal/version` | Version constraint handling |

## Build, test and lint

Use the Make targets; CI runs the same ones.

```bash
make build   # build ./fogcutter and compile all packages
make test    # go vet + go test -race ./...
make lint    # gofmt check + golangci-lint (v2)
make fmt     # gofmt -w .
make all     # lint, test, build
```

Run `make all` before proposing a change, and fix failures at the root cause instead of silencing them. Add or update tests alongside behavior changes.

## Commit messages and PR titles (mandatory)

All commits and pull request titles **must** follow [Conventional Commits](https://www.conventionalcommits.org/). PRs are squash-merged and the PR title becomes the commit on `main`; a CI check rejects titles that do not conform.

- Format: `<type>(<optional scope>): <description>`
- Types: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`
- Description: imperative mood, starts with a lowercase letter, no trailing period, under 72 characters
- Mark breaking changes with `!` before the colon or a `BREAKING CHANGE:` footer

See [CONTRIBUTIONS.md](CONTRIBUTIONS.md) for scopes and examples.

## Git workflow

- **Never commit directly to `main`.** It is protected; all changes go through a pull request.
- Start each piece of work on a new branch off an up-to-date `main`, named like `feat/dependency-source` or `fix/scanner-multiline`.
- If you already have uncommitted work or are on another branch, use a separate `git worktree` for the new branch instead of switching.
- Keep one logical change per PR, and link the issue with `Closes #123` in the PR description.
- Do not rewrite history on shared branches or push to `main`.

## Code conventions

- Match the surrounding style; `gofmt` and golangci-lint (see `.golangci.yml`) must pass.
- Every Go and YAML file starts with the SPDX header used by the existing files (`SPDX-FileCopyrightText` and `SPDX-License-Identifier: Apache-2.0`); add it to new files.
- Pin every GitHub Action to a full commit SHA with a version comment (e.g. `# v7.0.1`); Dependabot keeps pins current.
- [fogcutter-action](https://github.com/DanielArndt/fogcutter-action) downloads the release archives and verifies them against `checksums.txt`; keep the archive names (`fogcutter_<tag>_<os>_<arch>.<ext>`) and the checksum file stable, or update the Action in step.
- Never commit secrets or tokens, and never log them.
