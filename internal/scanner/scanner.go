// SPDX-FileCopyrightText: 2026 Daniel Arndt <dan@arndt.ca>
// SPDX-License-Identifier: Apache-2.0

package scanner

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/DanielArndt/fogcutter/internal/config"
)

var (
	// Matches:
	//   BLOCKER(spec): description
	//   TODO(blocked-by:spec): description
	//   TODO(blocked-on:spec): description
	//   FIXME(blocked-by:spec): description
	inlineAnnotationRe = regexp.MustCompile(`(?i)\b(?:BLOCKER\(([^)]+)\)|(?:TODO|FIXME)\(\s*blocked[-_ ](?:by|on):\s*([^)]+)\))\s*:?\s*(.*)$`)

	// Matches in GitHub issue bodies or comments:
	//   Blocked by owner/repo#123
	//   Blocked on snap:go/1.26/stable>=1.26.9
	//   Blocked by https://github.com/owner/repo/issues/123
	issueBlockedByRe = regexp.MustCompile(`(?im)^\s*(?:[-*]\s*)?blocked\s+(?:by|on):?\s+(\S+)(.*)$`)

	ghURLRe   = regexp.MustCompile(`^https://github\.com/([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)/(?:issues|pull)/(\d+)$`)
	ghShortRe = regexp.MustCompile(`^([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)#(\d+)$`)
	verOpRe   = regexp.MustCompile(`^(.*?)([><=!]+.*)$`)
)

var defaultIgnoredDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	"dist":         true,
	"build":        true,
	".venv":        true,
	"venv":         true,
	"__pycache__":  true,
}

// ScanFiles walks the given paths and extracts inline blocker annotations.
func ScanFiles(paths []string, ignorePatterns []string, defaultActions []config.ActionConfig) ([]config.Blocker, error) {
	var results []config.Blocker
	seen := make(map[string]bool)

	for _, root := range paths {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			name := d.Name()
			if d.IsDir() {
				if defaultIgnoredDirs[name] && path != root {
					return filepath.SkipDir
				}
				if matchesIgnore(path, name, ignorePatterns) {
					return filepath.SkipDir
				}
				return nil
			}
			if matchesIgnore(path, name, ignorePatterns) {
				return nil
			}
			// Skip binary or very large files (>2MB)
			info, err := d.Info()
			if err != nil || !info.Mode().IsRegular() || info.Size() > 2*1024*1024 {
				return nil
			}

			found, err := scanFile(path, defaultActions)
			if err != nil {
				return nil // ignore unreadable files
			}
			for _, b := range found {
				if !seen[b.ID] {
					seen[b.ID] = true
					results = append(results, b)
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return results, nil
}

func matchesIgnore(path, name string, patterns []string) bool {
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if matched, _ := filepath.Match(p, name); matched {
			return true
		}
		if matched, _ := filepath.Match(p, path); matched {
			return true
		}
		if strings.HasPrefix(path, strings.TrimSuffix(p, "/")) {
			return true
		}
	}
	return false
}

func scanFile(path string, defaultActions []config.ActionConfig) ([]config.Blocker, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	var blockers []config.Blocker
	for i, line := range lines {
		lineNum := i + 1
		// Quick check before regex
		lower := strings.ToLower(line)
		if !strings.Contains(lower, "blocker(") &&
			!strings.Contains(lower, "blocked-by:") && !strings.Contains(lower, "blocked_by:") && !strings.Contains(lower, "blocked by:") &&
			!strings.Contains(lower, "blocked-on:") && !strings.Contains(lower, "blocked_on:") && !strings.Contains(lower, "blocked on:") {
			continue
		}
		m := inlineAnnotationRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		spec := strings.TrimSpace(m[1])
		if spec == "" {
			spec = strings.TrimSpace(m[2])
		}
		desc := strings.TrimSpace(m[3])
		// Strip trailing comment closers like "*/", "-->", or "`"
		desc = strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(desc, "*/"), "-->"), "`"))

		src, err := ParseSpec(spec)
		if err != nil {
			continue
		}
		if src.Type == "dependency" && src.Ref == "" {
			// Follow the upstream version this file builds, so the check moves with
			// whatever bumps source-tag (e.g. Renovate). Explicit @latest opts out.
			src.Ref = enclosingSourceRef(lines, i, src.Repo)
		}
		loc := fmt.Sprintf("%s:%d", path, lineNum)
		id := fmt.Sprintf("code:%s:%s", loc, spec)
		actions := defaultActions
		if len(actions) == 0 {
			actions = []config.ActionConfig{
				{
					Type:  "create_issue",
					Title: "Blocker resolved: {{.ID}}",
					Body:  "External blocker defined at `{{.Location}}` has been resolved.\n\n- **Blocker:** `{{.SourceSummary}}`\n- **Details:** {{.Resolution}}\n- **Note:** {{.Description}}",
				},
			}
		}
		blockers = append(blockers, config.Blocker{
			ID:          id,
			Description: desc,
			Location:    loc,
			Source:      src,
			OnResolved:  actions,
		})
	}
	return blockers, nil
}

