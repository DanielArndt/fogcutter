# Design of the dependency source

The `dependency` source answers a narrow question: *does the upstream release I build already contain a safe version of this package?* This page explains the problem it solves and the decisions that shape its behaviour.

## The problem: overrides that outlive their purpose

When a transitive dependency has a vulnerability, packagers often cannot wait for upstream. They patch the build:

```text
go get golang.org/x/net@v0.62.0   # GO-2026-6860
```

That is correct today. But later upstream publishes a release that bumps `golang.org/x/net` itself, and the override becomes noise that can even *downgrade* things or hide real changes. "The new release probably bumped it" is a belief, not a fact. The `dependency` source replaces belief with a check: read upstream's lockfile at the version you build, and compare.

## Decision 1: read the manifest at a ref, not the release notes

Release notes are prose; lockfiles are data. Reading `go.mod` (or `uv.lock`, `Cargo.lock`, …) at a specific tag answers exactly what that release ships, and is independent of how well upstream documents its changes.

## Decision 2: follow the version you actually build

A check against the *latest* release says "a future upgrade will fix this", but your override is only removable if the version you currently build fixes it. So the ref can be read from a local file (`ref_file` + `ref_regex`), or, for annotations in craft files, from the enclosing part's `source-tag`. When Renovate bumps `source-tag`, the check follows automatically.

Both views are useful, and you choose by what you write:

| Check against | Question answered | Typical use |
| :--- | :--- | :--- |
| The tag you build | "Can I delete the override now?" | the upgrade pull request |
| `latest` | "Will my next upgrade make it removable?" | a scheduled job |

## Decision 3: `latest` means highest, not newest

Projects that maintain several release series publish backports after newer majors, for example `2026.5.9` shipped after `2026.8.0`. If "latest" meant "most recently published", the answer would flip between series depending on release timing. fogcutter instead picks the *highest* version among non-prerelease releases, stripping non-numeric tag prefixes such as `version/` so that tags order correctly.

## Decision 4: every copy must satisfy the constraint

Lockfiles may contain several copies of one package (different major versions, nested `node_modules`). The override is only redundant if none of them is vulnerable, so one failing copy keeps the blocker blocked.

## Decision 5: a missing package is not a success

If upstream dropped the dependency entirely, the override is probably unnecessary, but it might also mean the package moved, was renamed, or the parser looked in the wrong file. Reporting "resolved" would silently delete protection; reporting "blocked, dependency removed upstream?" asks a human to look. A false negative here costs one review; a false positive could cost a vulnerability.

## Decision 6: pin versions in overrides

`go get -u mod` or an unpinned `pkg` has no version to compare. Annotations therefore state a floor (`>=v0.62.0`) that mirrors the pin, which makes the override self-documenting and the check meaningful. A floating upgrade would also pull in unrelated changes, which defeats the purpose of a targeted CVE fix.

## Related

- How-to: [Retire a local CVE override](../how-to/retire-a-cve-override.md)
- Reference: [Sources: dependency](../reference/sources.md#dependency), [Supported manifests](../reference/manifests.md)
