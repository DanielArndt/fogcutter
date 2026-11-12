# Why fogcutter exists

## The problem

Software projects constantly carry workarounds for things they do not control:

- a pinned dependency override because upstream has not shipped the CVE fix,
- a retry loop for a bug in a service another team owns,
- an old API client because a feature is not in a released version yet,
- a rebuild that waits on a snap channel to catch up.

Each workaround is added with a comment like "remove when upstream fixes this". The comment is then forgotten. The upstream fix lands, but nobody notices, and the workaround stays for years. Nothing in a normal toolchain says "the thing you were waiting for has happened".

## The idea

fogcutter turns that comment into something executable. A blocker has two halves:

1. **A condition** that can be checked mechanically: an issue is closed, a release exists, a snap channel carries a version, a lockfile pins a version, a URL answers a certain way, a command succeeds.
2. **A response**: open an issue, comment on the blocked ticket, trigger a workflow, or run a command (for instance one that starts a coding agent).

The tool checks the condition regularly and, when it flips, performs the response exactly once.

## Principles

**Declarative and close to the code.** The best place to record "waiting for X" is next to the workaround. Inline annotations let the blocker live and move with the code, while a config file serves the cases that need more structure.

**Narrow, comparable conditions.** Each source yields a plain yes/no plus an explanation of why not yet. fogcutter deliberately avoids a general expression language; the sources cover the situations that actually come up, and `http` / `command` are escape hatches.

**Safe to run on a schedule.** Runs must be repeatable. A blocker that stays resolved should not generate new issues each morning. See [Idempotency and deduplication](idempotency.md).

**Act, do not just report.** A report that nobody reads has the same failure mode as the forgotten comment. Actions put the nudge where work happens: issue trackers, workflows, agents.

**Preview everything.** `list` and `-dry-run` let you see exactly what would happen. Nothing destructive is ever done: fogcutter creates issues and comments, but it does not edit your code, merge anything, or close tickets.

**One small binary.** No daemon, database or service. State is the target system (GitHub markers) plus, optionally, one JSON file.

## What fogcutter is not

- It is not a dependency updater. Renovate and Dependabot bump versions; fogcutter tells you which *overrides* a bump makes redundant.
- It is not a monitoring system. It checks occasionally, not continuously.
- It is not an issue tracker integration framework. It speaks GitHub, the Snap Store and plain HTTP.

## Related reading

- [How a run works](how-a-run-works.md)
- [Design of the dependency source](dependency-source.md)
