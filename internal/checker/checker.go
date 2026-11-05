package checker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/DanielArndt/fogcutter/internal/config"
	"github.com/DanielArndt/fogcutter/internal/version"
)

// Result holds the evaluation outcome of a single Blocker.
type Result struct {
	Blocker       config.Blocker `json:"blocker"`
	Resolved      bool           `json:"resolved"`
	Resolution    string         `json:"resolution"`
	SourceSummary string         `json:"source_summary"`
	CheckedAt     time.Time      `json:"checked_at"`
	Error         string         `json:"error,omitempty"`
}

// Evaluator checks external blockers against upstream APIs.
type Evaluator struct {
	HTTPClient    *http.Client
	GitHubAPIBase string
	SnapAPIBase   string
	GitHubToken   string
}

// NewEvaluator returns an Evaluator configured with sensible defaults.
func NewEvaluator(githubToken string) *Evaluator {
	return &Evaluator{
		HTTPClient:    &http.Client{Timeout: 20 * time.Second},
		GitHubAPIBase: "https://api.github.com",
		SnapAPIBase:   "https://api.snapcraft.io",
		GitHubToken:   githubToken,
	}
}

// SummarizeSource returns a concise human-readable identifier for a blocker's source.
func SummarizeSource(src config.SourceConfig) string {
	src.Normalize()
	switch src.Type {
	case "github-issue":
		return fmt.Sprintf("%s#%d", src.Repo, src.Issue)
	case "github-release":
		v := src.Version
		if v == "" {
			v = "latest"
		}
		return fmt.Sprintf("github-release:%s (%s)", src.Repo, v)
	case "snapcraft":
		ch := src.Channel
		if ch == "" {
			ch = "latest/stable"
		}
		target := src.Version
		if target == "" && src.Revision != "" {
			target = "rev" + src.Revision
		}
		if target != "" {
			return fmt.Sprintf("snap:%s/%s (%s)", src.Snap, ch, target)
		}
		return fmt.Sprintf("snap:%s/%s", src.Snap, ch)
	case "http":
		return fmt.Sprintf("http:%s", src.URL)
	case "command":
		return fmt.Sprintf("cmd:%s", src.Command)
	case "dependency":
		ref := src.Ref
		if src.RefFile != "" {
			ref = "<" + src.RefFile + ">"
		}
		if ref == "" {
			ref = "latest"
		}
		return fmt.Sprintf("dep:%s@%s:%s:%s%s", src.Repo, ref, src.Manifest, src.Package, src.Version)
	default:
		return src.Type
	}
}

// Check evaluates a single Blocker.
func (e *Evaluator) Check(ctx context.Context, b config.Blocker) Result {
	b.Source.Normalize()
	res := Result{
		Blocker:       b,
		SourceSummary: SummarizeSource(b.Source),
		CheckedAt:     time.Now().UTC(),
	}
	if err := b.Validate(); err != nil {
		res.Error = err.Error()
		return res
	}

	var (
		resolved   bool
		resolution string
		err        error
	)

	switch b.Source.Type {
	case "github-issue":
		resolved, resolution, err = e.checkGitHubIssue(ctx, b.Source)
	case "github-release":
		resolved, resolution, err = e.checkGitHubRelease(ctx, b.Source)
	case "snapcraft":
		resolved, resolution, err = e.checkSnapcraft(ctx, b.Source)
	case "http":
		resolved, resolution, err = e.checkHTTP(ctx, b.Source)
	case "command":
		resolved, resolution, err = e.checkCommand(ctx, b.Source)
	case "dependency":
		resolved, resolution, err = e.checkDependency(ctx, b.Source)
	default:
		err = fmt.Errorf("unsupported source type %q", b.Source.Type)
	}

	if err != nil {
		res.Error = err.Error()
		return res
	}
	res.Resolved = resolved
	res.Resolution = resolution
	return res
}