var (
	yamlKeyRe       = regexp.MustCompile(`^(\s*)(?:-\s+)?([A-Za-z0-9_.-]+):(?:\s|$)`)
	sourceRefKeyRe  = regexp.MustCompile(`^(\s*)source-(?:tag|commit):\s*["']?([^"'\s#]+)`)
	sourceRepoKeyRe = regexp.MustCompile(`^\s*source:\s*["']?([^"'\s#]+)`)
)

// enclosingSourceRef returns the source-tag (or source-commit) of the craft part
// (rockcraft.yaml, snapcraft.yaml, charmcraft.yaml) whose keys contain line idx,
// typically an annotation inside override-build. The part's key level is the
// indentation of the nearest key at or above the annotation's indentation; keys
// at that level are collected up and down until a less indented key (the part
// header) is reached. If the part's `source:` does not mention repo, nothing is
// inferred. Returns "" when no ref is found; the checker then uses the latest
// release.
func enclosingSourceRef(lines []string, idx int, repo string) string {
	annIndent := leadingSpaces(lines[idx])
	partIndent := -1
	for j := idx - 1; j >= 0; j-- {
		if m := yamlKeyRe.FindStringSubmatch(lines[j]); m != nil && len(m[1]) <= annIndent {
			partIndent = len(m[1])
			// A YAML comment directly under "  go-server:" sits inside that mapping,
			// at its key level, rather than beside it.
			if partIndent < annIndent && strings.HasSuffix(strings.TrimSpace(lines[j]), ":") {
				partIndent = annIndent
			}
			break
		}
	}
	if partIndent < 0 {
		return ""
	}

	var ref, source string
	visit := func(line string) bool {
		m := yamlKeyRe.FindStringSubmatch(line)
		if m == nil {
			return true // blank, comment or block scalar content
		}
		indent := len(m[1])
		if indent < partIndent {
			return false // part header or beyond: stop
		}
		if indent > partIndent {
			return true // nested key, e.g. inside build-environment
		}
		if r := sourceRefKeyRe.FindStringSubmatch(line); r != nil && ref == "" {
			ref = r[2]
		} else if s := sourceRepoKeyRe.FindStringSubmatch(line); s != nil {
			source = s[1]
		}
		return true
	}
	for j := idx - 1; j >= 0 && visit(lines[j]); j-- {
	}
	for j := idx + 1; j < len(lines) && visit(lines[j]); j++ {
	}

	if ref == "" {
		return ""
	}
	if source != "" && !strings.Contains(strings.TrimSuffix(source, ".git"), repo) {
		return ""
	}
	return ref
}

func leadingSpaces(s string) int {
	return len(s) - len(strings.TrimLeft(s, " \t"))
}

