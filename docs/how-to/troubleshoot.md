# How to troubleshoot common problems

| Symptom | Cause and fix |
| :--- | :--- |
| `create_issue requires target 'repo' (or top-level repository / $GITHUB_REPOSITORY)` | No target repository. Pass `-repo owner/name`, set `repository:` in config, or export `GITHUB_REPOSITORY`. |
| `error: config file "x.yaml" not found` | A config path you passed explicitly does not exist. (A missing *default* `.github/blockers.yaml` is silently ignored.) |
| `No blockers found.` | No config blockers, `-scan` finds no annotations. Run `fogcutter list -scan .`; check ignore patterns and that the spec parses. |
| An annotation is not found | Its spec did not parse (it is silently skipped), it contains `)`, or its directory/file is ignored or over 2 MB. Compare with the [spec grammar](../reference/annotations.md#spec-grammar). |
| Annotations in docs/tests show up as real blockers | Add them to `scan.ignore`. |
| `github API … returned HTTP 403` or `429` | Rate-limited. Set `GITHUB_TOKEN` or `GH_TOKEN`. |
| `… returned HTTP 404` for a private repo | The token cannot see it. Use a token with read access. |
| `unsupported manifest "x"` | `dependency` source reads only the [listed formats](../reference/manifests.md). |
| `ref_regex "…" did not match <file>` | Test the regex against the file; the file path is relative to the working directory. |
| `<pkg> not found in … (dependency removed upstream?)` | Blocked on purpose; check whether upstream really dropped it, or fix the `package` name. |
| `>=1.2.3` never matches a snap like `1.2.3-abc123` | Suffix is treated as pre-release. See [Version constraints](../reference/version-constraints.md#snap-style-versions). |
| A `command` action keeps re-running | No state file configured. See [Persist state](persist-state.md). |
| Second `comment_issue` on a blocker never runs with a state file | Same-type actions on one blocker share a state key. See [Persist state](persist-state.md#rules). |
| Duplicate issues despite markers | The marker lookup only reads the 100 most recent issues (all states). In very busy repos use a state file too. |
| `rendering … template` error | Template syntax error. See [Customize templates](customize-templates.md). |
| Comments lost after `fogcutter add` | `add` rewrites the file. Use `-print` and paste. |
| Exit status 1 although nothing is resolved | A source or action errored, or the state file could not be saved. Read the `[ERROR]` lines. |

## Get more information

- `fogcutter list …` shows what was discovered and IDs, without network access (except `-scan-issues`).
- `-format json` shows every field, including `error` and `checked_at`.
- `-dry-run` lets you test actions safely.