func (e *Evaluator) checkGitHubIssue(ctx context.Context, src config.SourceConfig) (bool, string, error) {
	u := fmt.Sprintf("%s/repos/%s/issues/%d", strings.TrimRight(e.GitHubAPIBase, "/"), src.Repo, src.Issue)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return false, "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if e.GitHubToken != "" {
		req.Header.Set("Authorization", "Bearer "+e.GitHubToken)
	}

	resp, err := e.HTTPClient.Do(req)
	if err != nil {
		return false, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, "", fmt.Errorf("github API %s returned HTTP %d", u, resp.StatusCode)
	}

	var issue struct {
		State       string `json:"state"`
		StateReason string `json:"state_reason"`
		Title       string `json:"title"`
		HTMLURL     string `json:"html_url"`
		ClosedAt    string `json:"closed_at"`
		PullRequest *struct {
			MergedAt *string `json:"merged_at"`
		} `json:"pull_request,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return false, "", err
	}

	if issue.State != "closed" {
		return false, fmt.Sprintf("%s#%d is still %s (%s)", src.Repo, src.Issue, issue.State, issue.Title), nil
	}

	if src.Reason != "" {
		if issue.PullRequest != nil && (src.Reason == "merged" || src.Reason == "completed") {
			if issue.PullRequest.MergedAt == nil {
				return false, fmt.Sprintf("PR %s#%d was closed without merging", src.Repo, src.Issue), nil
			}
		} else if !strings.EqualFold(issue.StateReason, src.Reason) {
			return false, fmt.Sprintf("%s#%d closed with reason %q (wanted %q)", src.Repo, src.Issue, issue.StateReason, src.Reason), nil
		}
	}

	reasonSuffix := ""
	if issue.StateReason != "" {
		reasonSuffix = fmt.Sprintf(" (%s)", issue.StateReason)
	} else if issue.PullRequest != nil && issue.PullRequest.MergedAt != nil {
		reasonSuffix = " (merged)"
	}
	return true, fmt.Sprintf("[%s#%d: %s](%s) is closed%s", src.Repo, src.Issue, issue.Title, issue.HTMLURL, reasonSuffix), nil
}

func (e *Evaluator) checkGitHubRelease(ctx context.Context, src config.SourceConfig) (bool, string, error) {
	base := strings.TrimRight(e.GitHubAPIBase, "/")
	u := fmt.Sprintf("%s/repos/%s/releases?per_page=30", base, src.Repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return false, "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if e.GitHubToken != "" {
		req.Header.Set("Authorization", "Bearer "+e.GitHubToken)
	}

	resp, err := e.HTTPClient.Do(req)
	if err != nil {
		return false, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, "", fmt.Errorf("github releases API %s returned HTTP %d", u, resp.StatusCode)
	}

	var releases []struct {
		TagName    string `json:"tag_name"`
		Name       string `json:"name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
		HTMLURL    string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return false, "", err
	}

	constraint := src.Version
	if constraint == "" {
		constraint = "*"
	}

	var latestSeen string
	for _, rel := range releases {
		if rel.Draft {
			continue
		}
		if rel.Prerelease && !src.IncludePre {
			continue
		}
		if latestSeen == "" {
			latestSeen = rel.TagName
		}
		ok, err := version.Match(rel.TagName, constraint)
		if err != nil {
			return false, "", err
		}
		if ok {
			return true, fmt.Sprintf("Release [%s %s](%s) satisfies %s", src.Repo, rel.TagName, rel.HTMLURL, constraint), nil
		}
	}

	// Fallback to git tags if no releases exist
	if len(releases) == 0 {
		tagsURL := fmt.Sprintf("%s/repos/%s/tags?per_page=30", base, src.Repo)
		tagReq, err := http.NewRequestWithContext(ctx, http.MethodGet, tagsURL, nil)
		if err == nil {
			tagReq.Header.Set("Accept", "application/vnd.github+json")
			if e.GitHubToken != "" {
				tagReq.Header.Set("Authorization", "Bearer "+e.GitHubToken)
			}
			if tagResp, err := e.HTTPClient.Do(tagReq); err == nil {
				defer tagResp.Body.Close()
				if tagResp.StatusCode == http.StatusOK {
					var tags []struct {
						Name string `json:"name"`
					}
					if json.NewDecoder(tagResp.Body).Decode(&tags) == nil {
						for _, t := range tags {
							if latestSeen == "" {
								latestSeen = t.Name
							}
							if ok, err := version.Match(t.Name, constraint); err == nil && ok {
								return true, fmt.Sprintf("Tag %s@%s satisfies %s", src.Repo, t.Name, constraint), nil
							}
						}
					}
				}
			}
		}
	}

	if latestSeen == "" {
		return false, fmt.Sprintf("no releases found for %s (waiting for %s)", src.Repo, constraint), nil
	}
	return false, fmt.Sprintf("latest release of %s is %s (waiting for %s)", src.Repo, latestSeen, constraint), nil
}

