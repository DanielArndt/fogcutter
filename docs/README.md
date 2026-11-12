# fogcutter documentation

`fogcutter` is a single-binary CLI that checks whether external blockers are resolved (an upstream issue closed, a snap published, a release tagged, a dependency bumped) and triggers follow-up actions when they are.

This documentation follows the [Diátaxis](https://diataxis.fr/) framework. Pick the section that matches what you are trying to do:

|  | **Learning** (study) | **Working** (apply) |
| :--- | :--- | :--- |
| **Practical** (doing) | [**Tutorials**](#tutorials): guided lessons | [**How-to guides**](#how-to-guides): recipes for specific goals |
| **Theoretical** (knowing) | [**Explanation**](#explanation): background and design | [**Reference**](#reference): precise technical description |

## Tutorials

Start here if you are new. Each lesson is a hands-on walk-through that ends with something working.

1. [Your first blocker](tutorials/01-your-first-blocker.md): annotate code, list and check the blocker from the command line.
2. [Automate follow-up with GitHub Actions](tutorials/02-automate-with-github-actions.md): run fogcutter on a schedule and have it open an issue when a blocker clears.

## How-to guides

Task-oriented recipes. They assume you know the basics.

- [Annotate source code with blockers](how-to/annotate-source-code.md)
- [Declare blockers in a config file](how-to/declare-blockers-in-config.md)
- [Wait for a snap to reach a version](how-to/wait-for-a-snap.md)
- [Retire a local CVE override](how-to/retire-a-cve-override.md)
- [Check an HTTP endpoint or a command](how-to/check-http-or-command.md)
- [Notify a blocked issue when it unblocks](how-to/notify-a-blocked-issue.md)
- [Customize issue and comment templates](how-to/customize-templates.md)
- [Launch a coding agent when a blocker resolves](how-to/launch-a-coding-agent.md)
- [Scan GitHub issues for blockers](how-to/scan-github-issues.md)
- [Persist state between runs](how-to/persist-state.md)
- [Use fogcutter as a CI gate](how-to/use-as-a-ci-gate.md)
- [Run fogcutter in GitHub Actions](how-to/run-in-github-actions.md)
- [Troubleshoot common problems](how-to/troubleshoot.md)

## Reference

Dry, complete and accurate descriptions of the machinery.

- [Command line](reference/cli.md): commands, flags, exit codes, environment variables
- [Configuration file](reference/configuration.md): every field of `blockers.yaml`
- [Sources](reference/sources.md): `github-issue`, `github-release`, `snapcraft`, `http`, `command`, `dependency`
- [Actions](reference/actions.md): `create_issue`, `comment_issue`, `workflow_dispatch`, `repository_dispatch`, `command`
- [Inline annotations and specs](reference/annotations.md): comment syntax and the compact spec grammar
- [Templates](reference/templates.md): variables available in action templates
- [Output formats](reference/output.md): text, JSON, `GITHUB_OUTPUT` and the step summary
- [Version constraints](reference/version-constraints.md): operators and comparison rules
- [Supported manifests](reference/manifests.md): lockfile formats for the `dependency` source

## Explanation

Background reading that explains why things work the way they do.

- [Why fogcutter exists](explanation/why-fogcutter.md)
- [How a run works](explanation/how-a-run-works.md)
- [Idempotency and deduplication](explanation/idempotency.md)
- [Design of the dependency source](explanation/dependency-source.md)
- [Limitations and sharp edges](explanation/limitations.md)

## Contributing

Working on fogcutter itself? See [CONTRIBUTIONS.md](../CONTRIBUTIONS.md) and [AGENTS.md](../AGENTS.md).
