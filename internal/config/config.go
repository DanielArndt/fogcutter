package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the top-level .github/blockers.yaml configuration.
type Config struct {
	// Repository is the default target GitHub repository (owner/repo) for follow-up actions.
	// Falls back to $GITHUB_REPOSITORY if empty.
	Repository string `yaml:"repository,omitempty" json:"repository,omitempty"`

	// StateFile is an optional path to a JSON file used to record already-triggered blocker IDs.
	StateFile string `yaml:"state_file,omitempty" json:"state_file,omitempty"`

	// Scan configures automatic discovery of blocker annotations in source files and/or GitHub issues.
	Scan *ScanConfig `yaml:"scan,omitempty" json:"scan,omitempty"`

	// DefaultActions are applied to any blocker (including scanned ones) that does not define its own on_resolved actions.
	DefaultActions []ActionConfig `yaml:"default_actions,omitempty" json:"default_actions,omitempty"`

	// Blockers is the list of declaratively configured external blockers.
	Blockers []Blocker `yaml:"blockers,omitempty" json:"blockers,omitempty"`
}

// ScanConfig configures code & issue scanning for inline blocker comments.
type ScanConfig struct {
	// Paths is the list of directory/file paths to scan for inline annotations (e.g. ["."]).
	Paths []string `yaml:"paths,omitempty" json:"paths,omitempty"`

	// Ignore is a list of directory or file glob patterns to skip during scanning.
	Ignore []string `yaml:"ignore,omitempty" json:"ignore,omitempty"`

	// Issues instructs the scanner to also scan open GitHub issues in Repository for blocker annotations.
	Issues bool `yaml:"issues,omitempty" json:"issues,omitempty"`

	// IssueLabel optionally restricts GitHub issue scanning to issues with a specific label (e.g. "blocked").
	IssueLabel string `yaml:"issue_label,omitempty" json:"issue_label,omitempty"`
}

// Blocker represents a single external blocker to monitor.
type Blocker struct {
	ID          string         `yaml:"id" json:"id"`
	Description string         `yaml:"description,omitempty" json:"description,omitempty"`
	Location    string         `yaml:"location,omitempty" json:"location,omitempty"` // e.g. "internal/auth/client.go:42" or "issue#15"
	Source      SourceConfig   `yaml:"source" json:"source"`
	OnResolved  []ActionConfig `yaml:"on_resolved,omitempty" json:"on_resolved,omitempty"`
}

// SourceConfig specifies how to check whether an external blocker is resolved.
type SourceConfig struct {
	// Type is one of: "github-issue", "github-release", "snapcraft", "http", "command", "dependency"
	Type string `yaml:"type" json:"type"`

	// GitHub Issue / PR / Release fields:
	// Repo in "owner/repo" format (or shorthand URL/ref in Issue)
	Repo   string `yaml:"repo,omitempty" json:"repo,omitempty"`
	Issue  int    `yaml:"issue,omitempty" json:"issue,omitempty"`   // Issue or PR number
	Reason string `yaml:"reason,omitempty" json:"reason,omitempty"` // Optional: e.g. "completed" (ignore "not_planned")

	// Version / Tag constraint (used by github-release, snapcraft, http, command):
	// Supports ">=1.2.0", "==2.0.0", "v1.3.0" (defaults to >=).
	Version    string `yaml:"version,omitempty" json:"version,omitempty"`
	IncludePre bool   `yaml:"include_prereleases,omitempty" json:"include_prereleases,omitempty"`

	// Snapcraft fields:
	Snap         string `yaml:"snap,omitempty" json:"snap,omitempty"`
	Channel      string `yaml:"channel,omitempty" json:"channel,omitempty"`           // Default: "latest/stable"
	Architecture string `yaml:"architecture,omitempty" json:"architecture,omitempty"` // Default: "amd64"
	Revision     string `yaml:"revision,omitempty" json:"revision,omitempty"`         // Optional revision constraint, e.g. ">=150"

	// HTTP / JSON check fields:
	URL        string            `yaml:"url,omitempty" json:"url,omitempty"`
	Method     string            `yaml:"method,omitempty" json:"method,omitempty"`
	Headers    map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
	StatusCode int               `yaml:"status_code,omitempty" json:"status_code,omitempty"` // Default: 200
	JSONPath   string            `yaml:"json_path,omitempty" json:"json_path,omitempty"`     // Dot-separated path, e.g. "status" or "release.version"
	Equals     string            `yaml:"equals,omitempty" json:"equals,omitempty"`           // Exact string match
	Regex      string            `yaml:"regex,omitempty" json:"regex,omitempty"`             // Regex match on extracted value or body

	// Command check fields:
	Command string `yaml:"command,omitempty" json:"command,omitempty"`

	// Dependency check fields: resolved when Package in Manifest of Repo at Ref satisfies Version.
	// Use it for local overrides (e.g. a CVE bump of a transitive dependency) that upstream
	// will make redundant once it ships the fixed version itself.
	Manifest string `yaml:"manifest,omitempty" json:"manifest,omitempty"` // Path in Repo, e.g. "go.mod", "uv.lock", "web/package-lock.json"
	Package  string `yaml:"package,omitempty" json:"package,omitempty"`   // e.g. "golang.org/x/net", "urllib3"
	// Ref is a git tag/branch/SHA, or "latest" (default) for the highest non-prerelease release.
	Ref string `yaml:"ref,omitempty" json:"ref,omitempty"`
	// RefFile and RefRegex read the ref from a local file instead, so the check follows the
	// upstream version you actually build (e.g. rockcraft.yaml with `source-tag: (\S+)`).
	// The first capture group (or the whole match) is used. Takes precedence over Ref.
	RefFile  string `yaml:"ref_file,omitempty" json:"ref_file,omitempty"`
	RefRegex string `yaml:"ref_regex,omitempty" json:"ref_regex,omitempty"`
}

