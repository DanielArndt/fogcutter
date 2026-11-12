# How to wait for a snap to reach a version

Use the `snapcraft` source to watch a channel of the Snap Store.

## From the command line

```bash
fogcutter check -spec 'snap:go/1.26/stable>=1.26.9'
```

Here `go` is the snap, `1.26/stable` the channel, and `>=1.26.9` the version constraint.

## As an annotation

```yaml
# BLOCKER(snap:lxd:5.21/stable>=5.21.2): upgrade LXD in the test image
```

The two spec spellings are equivalent: `snap:<name>:<channel>>=…` and `snap:<name>/<channel>>=…`. With no channel the default is `latest/stable`: `snap:lxd>=5.21`.

## In config

```yaml
blockers:
  - id: login-ui-983-go-1.26.9
    description: "Rebuild the rock once Go 1.26.9 is on go/1.26/stable"
    source:
      type: snapcraft
      snap: go/1.26/stable     # or: snap: go + channel: 1.26/stable
      version: ">=1.26.9"
    on_resolved:
      - type: comment_issue
        repo: canonical/identity-platform-login-ui
        issue: 983
        labels: ["unblocked"]
        remove_labels: ["blocked"]
        body: "go/1.26/stable is now {{.Resolution}}"
```

## Other constraints

Constrain by revision instead of (or as well as) version:

```yaml
source:
  type: snapcraft
  snap: postgresql
  channel: 16/stable
  revision: ">=29000"
```

Pick another architecture with `architecture: arm64` (default `amd64`).

## Gotcha: snap versions with a suffix

A version such as `6.10.1-f5c750d` is compared as the pre-release `f5c750d` of `6.10.1`, so it is *lower* than `6.10.1`, and `>=6.10.1` will not match it. Constrain on a lower patch level (`>6.10.0`) or use `revision`. See [Version constraints](../reference/version-constraints.md#snap-style-versions).
