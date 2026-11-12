# How to launch a coding agent when a blocker resolves

The `command` action runs a shell command with context about the blocker, which is enough to start a coding agent or any script.

## Example

```yaml
blockers:
  - id: openfga-batch-check
    description: "Adopt OpenFGA native batch check"
    source:
      type: github-release
      repo: openfga/openfga
      version: ">=v1.8.0"
    on_resolved:
      - type: command
        command: |
          claude -p "Blocker {{.ID}} resolved ({{.Resolution}}). Upgrade go.mod to openfga >= v1.8.0 and refactor internal/integration/openfga to use batch check. Open a PR."
```

## Pass data through the environment

The command also receives these variables, which are safer than templating values into a shell string (no quoting problems):

| Variable | Value |
| :--- | :--- |
| `FOGCUTTER_ID` | blocker ID |
| `FOGCUTTER_DESCRIPTION` | description |
| `FOGCUTTER_LOCATION` | `file:line` or `owner/repo#n`, may be empty |
| `FOGCUTTER_RESOLUTION` | human-readable result |
| `FOGCUTTER_SOURCE` | source summary |

```yaml
- type: command
  command: ./scripts/unblocked.sh
  env:
    TARGET_BRANCH: "unblock/{{.ID}}"   # env values are templates too
```

## Make it run once

A `command` action has no way to look for a marker on GitHub, so **without a state file it runs on every invocation while the blocker stays resolved**. Always configure one for commands:

```bash
fogcutter check -state-file .fogcutter-state.json
```

and persist that file between runs (see [Persist state between runs](persist-state.md)). Add `*.state.json` to `.gitignore` if you name it that way.

## Safety notes

- Standard output and error are captured and reported as the action `detail`; a non-zero exit is reported as `error` and makes fogcutter exit with status 1.
- The command inherits fogcutter's environment, including `GITHUB_TOKEN`.
- Test with `-dry-run`, which prints the rendered command instead of running it.
- Never run untrusted configuration: `command` actions and sources execute arbitrary shell.