// ActionConfig defines a follow-up action to run once a blocker is resolved.
type ActionConfig struct {
	// Type is one of: "create_issue", "comment_issue", "workflow_dispatch", "repository_dispatch", "command"
	Type string `yaml:"type" json:"type"`

	// Repo overrides the top-level target GitHub repository ("owner/repo").
	Repo string `yaml:"repo,omitempty" json:"repo,omitempty"`

	// create_issue / comment_issue fields:
	Issue        int      `yaml:"issue,omitempty" json:"issue,omitempty"`                   // Target issue/PR number for comment_issue
	Title        string   `yaml:"title,omitempty" json:"title,omitempty"`                   // Issue title template for create_issue
	Body         string   `yaml:"body,omitempty" json:"body,omitempty"`                     // Issue/comment body template
	Labels       []string `yaml:"labels,omitempty" json:"labels,omitempty"`                 // Labels for create_issue or add to comment_issue
	RemoveLabels []string `yaml:"remove_labels,omitempty" json:"remove_labels,omitempty"`   // Labels to remove on comment_issue (e.g. ["blocked"])
	Assignees    []string `yaml:"assignees,omitempty" json:"assignees,omitempty"`           // Assignees for create_issue
	SkipIfClosed *bool    `yaml:"skip_if_closed,omitempty" json:"skip_if_closed,omitempty"` // Skip comment_issue if target issue is already closed (default true)

	// workflow_dispatch / repository_dispatch fields:
	Workflow  string            `yaml:"workflow,omitempty" json:"workflow,omitempty"`     // Workflow filename or ID
	Ref       string            `yaml:"ref,omitempty" json:"ref,omitempty"`               // Git ref (default: "main")
	Inputs    map[string]string `yaml:"inputs,omitempty" json:"inputs,omitempty"`         // Workflow inputs (templates supported)
	EventType string            `yaml:"event_type,omitempty" json:"event_type,omitempty"` // repository_dispatch event_type

	// command fields (e.g., launching an agent CLI or script):
	Command string            `yaml:"command,omitempty" json:"command,omitempty"`
	Env     map[string]string `yaml:"env,omitempty" json:"env,omitempty"`

	// Deduplicate controls whether to check for existing GitHub issues/comments with the blocker marker.
	// Defaults to true when nil.
	Deduplicate *bool `yaml:"deduplicate,omitempty" json:"deduplicate,omitempty"`
}

// ShouldDeduplicate returns true unless Deduplicate was explicitly set to false.
func (a ActionConfig) ShouldDeduplicate() bool {
	if a.Deduplicate == nil {
		return true
	}
	return *a.Deduplicate
}

