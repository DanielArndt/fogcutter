# Supported manifests

The `dependency` source selects a parser from the **base name** of the `manifest` path. Directories are fine (`web/package-lock.json`).

| File | Reads | Notes |
| :--- | :--- | :--- |
| `go.mod` | `require` lines (single and block form) | A `replace` directive that points the module at a specific version wins over `require`. Replacements with local paths are ignored. `// comments` are stripped. `+incompatible` suffixes are kept. |
| `uv.lock`, `poetry.lock`, `pdm.lock` | `[[package]]` tables with `name` and `version` | Python names are compared with PEP 503 normalisation (`PyJWT` = `pyjwt`, `zope.interface` = `zope-interface`). |
| `Cargo.lock` | `[[package]]` tables | Names compared exactly. |
| `package-lock.json`, `npm-shrinkwrap.json` | `packages` map (lockfileVersion 2/3) or nested `dependencies` (v1) | Matches `node_modules/<pkg>` at any depth, so scoped names (`@scope/pkg`) and nested copies are included. |
| `requirements*.txt` | `name==version` or `name===version` lines | Extras (`name[extra]==1`), environment markers and hashes are tolerated. Unpinned lines are ignored. |

Any other file name is an error: `unsupported manifest "<name>"`.

## Multiple copies

All versions of the package found in the file are collected. The blocker resolves only if **every** one satisfies the constraint. The result message lists them: `… pins golang.org/x/net v0.62.0, v0.50.0 (waiting for >=v0.62.0)`.

## Not found

If the package does not appear at all, the blocker stays blocked with `… not found in …`. This guards against treating a dropped dependency as a successful bump.

## Limits

Files are fetched through the GitHub API, up to 64 MiB. There is no support for `go.sum`, `yarn.lock`, `pnpm-lock.yaml`, `Gemfile.lock` or `Pipfile.lock`.
