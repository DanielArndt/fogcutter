# How to use fogcutter as a CI gate

Fail a pipeline when a blocker has resolved, forcing someone to remove the workaround.

## Basic gate

```bash
fogcutter check -scan . -fail-on-resolved -dry-run
```

- `-fail-on-resolved` makes the exit code `1` if any blocker is resolved.
- `-dry-run` prevents follow-up actions, so the gate only reports.

Exit status is also `1` if any source or action had an error, so a gate can fail for broken checks as well; look at the output to tell the two apart (see [exit codes](../reference/cli.md#exit-codes)).

## In GitHub Actions

```yaml
- uses: actions/checkout@v4
- name: Fail if a workaround can be removed
  run: fogcutter check -scan . -fail-on-resolved -dry-run -format json > fogcutter.json
  env:
    GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

Setting `GITHUB_TOKEN` avoids rate limits. Install the binary first, for example from a release archive (verify it against `checksums.txt`) or with `go install`.

## Only gate on some blockers

Run two invocations: one gate with a limited config, one scheduled job with actions.

```bash
fogcutter check -config .github/gate.yaml -fail-on-resolved -dry-run
```

## Machine-readable results

Use `-format json` and process `results[].resolved` with `jq`:

```bash
fogcutter check -scan . -dry-run -format json | jq -r '.results[] | select(.resolved) | .blocker.id'
```

Inside GitHub Actions, `$GITHUB_OUTPUT` also receives `has_resolved` and `resolved_json`; see [Output formats](../reference/output.md).
