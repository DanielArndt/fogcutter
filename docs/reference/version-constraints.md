# Version constraints

Used by the `version` and `revision` fields of sources, and in the specs of annotations and `-spec`.

## Syntax

```text
<operator><version>
```

| Operator | Satisfied when actual … constraint |
| :--- | :--- |
| `>=` | is greater than or equal to |
| `<=` | is less than or equal to |
| `>` | is greater than |
| `<` | is less than |
| `==` or `=` | equals |
| `!=` | differs from |
| *(none)* | same as `>=` |

So `v1.8.0` means `>=v1.8.0`: "at least this version has been released". An empty constraint or `*` is satisfied by any non-empty actual value. Whitespace around the operator is ignored. An operator with no version is an error.

## Comparison rules

Both sides are normalised, then compared token by token:

1. A leading `v` or `V` is removed. Build metadata after `+` is removed.
2. The text after the first `-` or `~` is the **pre-release** part.
3. The main part is split into tokens at `.`, `_`, `-`, and wherever digits meet letters (`rc1` → `rc`, `1`).
4. Tokens compare numerically if both are numbers, otherwise as strings. Missing tokens count as `0`, so `1.2` equals `1.2.0`.
5. If the main parts are equal, a version without a pre-release is greater than one with (`1.0.0` > `1.0.0-rc.1`). Two pre-releases are compared token by token; the shorter is lower.

| Actual | Constraint | Result |
| :--- | :--- | :--- |
| `v1.10.0` | `>=1.9.0` | satisfied |
| `1.2` | `==1.2.0` | satisfied |
| `v1.2.3+build5` | `==1.2.3` | satisfied |
| `1.0.0-rc.1` | `>=1.0.0` | not satisfied |
| `2.0.0` | `>1.0.0-alpha.1` | satisfied |

## Snap-style versions

Snap versions often look like `5.21.2-20240812` or `6.10.1-f5c750d`. The part after `-` is treated as a pre-release, so `6.10.1-f5c750d` is **lower** than `6.10.1`. Consequences:

- `>=6.10.1` does **not** match `6.10.1-f5c750d`.
- `>=6.10.1-f5c750d` does match it, and `>6.10.0` does too.

Prefer a constraint on the revision (`revision: ">=41000"`) or a lower bound that avoids the suffix.

## PEP 440 versions

Python's `1.0.0rc1` (no hyphen) is not recognised as a pre-release: the `rc` token compares as greater than `0`, so `1.0.0rc1` is treated as *newer* than `1.0.0`. Likewise `1.0.0.post1`. Plain release numbers (`2.31.0`) work as expected.

## Tags

For `github-release` and `dependency` `latest`, a tag like `version/2026.5.8` has its non-numeric prefix stripped (`2026.5.8`) before ordering.
