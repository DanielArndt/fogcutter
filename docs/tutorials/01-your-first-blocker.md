# Tutorial: your first blocker

In this tutorial you will install fogcutter, mark a workaround in a small project with an annotation, and ask fogcutter whether the thing you are waiting for has happened yet. It takes about ten minutes and needs only a terminal and an internet connection. You do not need a GitHub token.

By the end you will have:

- a working `fogcutter` binary,
- a source file containing a blocker annotation,
- seen fogcutter list, check, and preview the follow-up for that blocker.

## 1. Install fogcutter

Download a prebuilt binary for your platform from the [releases page](https://github.com/DanielArndt/fogcutter/releases), or build it with Go (the version in [`go.mod`](../../go.mod) or newer):

```bash
go install github.com/DanielArndt/fogcutter/cmd/fogcutter@latest
```

Check that it runs:

```bash
fogcutter version
```

You should see something like:

```text
fogcutter v0.1.0
```

## 2. Create a project with a workaround

Make an empty directory and a Go file that contains a workaround you intend to remove one day:

```bash
mkdir fog-demo && cd fog-demo
cat > main.go <<'EOF'
package main

// BLOCKER(github-release:openfga/openfga>=v1.8.0): adopt native batch check
func main() {}
EOF
```

The comment is the important part. `BLOCKER(...)` holds a *spec* that says what you are waiting for (here: a release of `openfga/openfga` that is at least `v1.8.0`), and the text after the colon says what you will do about it.

## 3. List the blocker

`list` finds blockers but never contacts any upstream service:

```bash
fogcutter list -scan .
```

```text
- code:main.go:3:github-release:openfga/openfga>=v1.8.0 (github-release:openfga/openfga (>=v1.8.0)) [main.go:3]: adopt native batch check
```

fogcutter walked the current directory (`-scan .`), found one annotation on line 3 and gave it an ID built from the file, line and spec.

## 4. Check the blocker

Now ask the upstream whether the blocker is resolved. Because we have not said where follow-up actions should go, add `-dry-run` so nothing is created, and `-repo` so fogcutter knows which repository an issue *would* be opened in:

```bash
fogcutter check -scan . -repo you/fog-demo -dry-run
```

At the time of writing the latest OpenFGA release is newer than v1.8.0, so you will see output like this (versions will differ for you):

```text
Checked 1 blocker(s): 1 resolved

[RESOLVED] code:main.go:3:github-release:openfga/openfga>=v1.8.0 (github-release:openfga/openfga (>=v1.8.0)) @ main.go:3
  -> Release [openfga/openfga v1.22.1](https://github.com/openfga/openfga/releases/tag/v1.22.1) satisfies >=v1.8.0

Follow-up Actions (dry-run):
  - [DRY-RUN] code:main.go:3:github-release:openfga/openfga>=v1.8.0 (create_issue): would create issue "Blocker resolved: code:main.go:3:github-release:openfga/openfga>=v1.8.0" in you/fog-demo
```

Two things happened:

1. fogcutter asked GitHub for the releases of `openfga/openfga` and compared them to `>=v1.8.0`. The blocker is `RESOLVED`.
2. Because it is resolved, fogcutter worked out what it *would* do. Annotations get a default action, `create_issue`. In dry-run mode it only tells you.

> **What if I forget `-repo`?** The `create_issue` action needs a target repository, taken from `-repo`, the `repository:` config key, or `$GITHUB_REPOSITORY`. Without one the action reports an error and fogcutter exits with status 1.

## 5. See a blocker that is still blocked

Change the spec to a version that does not exist yet. Edit `main.go` so line 3 reads `>=v99.0.0`, then run:

```bash
fogcutter check -scan . -repo you/fog-demo -dry-run
```

```text
Checked 1 blocker(s): 0 resolved

[BLOCKED] code:main.go:3:github-release:openfga/openfga>=v99.0.0 (github-release:openfga/openfga (>=v99.0.0)) @ main.go:3
  -> latest release of openfga/openfga is v1.22.1 (waiting for >=v99.0.0)
```

The status is now `BLOCKED`, the explanation says what fogcutter saw, and there are no follow-up actions. Running this on a schedule means you find out the moment it flips.

## 6. Try an ad-hoc check

You do not need a file to check something. Pass a spec directly with `-spec`:

```bash
fogcutter check -spec 'snap:lxd>=5.0'
```

This asks the Snap Store whether `lxd` on `latest/stable` is at least version 5.0.

## What you learned

- A blocker is "something external I am waiting for" plus "what to do when it happens".
- You can write one as a comment next to the workaround it blocks.
- `list` shows what fogcutter found; `check` evaluates it; `-dry-run` previews follow-up actions safely.

## Next steps

- Continue with [Automate follow-up with GitHub Actions](02-automate-with-github-actions.md).
- Read the [annotation reference](../reference/annotations.md) to see every spec form.
- Learn why runs are safe to repeat in [Idempotency and deduplication](../explanation/idempotency.md).