// ParseSpec parses a compact blocker specification string into a SourceConfig.
// Supported forms:
//   - owner/repo#123
//   - https://github.com/owner/repo/issues/123
//   - github-issue:owner/repo#123
//   - github-release:owner/repo>=v1.2.0
//   - snap:lxd:latest/stable>=5.21  or  snap:lxd>=5.21
//   - dep:owner/repo[@ref]:manifest:package>=1.2.3  (without @ref, file scanning uses the
//     enclosing part's source-tag; otherwise the latest release)
func ParseSpec(spec string) (config.SourceConfig, error) {
	spec = strings.TrimSpace(spec)
	if m := ghURLRe.FindStringSubmatch(spec); m != nil {
		num, _ := strconv.Atoi(m[2])
		return config.SourceConfig{
			Type:  "github-issue",
			Repo:  m[1],
			Issue: num,
		}, nil
	}
	if m := ghShortRe.FindStringSubmatch(spec); m != nil {
		num, _ := strconv.Atoi(m[2])
		return config.SourceConfig{
			Type:  "github-issue",
			Repo:  m[1],
			Issue: num,
		}, nil
	}

	if strings.HasPrefix(spec, "github-issue:") {
		rest := strings.TrimPrefix(spec, "github-issue:")
		return ParseSpec(rest)
	}

	if strings.HasPrefix(spec, "github-release:") {
		rest := strings.TrimPrefix(spec, "github-release:")
		m := verOpRe.FindStringSubmatch(rest)
		if m != nil {
			return config.SourceConfig{
				Type:    "github-release",
				Repo:    strings.TrimSpace(m[1]),
				Version: strings.TrimSpace(m[2]),
			}, nil
		}
		parts := strings.SplitN(rest, "@", 2)
		if len(parts) == 2 {
			return config.SourceConfig{
				Type:    "github-release",
				Repo:    strings.TrimSpace(parts[0]),
				Version: strings.TrimSpace(parts[1]),
			}, nil
		}
		return config.SourceConfig{}, fmt.Errorf("invalid github-release spec %q", spec)
	}

	if strings.HasPrefix(spec, "snap:") || strings.HasPrefix(spec, "snapcraft:") {
		rest := strings.TrimPrefix(strings.TrimPrefix(spec, "snapcraft:"), "snap:")
		var left, constraint string
		if m := verOpRe.FindStringSubmatch(rest); m != nil {
			left = strings.TrimSpace(m[1])
			constraint = strings.TrimSpace(m[2])
		} else if parts := strings.SplitN(rest, "@", 2); len(parts) == 2 {
			left = strings.TrimSpace(parts[0])
			constraint = strings.TrimSpace(parts[1])
		} else {
			return config.SourceConfig{}, fmt.Errorf("invalid snap spec %q (missing version constraint)", spec)
		}
		snapName := left
		channel := "latest/stable"
		if idx := strings.IndexByte(left, ':'); idx != -1 {
			snapName = left[:idx]
			channel = left[idx+1:]
		} else if idx := strings.IndexByte(left, '/'); idx != -1 {
			snapName = left[:idx]
			channel = left[idx+1:]
		}
		return config.SourceConfig{
			Type:    "snapcraft",
			Snap:    snapName,
			Channel: channel,
			Version: constraint,
		}, nil
	}

	if strings.HasPrefix(spec, "dep:") || strings.HasPrefix(spec, "dependency:") {
		rest := strings.TrimPrefix(strings.TrimPrefix(spec, "dependency:"), "dep:")
		m := verOpRe.FindStringSubmatch(rest)
		if m == nil {
			return config.SourceConfig{}, fmt.Errorf("invalid dep spec %q (missing version constraint)", spec)
		}
		parts := strings.Split(strings.TrimSpace(m[1]), ":")
		if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
			return config.SourceConfig{}, fmt.Errorf("invalid dep spec %q (want dep:owner/repo[@ref]:manifest:package>=version)", spec)
		}
		repo, ref := parts[0], ""
		if i := strings.IndexByte(repo, '@'); i != -1 {
			repo, ref = repo[:i], repo[i+1:]
		}
		return config.SourceConfig{
			Type:     "dependency",
			Repo:     repo,
			Ref:      ref,
			Manifest: parts[1],
			Package:  parts[2],
			Version:  strings.TrimSpace(m[2]),
		}, nil
	}

	return config.SourceConfig{}, fmt.Errorf("unrecognized blocker spec %q", spec)
}

