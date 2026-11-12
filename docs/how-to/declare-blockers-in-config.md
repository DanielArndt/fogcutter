# How to declare blockers in a config file

Use a config file when you want a stable ID, several actions, or a source that annotations cannot express.

## Create the file

By default fogcutter reads `.github/blockers.yaml`:

```yaml
repository: owner/name        # optional; falls back to $GITHUB_REPOSITORY

blockers:
  - id: kratos-multistatements-fix
    description: "Remove multiStatements DSN workaround in internal/db/conn.go"
    source:
      type: github-issue
      repo: ory/kratos
      issue: 364
      reason: completed
    on_resolved:
      - type: create_issue
        title: "Remove Kratos DSN workaround (ory/kratos#364 closed)"
        labels: ["tech-debt", "unblocked"]
```

A complete example lives in [`.github/blockers.example.yaml`](../../.github/blockers.example.yaml). Every field is described in the [configuration reference](../reference/configuration.md).

## Check it

```bash
fogcutter list                    # parse and print
fogcutter check -dry-run          # evaluate, preview actions
```

Use another path with `-config path/to/file.yaml`. An explicitly named file that does not exist is an error; a missing *default* file is silently treated as an empty config.

## Add a blocker from the command line

`add` appends an entry for you:

```bash
fogcutter add -spec 'ory/kratos#364' -desc 'DSN workaround' -issue 42
```

Use `-print` to see the YAML without touching the file. Note that `add` rewrites the whole file from the parsed structure, so **comments in the existing file are lost**. If you hand-maintain comments, use `-print` and paste the result. See [`add` in the CLI reference](../reference/cli.md#fogcutter-add).

## Share one action across blockers

Blockers that have no `on_resolved` inherit `default_actions`:

```yaml
default_actions:
  - type: create_issue
    labels: ["unblocked"]

blockers:
  - id: a
    source: {type: github-issue, repo: org/a, issue: 1}
  - id: b
    source: {type: github-release, repo: org/b, version: ">=v2.0.0"}
```

## Combine with scanning

Config blockers, `scan` annotations and `-spec` are merged into one list for a run. See [How a run works](../explanation/how-a-run-works.md).
