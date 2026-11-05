// SPDX-FileCopyrightText: 2026 Daniel Arndt <dan@arndt.ca>
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/DanielArndt/fogcutter/internal/action"
	"github.com/DanielArndt/fogcutter/internal/checker"
	"github.com/DanielArndt/fogcutter/internal/config"
	"github.com/DanielArndt/fogcutter/internal/scanner"
	"gopkg.in/yaml.v3"
)

const versionStr = "0.1.0" // x-release-please-version

// Report is the structured output of `fogcutter check`.
type Report struct {
	TotalBlockers int                      `json:"total_blockers"`
	ResolvedCount int                      `json:"resolved_count"`
	Results       []checker.Result         `json:"results"`
	Actions       []action.ExecutionRecord `json:"actions,omitempty"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	subcmd := "check"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		subcmd = args[0]
		args = args[1:]
	}

	switch subcmd {
	case "version", "--version", "-v":
		fmt.Fprintf(stdout, "fogcutter v%s\n", versionStr)
		return 0
	case "help", "--help", "-h":
		printUsage(stdout)
		return 0
	case "check", "list":
		return runCheckOrList(subcmd, args, stdout, stderr)
	case "add":
		return runAdd(args, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n", subcmd)
		printUsage(stderr)
		return 2
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintf(w, `fogcutter - Monitor external blockers and trigger follow-up actions when resolved

Usage:
  fogcutter [check|list|add|version] [flags]

Commands:
  check     Evaluate blockers from config and/or scanned files/issues and trigger follow-up actions (default)
  list      List all discovered blockers without querying upstream APIs
  add       Add a blocker entry to .github/blockers.yaml (or print YAML with -print)
  version   Print version

Flags (check / list):
  -config string        Path to blockers YAML config (default ".github/blockers.yaml")
  -spec string          Ad-hoc blocker spec to check (e.g. "snap:go/1.26/stable>=1.26.9" or "owner/repo#123")
  -scan string          Comma-separated paths to scan for BLOCKER(...) or TODO(blocked-by:...) comments
  -scan-issues          Scan open GitHub issues in target repo for BLOCKER(...) or "Blocked by/on ..." lines
  -repo string          Target GitHub repository (owner/name); defaults to config repository or $GITHUB_REPOSITORY
  -state-file string    Optional JSON file path to persist triggered blocker IDs across runs
  -dry-run              Evaluate blockers and preview follow-up actions without executing them
  -format string        Output format: text, json, or github (default "text")
  -fail-on-resolved     Exit with code 1 if any blocker is resolved (useful for CI gates)
`)
}

func runAdd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	fs.SetOutput(stderr)

	configPath := fs.String("config", ".github/blockers.yaml", "Path to blockers YAML config")
	id := fs.String("id", "", "Blocker ID (auto-derived if omitted)")
	spec := fs.String("spec", "", "Blocker spec (e.g. 'snap:go/1.26/stable>=1.26.9' or 'owner/repo#123') [required]")
	desc := fs.String("desc", "", "Blocker description")
	targetIssue := fs.String("issue", "", "Blocked GitHub issue to comment on when resolved (e.g. '983' or 'owner/repo#983')")
	commentBody := fs.String("body", "", "Custom comment or issue body template")
	workflow := fs.String("workflow", "", "GitHub Actions workflow to dispatch when resolved")
	command := fs.String("command", "", "Shell command to run when resolved")
	printOnly := fs.Bool("print", false, "Print YAML entry to stdout without modifying the config file")

	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *spec == "" {
		fmt.Fprintln(stderr, "error: -spec is required (e.g. -spec 'snap:go/1.26/stable>=1.26.9')")
		return 2
	}

	src, err := scanner.ParseSpec(*spec)
	if err != nil {
		fmt.Fprintf(stderr, "error parsing -spec: %v\n", err)
		return 1
	}

	var actions []config.ActionConfig
	if *targetIssue != "" {
		act := config.ActionConfig{
			Type: "comment_issue",
			Body: *commentBody,
		}
		rawIss := strings.TrimSpace(*targetIssue)
		if strings.HasPrefix(rawIss, "https://github.com/") {
			if parsed, err := scanner.ParseSpec(rawIss); err == nil && parsed.Type == "github-issue" {
				act.Repo = parsed.Repo
				act.Issue = parsed.Issue
			}
		} else if parts := strings.SplitN(rawIss, "#", 2); len(parts) == 2 {
			act.Repo = parts[0]
			act.Issue, _ = strconv.Atoi(parts[1])
		} else {
			act.Issue, _ = strconv.Atoi(strings.TrimPrefix(rawIss, "#"))
		}
		if act.Issue <= 0 {
			fmt.Fprintf(stderr, "error: invalid -issue %q\n", *targetIssue)
			return 1
		}
		actions = append(actions, act)
	}
	if *workflow != "" {
		actions = append(actions, config.ActionConfig{
			Type:     "workflow_dispatch",
			Workflow: *workflow,
			Ref:      "main",
		})
	}
	if *command != "" {
		actions = append(actions, config.ActionConfig{
			Type:    "command",
			Command: *command,
		})
	}
	if len(actions) == 0 {
		actions = append(actions, config.ActionConfig{
			Type:  "create_issue",
			Title: "Unblocked: {{.SourceSummary}}",
			Body:  *commentBody,
		})
	}

	b := config.Blocker{
		ID:          *id,
		Description: *desc,
		Source:      src,
		OnResolved:  actions,
	}
	if b.ID == "" {
		b.ID = strings.NewReplacer(":", "-", "/", "-", ">=", "-", "<=", "-", ">", "-", "<", "-", "=", "-", "#", "-").Replace(*spec)
	}

	if *printOnly {
		out, err := yaml.Marshal([]config.Blocker{b})
		if err != nil {
			fmt.Fprintf(stderr, "error marshaling YAML: %v\n", err)
			return 1
		}
		fmt.Fprint(stdout, string(out))
		return 0
	}

	var cfg config.Config
	if _, err := os.Stat(*configPath); err == nil {
		loaded, err := config.Load(*configPath)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		cfg = *loaded
	}
	cfg.Blockers = append(cfg.Blockers, b)
	out, err := yaml.Marshal(&cfg)
	if err != nil {
		fmt.Fprintf(stderr, "error marshaling config: %v\n", err)
		return 1
	}
	if dir := filepath.Dir(*configPath); dir != "." && dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}
	if err := os.WriteFile(*configPath, out, 0o644); err != nil {
		fmt.Fprintf(stderr, "error writing %s: %v\n", *configPath, err)
		return 1
	}
	fmt.Fprintf(stdout, "Added blocker %q to %s\n", b.ID, *configPath)
	return 0
}

func runCheckOrList(mode string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet(mode, flag.ContinueOnError)
	fs.SetOutput(stderr)

	configPath := fs.String("config", ".github/blockers.yaml", "Path to blockers YAML config")
	specFlag := fs.String("spec", "", "Ad-hoc blocker spec to evaluate (e.g. 'snap:go/1.26/stable>=1.26.9')")
	scanPaths := fs.String("scan", "", "Comma-separated paths to scan for inline blocker annotations")
	scanIssues := fs.Bool("scan-issues", false, "Scan open GitHub issues in target repo for blocker annotations")
	repoFlag := fs.String("repo", "", "Target GitHub repository (owner/name)")
	stateFileFlag := fs.String("state-file", "", "Path to JSON state file")
	dryRun := fs.Bool("dry-run", false, "Evaluate blockers without executing follow-up actions")
	format := fs.String("format", "text", "Output format: text, json, or github")
	failOnResolved := fs.Bool("fail-on-resolved", false, "Exit with code 1 if any blocker is resolved")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	var cfg *config.Config
	if _, err := os.Stat(*configPath); err == nil {
		loaded, err := config.Load(*configPath)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		cfg = loaded
	} else if *configPath != ".github/blockers.yaml" {
		fmt.Fprintf(stderr, "error: config file %q not found\n", *configPath)
		return 1
	} else {
		cfg = &config.Config{
			Repository: os.Getenv("GITHUB_REPOSITORY"),
		}
	}

	if *repoFlag != "" {
		cfg.Repository = *repoFlag
	}
	if *stateFileFlag != "" {
		cfg.StateFile = *stateFileFlag
	}

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		token = os.Getenv("GH_TOKEN")
	}

	ctx := context.Background()
	eval := checker.NewEvaluator(token)

	// Collect all blockers from config + optional -spec + optional file scan + optional GitHub issue scan
	allBlockers := append([]config.Blocker(nil), cfg.Blockers...)
	if *specFlag != "" {
		src, err := scanner.ParseSpec(*specFlag)
		if err != nil {
			fmt.Fprintf(stderr, "error parsing -spec: %v\n", err)
			return 1
		}
		allBlockers = append(allBlockers, config.Blocker{
			ID:     *specFlag,
			Source: src,
		})
	}

	var pathsToScan []string
	var ignorePatterns []string
	if cfg.Scan != nil {
		pathsToScan = append(pathsToScan, cfg.Scan.Paths...)
		ignorePatterns = append(ignorePatterns, cfg.Scan.Ignore...)
	}
	if *scanPaths != "" {
		for _, p := range strings.Split(*scanPaths, ",") {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				pathsToScan = append(pathsToScan, trimmed)
			}
		}
	}

	if len(pathsToScan) > 0 {
		scanned, err := scanner.ScanFiles(pathsToScan, ignorePatterns, cfg.DefaultActions)
		if err != nil {
			fmt.Fprintf(stderr, "error scanning files: %v\n", err)
			return 1
		}
		allBlockers = append(allBlockers, scanned...)
	}

	shouldScanIssues := *scanIssues || (cfg.Scan != nil && cfg.Scan.Issues)
	if shouldScanIssues {
		issueLabel := ""
		if cfg.Scan != nil {
			issueLabel = cfg.Scan.IssueLabel
		}
		issueBlockers, err := scanner.ScanGitHubIssues(ctx, eval.HTTPClient, eval.GitHubAPIBase, token, cfg.Repository, issueLabel)
		if err != nil {
			fmt.Fprintf(stderr, "error scanning GitHub issues: %v\n", err)
			return 1
		}
		allBlockers = append(allBlockers, issueBlockers...)
	}

	if mode == "list" {
		if *format == "json" {
			enc := json.NewEncoder(stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(allBlockers)
			return 0
		}
		if len(allBlockers) == 0 {
			fmt.Fprintln(stdout, "No blockers configured or discovered.")
			return 0
		}
		for _, b := range allBlockers {
			loc := ""
			if b.Location != "" {
				loc = fmt.Sprintf(" [%s]", b.Location)
			}
			fmt.Fprintf(stdout, "- %s (%s)%s: %s\n", b.ID, checker.SummarizeSource(b.Source), loc, b.Description)
		}
		return 0
	}

	stateStore, err := action.LoadStateStore(cfg.StateFile)
	if err != nil {
		fmt.Fprintf(stderr, "error loading state file: %v\n", err)
		return 1
	}

	runner := action.NewRunner(token, cfg.Repository, *dryRun, stateStore)

	report := Report{
		TotalBlockers: len(allBlockers),
		Results:       make([]checker.Result, 0, len(allBlockers)),
	}

	hasErrors := false
	for _, b := range allBlockers {
		res := eval.Check(ctx, b)
		report.Results = append(report.Results, res)
		if res.Error != "" {
			hasErrors = true
			continue
		}
		if res.Resolved {
			report.ResolvedCount++
			recs := runner.ExecuteAll(ctx, res)
			report.Actions = append(report.Actions, recs...)
			for _, rec := range recs {
				if rec.Status == "error" {
					hasErrors = true
				}
			}
		}
	}

	if !*dryRun {
		if err := stateStore.Save(); err != nil {
			fmt.Fprintf(stderr, "error saving state file: %v\n", err)
			hasErrors = true
		}
	}

	// Always populate $GITHUB_OUTPUT and $GITHUB_STEP_SUMMARY when running inside GitHub Actions
	if os.Getenv("GITHUB_OUTPUT") != "" || *format == "github" {
		writeGitHubOutputs(report)
	}

	switch *format {
	case "json":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(report)
	default:
		printTextReport(stdout, report, *dryRun)
	}

	if hasErrors {
		return 1
	}
	if *failOnResolved && report.ResolvedCount > 0 {
		return 1
	}
	return 0
}

func printTextReport(w io.Writer, report Report, dryRun bool) {
	if report.TotalBlockers == 0 {
		fmt.Fprintln(w, "No blockers found.")
		return
	}
	fmt.Fprintf(w, "Checked %d blocker(s): %d resolved\n\n", report.TotalBlockers, report.ResolvedCount)
	for _, r := range report.Results {
		status := "BLOCKED"
		if r.Error != "" {
			status = "ERROR"
		} else if r.Resolved {
			status = "RESOLVED"
		}
		loc := ""
		if r.Blocker.Location != "" {
			loc = fmt.Sprintf(" @ %s", r.Blocker.Location)
		}
		detail := r.Resolution
		if r.Error != "" {
			detail = r.Error
		}
		fmt.Fprintf(w, "[%s] %s (%s)%s\n  -> %s\n", status, r.Blocker.ID, r.SourceSummary, loc, detail)
	}

	if len(report.Actions) > 0 {
		header := "\nFollow-up Actions:"
		if dryRun {
			header = "\nFollow-up Actions (dry-run):"
		}
		fmt.Fprintln(w, header)
		for _, a := range report.Actions {
			msg := a.Detail
			if a.Error != "" {
				msg = a.Error
			}
			fmt.Fprintf(w, "  - [%s] %s (%s): %s\n", strings.ToUpper(a.Status), a.BlockerID, a.Type, msg)
		}
	}
}

func writeGitHubOutputs(report Report) {
	if outPath := os.Getenv("GITHUB_OUTPUT"); outPath != "" {
		if f, err := os.OpenFile(outPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644); err == nil {
			defer f.Close()
			fmt.Fprintf(f, "total_blockers=%d\n", report.TotalBlockers)
			fmt.Fprintf(f, "resolved_count=%d\n", report.ResolvedCount)
			fmt.Fprintf(f, "has_resolved=%t\n", report.ResolvedCount > 0)
			var resolvedOnly []checker.Result
			for _, r := range report.Results {
				if r.Resolved {
					resolvedOnly = append(resolvedOnly, r)
				}
			}
			if b, err := json.Marshal(resolvedOnly); err == nil {
				fmt.Fprintf(f, "resolved_json=%s\n", string(b))
			}
		}
	}

	if summaryPath := os.Getenv("GITHUB_STEP_SUMMARY"); summaryPath != "" {
		if f, err := os.OpenFile(summaryPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644); err == nil {
			defer f.Close()
			fmt.Fprintf(f, "## Fogcutter Report (%d/%d resolved)\n\n", report.ResolvedCount, report.TotalBlockers)
			fmt.Fprintln(f, "| Status | Blocker ID | Source | Details |")
			fmt.Fprintln(f, "| :--- | :--- | :--- | :--- |")
			for _, r := range report.Results {
				status := "⏳ Blocked"
				detail := r.Resolution
				if r.Error != "" {
					status = "❌ Error"
					detail = r.Error
				} else if r.Resolved {
					status = "✅ Resolved"
				}
				fmt.Fprintf(f, "| %s | `%s` | `%s` | %s |\n", status, r.Blocker.ID, r.SourceSummary, detail)
			}
			if len(report.Actions) > 0 {
				fmt.Fprintln(f, "\n### Follow-up Actions")
				for _, a := range report.Actions {
					msg := a.Detail
					if a.Error != "" {
						msg = a.Error
					}
					fmt.Fprintf(f, "- **`%s` (`%s`)**: %s — %s\n", a.BlockerID, a.Type, a.Status, msg)
				}
			}
		}
	}
}