// ScanGitHubIssues queries open issues in repo (owner/name) and extracts any inline BLOCKER(...) or "Blocked by ..." lines.
func ScanGitHubIssues(ctx context.Context, client *http.Client, apiBase, token, repo, label string) ([]config.Blocker, error) {
	if repo == "" {
		return nil, fmt.Errorf("repository (owner/name) is required to scan GitHub issues")
	}
	if apiBase == "" {
		apiBase = "https://api.github.com"
	}
	u := fmt.Sprintf("%s/repos/%s/issues?state=open&per_page=100", strings.TrimRight(apiBase, "/"), repo)
	if label != "" {
		u += "&labels=" + url.QueryEscape(label)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching open issues from %s: %w", repo, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching open issues from %s: HTTP %d", repo, resp.StatusCode)
	}

	var issues []struct {
		Number      int       `json:"number"`
		Title       string    `json:"title"`
		Body        string    `json:"body"`
		Comments    int       `json:"comments"`
		PullRequest *struct{} `json:"pull_request,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&issues); err != nil {
		return nil, fmt.Errorf("decoding issues response: %w", err)
	}

	var blockers []config.Blocker
	seen := make(map[string]bool)
	for _, iss := range issues {
		combinedText := iss.Body
		if iss.Comments > 0 {
			commentsText, err := fetchIssueCommentsText(ctx, client, apiBase, token, repo, iss.Number)
			if err == nil && commentsText != "" {
				combinedText += "\n" + commentsText
			}
		}
		found := ExtractFromIssueBody(repo, iss.Number, iss.Title, combinedText)
		for _, b := range found {
			if !seen[b.ID] {
				seen[b.ID] = true
				blockers = append(blockers, b)
			}
		}
	}
	return blockers, nil
}

func fetchIssueCommentsText(ctx context.Context, client *http.Client, apiBase, token, repo string, issueNum int) (string, error) {
	u := fmt.Sprintf("%s/repos/%s/issues/%d/comments?per_page=100", strings.TrimRight(apiBase, "/"), repo, issueNum)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var comments []struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&comments); err != nil {
		return "", err
	}
	var sb strings.Builder
	for _, c := range comments {
		if strings.Contains(c.Body, "<!-- fogcutter:id=") {
			continue
		}
		sb.WriteString(c.Body)
		sb.WriteByte('\n')
	}
	return sb.String(), nil
}

// ExtractFromIssueBody parses an issue body for BLOCKER(...) or "Blocked by <ref>" lines.
func ExtractFromIssueBody(repo string, issueNum int, issueTitle, body string) []config.Blocker {
	var out []config.Blocker
	seen := make(map[string]bool)

	lines := strings.Split(body, "\n")
	for _, line := range lines {
		var spec, desc string
		if m := inlineAnnotationRe.FindStringSubmatch(line); m != nil {
			spec = strings.TrimSpace(m[1])
			if spec == "" {
				spec = strings.TrimSpace(m[2])
			}
			desc = strings.TrimSpace(m[3])
		} else if m := issueBlockedByRe.FindStringSubmatch(line); m != nil {
			spec = strings.TrimSpace(m[1])
			desc = strings.TrimSpace(m[2])
		}
		if spec == "" {
			continue
		}
		src, err := ParseSpec(spec)
		if err != nil {
			continue
		}
		id := fmt.Sprintf("issue:%s#%d<-%s", repo, issueNum, spec)
		if seen[id] {
			continue
		}
		seen[id] = true
		if desc == "" {
			desc = fmt.Sprintf("Unblocks #%d (%s)", issueNum, issueTitle)
		}
		out = append(out, config.Blocker{
			ID:          id,
			Description: desc,
			Location:    fmt.Sprintf("%s#%d", repo, issueNum),
			Source:      src,
			OnResolved: []config.ActionConfig{
				{
					Type:  "comment_issue",
					Repo:  repo,
					Issue: issueNum,
					Body:  "External blocker `{{.SourceSummary}}` is now resolved ({{.Resolution}}).\n\n{{.Description}}",
				},
			},
		})
	}
	return out
}
