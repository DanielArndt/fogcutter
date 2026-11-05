# Contributing to fogcutter

Thanks for your interest in contributing! Bug fixes, feature suggestions and documentation improvements are all welcome.

---

## Commit Message Guidelines

This project follows the **[Conventional Commits](https://www.conventionalcommits.org/)** specification (v1.0.0). Commit messages and pull request titles, whether written by people or by automated agents, must use this format.

### Commit Format

```text
<type>(<optional scope>): <description>

[optional body]

[optional footer(s)]
```

### Commit Types

| Type | Description |
| :--- | :--- |
| `feat` | A new feature or user-facing functionality |
| `fix` | A bug fix |
| `docs` | Documentation-only changes (README, CONTRIBUTIONS, code comments) |
| `style` | Code formatting and whitespace (no functional changes) |
| `refactor` | Code restructuring that neither fixes a bug nor adds a feature |
| `perf` | Performance improvements |
| `test` | Adding or updating tests |
| `build` | Build system, Makefile, packaging changes |
| `ci` | Continuous integration and deployment configuration |
| `chore` | Routine tasks, dependency updates, repo maintenance |
| `revert` | Reverting a previous commit |

### Commit Rules

1. **Imperative mood**: write `add dependency source`, not `added dependency source` or `adds dependency source`.
2. **Lowercase**: start the description with a lowercase letter.
3. **No trailing period**: do not end the subject line with a period.
4. **Length**: keep the subject line concise (under 72 characters).
5. **Scopes**: use an optional scope for the component you changed, for example:
   - `(checker)`: blocker evaluation and the built-in sources (`internal/checker`)
   - `(scanner)`: source code and issue scanning (`internal/scanner`)
   - `(action)`: `on_resolved` follow-up actions (`internal/action`)
   - `(config)`: YAML config schema (`internal/config`)
   - `(manifest)`: dependency manifest and lockfile parsing (`internal/manifest`)
   - `(version)`: version constraint handling (`internal/version`)
   - `(cli)`: the command line entry point (`cmd/fogcutter`)
6. **Breaking changes**: append `!` before the colon (for example `feat(config)!: rename blocker fields`) or add a `BREAKING CHANGE:` footer.

### Examples

- `feat: add dependency source for retiring local overrides`
- `fix(scanner): handle blockers inside multi-line comments`
- `refactor(checker): split snapcraft channel matching`
- `test(version): cover pre-release comparisons`
- `docs: add CONTRIBUTIONS.md`

---

## Pull Requests and Merging

- **PR titles are validated.** The `Validate PR Title` workflow checks every pull request title against the types and rules above whenever it is opened or edited. It is a required check on `main`.
- **PRs are squash-merged.** The PR title becomes the commit message on `main`, and the PR body becomes the commit body. Rebase merging is also allowed; merge commits are disabled.
- **Linear history.** `main` is protected: changes go through a PR, force-pushes and deletions are blocked, and the `test`, `lint` and `Validate PR Title` checks must pass.
- Head branches are deleted automatically after merge.

### Checklist

1. Use a descriptive branch name such as `feat/dependency-source` or `fix/scanner-multiline`.
2. Add or update tests for the behavior you change.
3. Make sure the tests and linters pass locally (see below).
4. Give the PR a Conventional Commits title.
5. In the description, say what changed, why, and how you tested it. Link issues with `Closes #123`.

---

## Development Workflow

### Prerequisites

- Go (the version in [`go.mod`](go.mod))
- [golangci-lint](https://golangci-lint.run/) v2
- `make`

### Make Targets

| Target | What it does |
| :--- | :--- |
| `make build` | Builds the CLI to `./fogcutter` and compiles all packages |
| `make test` | Runs `go vet` and `go test -race ./...` |
| `make lint` | Fails on unformatted files, then runs golangci-lint (configured in [`.golangci.yml`](.golangci.yml)) |
| `make fmt` | Formats all Go files with `gofmt -w` |
| `make all` | Runs `lint`, `test` and `build` |
| `make clean` | Removes the built binary |

Run `make all` before opening a PR. CI runs the same `make build test` and `make lint` targets on every pull request and on pushes to `main`.

---

## Releases and Versioning

Releases are meant to be automated with **[Release Please](https://github.com/googleapis/release-please)**, which reads the Conventional Commits on `main`:

- `fix` produces a patch release, `feat` produces a minor release, and a breaking change produces a major release.
- Release Please keeps an open Release PR that updates the changelog and version. Merging it tags the release and publishes the GitHub Release.

This is why the PR title matters: the squash-merge title is the commit Release Please reads. The workflow itself is tracked in [#3](https://github.com/DanielArndt/fogcutter/issues/3).