// ShouldSkipIfClosed returns true unless SkipIfClosed was explicitly set to false.
func (a ActionConfig) ShouldSkipIfClosed() bool {
	if a.SkipIfClosed == nil {
		return true
	}
	return *a.SkipIfClosed
}

// Normalize canonicalizes shorthand fields on SourceConfig (such as "snap: go/1.26/stable").
func (s *SourceConfig) Normalize() {
	if s.Type == "snap" {
		s.Type = "snapcraft"
	}
	if s.Type == "dep" {
		s.Type = "dependency"
	}
	if s.Type == "snapcraft" && strings.Contains(s.Snap, "/") && s.Channel == "" {
		parts := strings.SplitN(s.Snap, "/", 2)
		s.Snap = parts[0]
		s.Channel = parts[1]
	}
}

// Load reads and parses a YAML configuration file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}
	if cfg.Repository == "" {
		cfg.Repository = os.Getenv("GITHUB_REPOSITORY")
	}
	for i := range cfg.Blockers {
		b := &cfg.Blockers[i]
		b.Source.Normalize()
		if b.ID == "" {
			b.ID = deriveID(*b, i)
		}
		if len(b.OnResolved) == 0 && len(cfg.DefaultActions) > 0 {
			b.OnResolved = cfg.DefaultActions
		}
	}
	return &cfg, nil
}

func deriveID(b Blocker, idx int) string {
	switch b.Source.Type {
	case "github-issue":
		return fmt.Sprintf("gh-issue:%s#%d", b.Source.Repo, b.Source.Issue)
	case "github-release":
		return fmt.Sprintf("gh-release:%s:%s", b.Source.Repo, b.Source.Version)
	case "snapcraft":
		ch := b.Source.Channel
		if ch == "" {
			ch = "latest/stable"
		}
		return fmt.Sprintf("snap:%s:%s:%s%s", b.Source.Snap, ch, b.Source.Version, b.Source.Revision)
	case "dependency":
		ref := b.Source.Ref
		if b.Source.RefFile != "" {
			ref = b.Source.RefFile
		}
		if ref == "" {
			ref = "latest"
		}
		return fmt.Sprintf("dep:%s@%s:%s:%s:%s", b.Source.Repo, ref, b.Source.Manifest, b.Source.Package, b.Source.Version)
	default:
		return fmt.Sprintf("blocker-%d", idx+1)
	}
}

// Validate checks that a Blocker has valid source and action definitions.
func (b *Blocker) Validate() error {
	b.Source.Normalize()
	if strings.TrimSpace(b.ID) == "" {
		return fmt.Errorf("blocker missing id")
	}
	switch b.Source.Type {
	case "github-issue":
		if b.Source.Repo == "" || b.Source.Issue <= 0 {
			return fmt.Errorf("blocker %q: github-issue source requires 'repo' (owner/name) and positive 'issue'", b.ID)
		}
	case "github-release":
		if b.Source.Repo == "" {
			return fmt.Errorf("blocker %q: github-release source requires 'repo' (owner/name)", b.ID)
		}
	case "snapcraft":
		if b.Source.Snap == "" {
			return fmt.Errorf("blocker %q: snapcraft source requires 'snap'", b.ID)
		}
		if b.Source.Version == "" && b.Source.Revision == "" && b.Source.Channel == "" {
			return fmt.Errorf("blocker %q: snapcraft source requires 'version', 'revision', or 'channel'", b.ID)
		}
	case "http":
		if b.Source.URL == "" {
			return fmt.Errorf("blocker %q: http source requires 'url'", b.ID)
		}
	case "command":
		if b.Source.Command == "" {
			return fmt.Errorf("blocker %q: command source requires 'command'", b.ID)
		}
	case "dependency":
		if b.Source.Repo == "" || b.Source.Manifest == "" || b.Source.Package == "" || b.Source.Version == "" {
			return fmt.Errorf("blocker %q: dependency source requires 'repo', 'manifest', 'package' and 'version'", b.ID)
		}
		if (b.Source.RefFile == "") != (b.Source.RefRegex == "") {
			return fmt.Errorf("blocker %q: dependency source needs both 'ref_file' and 'ref_regex' (or neither)", b.ID)
		}
	default:
		return fmt.Errorf("blocker %q: unknown source type %q (expected github-issue, github-release, snapcraft, http, command, or dependency)", b.ID, b.Source.Type)
	}
	return nil
}