func (e *Evaluator) checkSnapcraft(ctx context.Context, src config.SourceConfig) (bool, string, error) {
	base := strings.TrimRight(e.SnapAPIBase, "/")
	u := fmt.Sprintf("%s/v2/snaps/info/%s", base, src.Snap)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return false, "", err
	}
	req.Header.Set("Snap-Device-Series", "16")
	req.Header.Set("Accept", "application/json")

	resp, err := e.HTTPClient.Do(req)
	if err != nil {
		return false, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, "", fmt.Errorf("snapcraft API %s returned HTTP %d", u, resp.StatusCode)
	}

	var info struct {
		Name       string `json:"name"`
		ChannelMap []struct {
			Channel struct {
				Architecture string `json:"architecture"`
				Name         string `json:"name"`
				Track        string `json:"track"`
				Risk         string `json:"risk"`
			} `json:"channel"`
			Revision int    `json:"revision"`
			Version  string `json:"version"`
		} `json:"channel-map"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return false, "", err
	}

	wantChannel := src.Channel
	if wantChannel == "" {
		wantChannel = "latest/stable"
	}
	wantArch := src.Architecture
	if wantArch == "" {
		wantArch = "amd64"
	}

	for _, entry := range info.ChannelMap {
		if entry.Channel.Architecture != wantArch {
			continue
		}
		fullTrackRisk := entry.Channel.Track + "/" + entry.Channel.Risk
		if entry.Channel.Name != wantChannel && fullTrackRisk != wantChannel && (wantChannel != entry.Channel.Risk || entry.Channel.Track != "latest") {
			continue
		}

		if src.Version != "" {
			ok, err := version.Match(entry.Version, src.Version)
			if err != nil {
				return false, "", err
			}
			if !ok {
				return false, fmt.Sprintf("snap %s on %s (%s) is at version %s (rev %d), waiting for %s",
					src.Snap, entry.Channel.Name, wantArch, entry.Version, entry.Revision, src.Version), nil
			}
		}

		if src.Revision != "" {
			ok, err := version.Match(strconv.Itoa(entry.Revision), src.Revision)
			if err != nil {
				return false, "", err
			}
			if !ok {
				return false, fmt.Sprintf("snap %s on %s (%s) is at revision %d (version %s), waiting for revision %s",
					src.Snap, entry.Channel.Name, wantArch, entry.Revision, entry.Version, src.Revision), nil
			}
		}

		return true, fmt.Sprintf("snap %q on %s (%s) is at version %s (revision %d)",
			src.Snap, entry.Channel.Name, wantArch, entry.Version, entry.Revision), nil
	}

	return false, fmt.Sprintf("channel %q (%s) not found for snap %q", wantChannel, wantArch, src.Snap), nil
}

func (e *Evaluator) checkHTTP(ctx context.Context, src config.SourceConfig) (bool, string, error) {
	method := src.Method
	if method == "" {
		method = http.MethodGet
	}
	req, err := http.NewRequestWithContext(ctx, method, src.URL, nil)
	if err != nil {
		return false, "", err
	}
	for k, v := range src.Headers {
		req.Header.Set(k, v)
	}

	resp, err := e.HTTPClient.Do(req)
	if err != nil {
		return false, "", err
	}
	defer resp.Body.Close()

	wantStatus := src.StatusCode
	if wantStatus == 0 {
		wantStatus = http.StatusOK
	}
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return false, "", err
	}

	if resp.StatusCode != wantStatus {
		return false, fmt.Sprintf("HTTP %s returned %d (wanted %d)", src.URL, resp.StatusCode, wantStatus), nil
	}

	candidate := strings.TrimSpace(string(bodyBytes))
	if src.JSONPath != "" {
		var payload any
		if err := json.Unmarshal(bodyBytes, &payload); err != nil {
			return false, "", fmt.Errorf("decoding JSON from %s: %w", src.URL, err)
		}
		val, ok := extractJSONPath(payload, src.JSONPath)
		if !ok {
			return false, fmt.Sprintf("JSON path %q not found in response from %s", src.JSONPath, src.URL), nil
		}
		candidate = fmt.Sprintf("%v", val)
	}

	ok, detail, err := matchValue(candidate, src)
	if err != nil {
		return false, "", err
	}
	if !ok {
		return false, fmt.Sprintf("HTTP %s value %q did not match (%s)", src.URL, candidate, detail), nil
	}
	return true, fmt.Sprintf("HTTP %s satisfied check (%s)", src.URL, detail), nil
}

func (e *Evaluator) checkCommand(ctx context.Context, src config.SourceConfig) (bool, string, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", src.Command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	outStr := strings.TrimSpace(stdout.String())
	if err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = outStr
		}
		return false, fmt.Sprintf("command exited non-zero (%v): %s", err, errMsg), nil
	}

	if src.Equals == "" && src.Regex == "" && src.Version == "" {
		return true, fmt.Sprintf("command %q succeeded (output: %s)", src.Command, outStr), nil
	}

	ok, detail, matchErr := matchValue(outStr, src)
	if matchErr != nil {
		return false, "", matchErr
	}
	if !ok {
		return false, fmt.Sprintf("command output %q did not match (%s)", outStr, detail), nil
	}
	return true, fmt.Sprintf("command %q output %q matched (%s)", src.Command, outStr, detail), nil
}

func matchValue(candidate string, src config.SourceConfig) (bool, string, error) {
	if src.Equals != "" {
		if candidate == src.Equals {
			return true, fmt.Sprintf("equals %q", src.Equals), nil
		}
		return false, fmt.Sprintf("expected %q", src.Equals), nil
	}
	if src.Regex != "" {
		re, err := regexp.Compile(src.Regex)
		if err != nil {
			return false, "", fmt.Errorf("invalid regex %q: %w", src.Regex, err)
		}
		if re.MatchString(candidate) {
			return true, fmt.Sprintf("matched regex %q", src.Regex), nil
		}
		return false, fmt.Sprintf("did not match regex %q", src.Regex), nil
	}
	if src.Version != "" {
		ok, err := version.Match(candidate, src.Version)
		if err != nil {
			return false, "", err
		}
		return ok, fmt.Sprintf("version %s vs %s", candidate, src.Version), nil
	}
	return true, fmt.Sprintf("value=%q", candidate), nil
}

func extractJSONPath(root any, path string) (any, bool) {
	path = strings.TrimPrefix(path, ".")
	if path == "" {
		return root, true
	}
	parts := strings.Split(path, ".")
	cur := root
	for _, p := range parts {
		switch node := cur.(type) {
		case map[string]any:
			next, exists := node[p]
			if !exists {
				return nil, false
			}
			cur = next
		case []any:
			idx, err := strconv.Atoi(p)
			if err != nil || idx < 0 || idx >= len(node) {
				return nil, false
			}
			cur = node[idx]
		default:
			return nil, false
		}
	}
	return cur, true
}
