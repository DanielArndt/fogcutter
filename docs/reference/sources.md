# Sources reference

A source describes how fogcutter decides a blocker is resolved. It is the `source:` object of a blocker, selected by `type`.

| `type` | Resolved when |
| :--- | :--- |
| [`github-issue`](#github-issue) | an issue or PR is closed |
| [`github-release`](#github-release) | a release or tag satisfies a version |
| [`snapcraft`](#snapcraft) | a snap channel satisfies a version and/or revision |
| [`http`](#http) | an HTTP response matches |
| [`command`](#command) | a command succeeds (and optionally prints a matching value) |
| [`dependency`](#dependency) | an upstream manifest pins a package at a satisfying version |

A source reports one of three outcomes: **resolved**, **blocked** (with an explanation), or **error** (the check itself failed, for example HTTP 5xx, invalid regex, missing field).

Accepted constraint syntax for every `version` field is in [Version constraints](version-constraints.md).

## github-issue

| Field | Required | Description |
| :--- | :--- | :--- |
| `repo` | yes | `owner/name` |
| `issue` | yes | Issue or PR number (positive integer) |
| `reason` | no | Required close reason. |

Calls `GET /repos/{repo}/issues/{issue}`. Resolved when `state` is `closed`.

`reason` behaviour:

- For an **issue**, `state_reason` must equal `reason` case-insensitively (`completed`, `not_planned`, `reopened`).
- For a **PR**, `merged` and `completed` both require the PR to have been merged; a PR closed without merging is blocked. Any other value is compared with `state_reason`.

Resolution text is a Markdown link: `[owner/repo#123: title](url) is closed (completed)`.

## github-release

| Field | Required | Description |
| :--- | :--- | :--- |
| `repo` | yes | `owner/name` |
| `version` | no | Constraint on the release tag, e.g. `>=v1.8.0`. Empty means any release. |
| `include_prereleases` | no | Consider prereleases (default `false`). Drafts are always ignored. |

Reads the 30 most recent releases and resolves if **any** satisfies the constraint. If the repository has *no* releases, it falls back to the 30 most recent git tags. Tags are compared as versions with the `v` prefix tolerated.

## snapcraft

| Field | Required | Description |
| :--- | :--- | :--- |
| `snap` | yes | Snap name. `name/track/risk` shorthand is split when `channel` is empty. |
| `channel` | one of | Channel, default `latest/stable`. |
| `architecture` | no | Default `amd64`. |
| `version` | one of | Constraint on the channel's version string. |
| `revision` | one of | Constraint on the revision number, e.g. `>=29000`. |

At least one of `version`, `revision`, `channel` is required. Queries `https://api.snapcraft.io/v2/snaps/info/<snap>`. A channel matches by full name (`latest/stable`), by `track/risk` (`1.26/stable`), or, for the `latest` track, by bare risk (`stable`). If both `version` and `revision` are given, both must hold. If the channel does not exist for that architecture the blocker is blocked with `channel … not found`.

## http

| Field | Required | Description |
| :--- | :--- | :--- |
| `url` | yes | Request URL. |
| `method` | no | Default `GET`. |
| `headers` | no | Map of header name to literal value. |
| `status_code` | no | Expected status, default `200`. A different status is *blocked*, not an error. |
| `json_path` | no | Dot path into a JSON response (`a.b`, array index as number: `items.0.id`). A leading `.` is allowed. |
| `equals` | no | Exact string match of the value. |
| `regex` | no | Go (RE2) regular expression matched against the value. |
| `version` | no | Version constraint applied to the value. |

The *value* is the JSON field at `json_path` (formatted with `%v`), or the whole trimmed body when `json_path` is empty. Bodies are limited to 2 MiB. Only one of `equals`, `regex`, `version` applies, with precedence in that order. With none, a matching status is enough. A non-JSON body with `json_path` is an error; a missing path is *blocked*.

## command

| Field | Required | Description |
| :--- | :--- | :--- |
| `command` | yes | Run with `sh -c`. |
| `equals`, `regex`, `version` | no | Test on trimmed stdout, same precedence as `http`. |

A non-zero exit is **blocked** (the message includes stderr). Exit `0` is resolved when no matcher is configured, otherwise stdout must match.

## dependency

| Field | Required | Description |
| :--- | :--- | :--- |
| `repo` | yes | Upstream `owner/name`. |
| `manifest` | yes | Path to the manifest in `repo`. Its base name selects the [format](manifests.md). |
| `package` | yes | Package/module name. |
| `version` | yes | Constraint every copy of the package must satisfy. |
| `ref` | no | Tag, branch or SHA; `latest` (default) = highest-versioned non-prerelease release. |
| `ref_file` / `ref_regex` | together | Read the ref from a local file. First capture group, or whole match. Takes precedence over `ref`. |
| `include_prereleases` | no | Allow prereleases when resolving `latest`. |

The manifest is fetched through the GitHub contents API (up to 64 MiB). Outcomes:

- **resolved**: the package is present and every recorded version satisfies `version`.
- **blocked**: some version does not satisfy it, or the package is absent (`dependency removed upstream?`).
- **error**: file not found at that ref, unsupported manifest, bad regex, no releases.

`latest` picks the *highest* version, not the most recently published, after stripping a non-numeric tag prefix such as `version/`. See [Design of the dependency source](../explanation/dependency-source.md).

Short form (annotations, `-spec`): `dep:owner/repo[@ref]:manifest:package>=version`.
