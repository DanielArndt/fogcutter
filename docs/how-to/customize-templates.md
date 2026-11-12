# How to customize issue and comment templates

Titles, bodies, workflow inputs, commands and command environment values are [Go `text/template`](https://pkg.go.dev/text/template) strings.

## Use the built-in variables

```yaml
on_resolved:
  - type: create_issue
    title: "Drop workaround: {{.Description}}"
    body: |
      **{{.ID}}** is resolved.

      - Waiting on: `{{.SourceSummary}}`
      - Result: {{.Resolution}}
      - Where: `{{.Location}}`
```

The full list is in the [templates reference](../reference/templates.md).

## Make parts conditional

Variables may be empty (for instance `Location` for declared blockers). Use `if`:

```yaml
body: |
  {{.Resolution}}
  {{if .Location}}Location: `{{.Location}}`{{end}}
  {{if .Description}}Context: {{.Description}}{{end}}
```

## Reach into the source

`.Source` is the whole source block, so you can use its fields:

```yaml
title: "{{.Source.Repo}} reached {{.Source.Version}}"
```

## Use templates in workflow inputs

```yaml
- type: workflow_dispatch
  workflow: rebuild.yml
  ref: main
  inputs:
    reason: "{{.ID}}: {{.Resolution}}"
```

## Watch out for

- A template syntax error makes that action fail with `rendering … template` and exit status 1. Preview with `-dry-run` first; dry-run renders titles and inputs.
- `Resolution` is Markdown that already contains links for issue and release sources, which suits issue bodies. For shell commands prefer the plain [`FOGCUTTER_*` environment variables](../reference/actions.md#command).
- YAML block scalars (`|`) keep newlines; plain scalars containing `{{` must be quoted.
- Config values are **not** expanded for environment variables. Only templates are evaluated, and only in action fields.
