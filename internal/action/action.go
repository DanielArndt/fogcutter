// SPDX-FileCopyrightText: 2026 Daniel Arndt <dan@arndt.ca>
// SPDX-License-Identifier: Apache-2.0

package action

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"text/template"
	"time"

	"github.com/DanielArndt/fogcutter/internal/checker"
	"github.com/DanielArndt/fogcutter/internal/config"
)

// ExecutionRecord describes the outcome of running (or skipping) a follow-up action.
type ExecutionRecord struct {
	BlockerID string `json:"blocker_id"`
	Type      string `json:"type"`
	Target    string `json:"target"`
	Status    string `json:"status"` // "executed", "dry-run", "skipped-duplicate", "error"
	Detail    string `json:"detail"`
	Error     string `json:"error,omitempty"`
}

// Runner executes follow-up actions for resolved blockers.
type Runner struct {
	HTTPClient    *http.Client
	GitHubAPIBase string
	GitHubToken   string
	DefaultRepo   string
	DryRun        bool
	State         *StateStore
}

// StateStore persists triggered blocker IDs in a local JSON file when configured.
type StateStore struct {
	Path      string               `json:"-"`
	Triggered map[string]time.Time `json:"triggered"`
}

// LoadStateStore loads or initializes a JSON state file.
func LoadStateStore(path string) (*StateStore, error) {
	s := &StateStore{
		Path:      path,
		Triggered: make(map[string]time.Time),
	}
	if path == "" {
		return s, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, fmt.Errorf("reading state file %s: %w", path, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("parsing state file %s: %w", path, err)
	}
	if s.Triggered == nil {
		s.Triggered = make(map[string]time.Time)
	}
	return s, nil
}

// Save writes the state file to disk if Path is configured.
func (s *StateStore) Save() error {
	if s == nil || s.Path == "" {
		return nil
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.Path, append(data, '\n'), 0o644)
}

// NewRunner creates an action Runner.
func NewRunner(githubToken, defaultRepo string, dryRun bool, state *StateStore) *Runner {
	return &Runner{
		HTTPClient:    &http.Client{Timeout: 30 * time.Second},
		GitHubAPIBase: "https://api.github.com",
		GitHubToken:   githubToken,
		DefaultRepo:   defaultRepo,
		DryRun:        dryRun,
		State:         state,
	}
}

// Marker returns the hidden HTML marker used to deduplicate GitHub issues and comments.
func Marker(blockerID string) string {
	return fmt.Sprintf("<!-- fogcutter:id=%s -->", blockerID)
}

type templateContext struct {
	ID            string
	Description   string
	Location      string
	Resolution    string
	SourceSummary string
	Source        config.SourceConfig
}

func renderTemplate(tmplStr string, res checker.Result) (string, error) {
	if tmplStr == "" {
		return "", nil
	}
	t, err := template.New("action").Parse(tmplStr)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	ctx := templateContext{
		ID:            res.Blocker.ID,
		Description:   res.Blocker.Description,
		Location:      res.Blocker.Location,
		Resolution:    res.Resolution,
		SourceSummary: res.SourceSummary,
		Source:        res.Blocker.Source,
	}
	if err := t.Execute(&buf, ctx); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// ExecuteAll runs all configured on_resolved actions for a resolved blocker.
func (r *Runner) ExecuteAll(ctx context.Context, res checker.Result) []ExecutionRecord {
	var records []ExecutionRecord
	for _, act := range res.Blocker.OnResolved {
		rec := r.ExecuteOne(ctx, res, act)
		records = append(records, rec)
	}
	return records
}

// ExecuteOne executes a single ActionConfig for a resolved blocker.
func (r *Runner) ExecuteOne(ctx context.Context, res checker.Result, act config.ActionConfig) ExecutionRecord {
	stateKey := fmt.Sprintf("%s::%s", res.Blocker.ID, act.Type)
	if act.ShouldDeduplicate() && r.State != nil && r.State.Path != "" {
		if ts, ok := r.State.Triggered[stateKey]; ok {
			return ExecutionRecord{
				BlockerID: res.Blocker.ID,
				Type:      act.Type,
				Status:    "skipped-duplicate",
				Detail:    fmt.Sprintf("already recorded in state file at %s", ts.Format(time.RFC3339)),
			}
		}
	}

	var rec ExecutionRecord
	switch act.Type {
	case "create_issue":
		rec = r.runCreateIssue(ctx, res, act)
	case "comment_issue":
		rec = r.runCommentIssue(ctx, res, act)
	case "workflow_dispatch":
		rec = r.runWorkflowDispatch(ctx, res, act)
	case "repository_dispatch":
		rec = r.runRepositoryDispatch(ctx, res, act)
	case "command":
		rec = r.runCommand(ctx, res, act)
	default:
		rec = ExecutionRecord{
			BlockerID: res.Blocker.ID,
			Type:      act.Type,
			Status:    "error",
			Error:     fmt.Sprintf("unsupported action type %q", act.Type),
		}
	}

	if rec.Status == "executed" && r.State != nil {
		r.State.Triggered[stateKey] = time.Now().UTC()
	}
	return rec
}

func (r *Runner) targetRepo(act config.ActionConfig) string {
	if act.Repo != "" {
		return act.Repo
	}
	return r.DefaultRepo
}

func (r *Runner) runCreateIssue(ctx context.Context, res checker.Result, act config.ActionConfig) ExecutionRecord {
	repo := r.targetRepo(act)
	rec := ExecutionRecord{
		BlockerID: res.Blocker.ID,
		Type:      "create_issue",
		Target:    repo,
	}
	if repo == "" {
		rec.Status = "error"
		rec.Error = "create_issue requires target 'repo' (or top-level repository / $GITHUB_REPOSITORY)"
		return rec
	}

	titleTmpl := act.Title
	if titleTmpl == "" {
		titleTmpl = "Blocker resolved: {{.SourceSummary}}"
	}
	bodyTmpl := act.Body
	if bodyTmpl == "" {
		bodyTmpl = "External blocker **{{.ID}}** is now resolved.\n\n- **Source:** `{{.SourceSummary}}`\n- **Resolution:** {{.Resolution}}\n{{if .Description}}- **Description:** {{.Description}}\n{{end}}{{if .Location}}- **Location:** `{{.Location}}`\n{{end}}"
	}

	title, err := renderTemplate(titleTmpl, res)
	if err != nil {
		rec.Status = "error"
		rec.Error = fmt.Sprintf("rendering title template: %v", err)
		return rec
	}
	body, err := renderTemplate(bodyTmpl, res)
	if err != nil {
		rec.Status = "error"
		rec.Error = fmt.Sprintf("rendering body template: %v", err)
		return rec
	}
	marker := Marker(res.Blocker.ID)
	fullBody := body + "\n\n" + marker

	if act.ShouldDeduplicate() && (!r.DryRun || r.GitHubToken != "") {
		existingURL, found, err := r.findExistingIssueByMarker(ctx, repo, marker)
		if err != nil {
			if !r.DryRun {
				rec.Status = "error"
				rec.Error = fmt.Sprintf("checking existing issues: %v", err)
				return rec
			}
		} else if found {
			rec.Status = "skipped-duplicate"
			rec.Detail = fmt.Sprintf("issue already exists: %s", existingURL)
			return rec
		}
	}

	if r.DryRun {
		rec.Status = "dry-run"
		rec.Detail = fmt.Sprintf("would create issue %q in %s", title, repo)
		return rec
	}

	payload := map[string]any{
		"title": title,
		"body":  fullBody,
	}
	if len(act.Labels) > 0 {
		payload["labels"] = act.Labels
	}
	if len(act.Assignees) > 0 {
		payload["assignees"] = act.Assignees
	}

	u := fmt.Sprintf("%s/repos/%s/issues", strings.TrimRight(r.GitHubAPIBase, "/"), repo)
	var created struct {
		Number  int    `json:"number"`
		HTMLURL string `json:"html_url"`
	}
	if err := r.doGitHubJSON(ctx, http.MethodPost, u, payload, &created); err != nil {
		rec.Status = "error"
		rec.Error = err.Error()
		return rec
	}

	rec.Status = "executed"
	rec.Detail = fmt.Sprintf("created issue [%s#%d: %s](%s)", repo, created.Number, title, created.HTMLURL)
	return rec
}

func (r *Runner) runCommentIssue(ctx context.Context, res checker.Result, act config.ActionConfig) ExecutionRecord {
	repo := r.targetRepo(act)
	target := fmt.Sprintf("%s#%d", repo, act.Issue)
	rec := ExecutionRecord{
		BlockerID: res.Blocker.ID,
		Type:      "comment_issue",
		Target:    target,
	}
	if repo == "" || act.Issue <= 0 {
		rec.Status = "error"
		rec.Error = "comment_issue requires target 'repo' and positive 'issue' number"
		return rec
	}

	bodyTmpl := act.Body
	if bodyTmpl == "" {
		bodyTmpl = "External blocker **{{.ID}}** (`{{.SourceSummary}}`) is now resolved: {{.Resolution}}"
	}
	body, err := renderTemplate(bodyTmpl, res)
	if err != nil {
		rec.Status = "error"
		rec.Error = fmt.Sprintf("rendering comment template: %v", err)
		return rec
	}
	marker := Marker(res.Blocker.ID)
	fullBody := body + "\n\n" + marker

	if act.ShouldSkipIfClosed() && (!r.DryRun || r.GitHubToken != "") {
		isClosed, err := r.isIssueClosed(ctx, repo, act.Issue)
		if err == nil && isClosed {
			rec.Status = "skipped-closed"
			rec.Detail = fmt.Sprintf("target issue %s is already closed", target)
			return rec
		}
	}

	if act.ShouldDeduplicate() && (!r.DryRun || r.GitHubToken != "") {
		existingURL, found, err := r.findExistingCommentByMarker(ctx, repo, act.Issue, marker)
		if err != nil {
			if !r.DryRun {
				rec.Status = "error"
				rec.Error = fmt.Sprintf("checking existing comments: %v", err)
				return rec
			}
		} else if found {
			rec.Status = "skipped-duplicate"
			rec.Detail = fmt.Sprintf("comment already exists on %s: %s", target, existingURL)
			return rec
		}
	}

	if r.DryRun {
		rec.Status = "dry-run"
		rec.Detail = fmt.Sprintf("would comment on %s", target)
		return rec
	}

	u := fmt.Sprintf("%s/repos/%s/issues/%d/comments", strings.TrimRight(r.GitHubAPIBase, "/"), repo, act.Issue)
	var created struct {
		HTMLURL string `json:"html_url"`
	}
	if err := r.doGitHubJSON(ctx, http.MethodPost, u, map[string]any{"body": fullBody}, &created); err != nil {
		rec.Status = "error"
		rec.Error = err.Error()
		return rec
	}

	if len(act.Labels) > 0 {
		labelsURL := fmt.Sprintf("%s/repos/%s/issues/%d/labels", strings.TrimRight(r.GitHubAPIBase, "/"), repo, act.Issue)
		_ = r.doGitHubJSON(ctx, http.MethodPost, labelsURL, map[string]any{"labels": act.Labels}, nil)
	}
	for _, rmLabel := range act.RemoveLabels {
		rmURL := fmt.Sprintf("%s/repos/%s/issues/%d/labels/%s", strings.TrimRight(r.GitHubAPIBase, "/"), repo, act.Issue, rmLabel)
		_ = r.doGitHubJSON(ctx, http.MethodDelete, rmURL, nil, nil)
	}

	rec.Status = "executed"
	rec.Detail = fmt.Sprintf("commented on [%s](%s)", target, created.HTMLURL)
	return rec
}

func (r *Runner) isIssueClosed(ctx context.Context, repo string, issueNum int) (bool, error) {
	u := fmt.Sprintf("%s/repos/%s/issues/%d", strings.TrimRight(r.GitHubAPIBase, "/"), repo, issueNum)
	var iss struct {
		State string `json:"state"`
	}
	if err := r.doGitHubJSON(ctx, http.MethodGet, u, nil, &iss); err != nil {
		return false, err
	}
	return iss.State == "closed", nil
}

func (r *Runner) runWorkflowDispatch(ctx context.Context, res checker.Result, act config.ActionConfig) ExecutionRecord {
	repo := r.targetRepo(act)
	ref := act.Ref
	if ref == "" {
		ref = "main"
	}
	target := fmt.Sprintf("%s:%s@%s", repo, act.Workflow, ref)
	rec := ExecutionRecord{
		BlockerID: res.Blocker.ID,
		Type:      "workflow_dispatch",
		Target:    target,
	}
	if repo == "" || act.Workflow == "" {
		rec.Status = "error"
		rec.Error = "workflow_dispatch requires 'repo' and 'workflow'"
		return rec
	}

	renderedInputs := make(map[string]string, len(act.Inputs))
	for k, v := range act.Inputs {
		val, err := renderTemplate(v, res)
		if err != nil {
			rec.Status = "error"
			rec.Error = fmt.Sprintf("rendering workflow input %q: %v", k, err)
			return rec
		}
		renderedInputs[k] = val
	}

	if r.DryRun {
		rec.Status = "dry-run"
		rec.Detail = fmt.Sprintf("would dispatch workflow %s with inputs %v", target, renderedInputs)
		return rec
	}

	u := fmt.Sprintf("%s/repos/%s/actions/workflows/%s/dispatches", strings.TrimRight(r.GitHubAPIBase, "/"), repo, act.Workflow)
	payload := map[string]any{
		"ref":    ref,
		"inputs": renderedInputs,
	}
	if err := r.doGitHubJSON(ctx, http.MethodPost, u, payload, nil); err != nil {
		rec.Status = "error"
		rec.Error = err.Error()
		return rec
	}

	rec.Status = "executed"
	rec.Detail = fmt.Sprintf("dispatched workflow %s", target)
	return rec
}

func (r *Runner) runRepositoryDispatch(ctx context.Context, res checker.Result, act config.ActionConfig) ExecutionRecord {
	repo := r.targetRepo(act)
	eventType := act.EventType
	if eventType == "" {
		eventType = "blocker_resolved"
	}
	target := fmt.Sprintf("%s (%s)", repo, eventType)
	rec := ExecutionRecord{
		BlockerID: res.Blocker.ID,
		Type:      "repository_dispatch",
		Target:    target,
	}
	if repo == "" {
		rec.Status = "error"
		rec.Error = "repository_dispatch requires 'repo'"
		return rec
	}

	if r.DryRun {
		rec.Status = "dry-run"
		rec.Detail = fmt.Sprintf("would send repository_dispatch %q to %s", eventType, repo)
		return rec
	}

	u := fmt.Sprintf("%s/repos/%s/dispatches", strings.TrimRight(r.GitHubAPIBase, "/"), repo)
	payload := map[string]any{
		"event_type": eventType,
		"client_payload": map[string]any{
			"blocker_id":     res.Blocker.ID,
			"description":    res.Blocker.Description,
			"location":       res.Blocker.Location,
			"resolution":     res.Resolution,
			"source_summary": res.SourceSummary,
		},
	}
	if err := r.doGitHubJSON(ctx, http.MethodPost, u, payload, nil); err != nil {
		rec.Status = "error"
		rec.Error = err.Error()
		return rec
	}

	rec.Status = "executed"
	rec.Detail = fmt.Sprintf("dispatched repository event %q to %s", eventType, repo)
	return rec
}

func (r *Runner) runCommand(ctx context.Context, res checker.Result, act config.ActionConfig) ExecutionRecord {
	rec := ExecutionRecord{
		BlockerID: res.Blocker.ID,
		Type:      "command",
		Target:    act.Command,
	}
	if act.Command == "" {
		rec.Status = "error"
		rec.Error = "command action requires 'command'"
		return rec
	}

	cmdStr, err := renderTemplate(act.Command, res)
	if err != nil {
		rec.Status = "error"
		rec.Error = fmt.Sprintf("rendering command template: %v", err)
		return rec
	}
	rec.Target = cmdStr

	if r.DryRun {
		rec.Status = "dry-run"
		rec.Detail = fmt.Sprintf("would execute command: %s", cmdStr)
		return rec
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	env := os.Environ()
	env = append(env,
		"FOGCUTTER_ID="+res.Blocker.ID,
		"FOGCUTTER_DESCRIPTION="+res.Blocker.Description,
		"FOGCUTTER_LOCATION="+res.Blocker.Location,
		"FOGCUTTER_RESOLUTION="+res.Resolution,
		"FOGCUTTER_SOURCE="+res.SourceSummary,
	)
	for k, v := range act.Env {
		renderedVal, err := renderTemplate(v, res)
		if err != nil {
			renderedVal = v
		}
		env = append(env, fmt.Sprintf("%s=%s", k, renderedVal))
	}
	cmd.Env = env

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		rec.Status = "error"
		rec.Error = fmt.Sprintf("command failed (%v): %s", err, strings.TrimSpace(out.String()))
		return rec
	}

	rec.Status = "executed"
	rec.Detail = strings.TrimSpace(out.String())
	if rec.Detail == "" {
		rec.Detail = "command completed with exit code 0"
	}
	return rec
}

func (r *Runner) findExistingIssueByMarker(ctx context.Context, repo, marker string) (string, bool, error) {
	u := fmt.Sprintf("%s/repos/%s/issues?state=all&per_page=100", strings.TrimRight(r.GitHubAPIBase, "/"), repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", false, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if r.GitHubToken != "" {
		req.Header.Set("Authorization", "Bearer "+r.GitHubToken)
	}

	resp, err := r.HTTPClient.Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("listing issues in %s returned HTTP %d", repo, resp.StatusCode)
	}

	var issues []struct {
		Body    string `json:"body"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&issues); err != nil {
		return "", false, err
	}
	for _, iss := range issues {
		if strings.Contains(iss.Body, marker) {
			return iss.HTMLURL, true, nil
		}
	}
	return "", false, nil
}

func (r *Runner) findExistingCommentByMarker(ctx context.Context, repo string, issueNum int, marker string) (string, bool, error) {
	u := fmt.Sprintf("%s/repos/%s/issues/%d/comments?per_page=100", strings.TrimRight(r.GitHubAPIBase, "/"), repo, issueNum)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", false, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if r.GitHubToken != "" {
		req.Header.Set("Authorization", "Bearer "+r.GitHubToken)
	}

	resp, err := r.HTTPClient.Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("listing comments on %s#%d returned HTTP %d", repo, issueNum, resp.StatusCode)
	}

	var comments []struct {
		Body    string `json:"body"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&comments); err != nil {
		return "", false, err
	}
	for _, c := range comments {
		if strings.Contains(c.Body, marker) {
			return c.HTMLURL, true, nil
		}
	}
	return "", false, nil
}

func (r *Runner) doGitHubJSON(ctx context.Context, method, urlStr string, payload any, out any) error {
	var bodyReader io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, urlStr, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if r.GitHubToken != "" {
		req.Header.Set("Authorization", "Bearer "+r.GitHubToken)
	}

	resp, err := r.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("GitHub API %s %s returned HTTP %d: %s", method, urlStr, resp.StatusCode, strings.TrimSpace(string(msg)))
	}

	if out != nil && resp.StatusCode != http.StatusNoContent {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}
