package action

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/DanielArndt/fogcutter/internal/checker"
	"github.com/DanielArndt/fogcutter/internal/config"
)

func TestRunnerActionsAndDeduplication(t *testing.T) {
	var mu sync.Mutex
	var createdIssues []map[string]any
	var createdComments []map[string]any
	var dispatchedWorkflows int

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/canonical/authz/issues", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(createdIssues)
			return
		}
		if r.Method == http.MethodPost {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			body["number"] = len(createdIssues) + 1
			body["html_url"] = "https://github.com/canonical/authz/issues/1"
			createdIssues = append(createdIssues, body)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(body)
			return
		}
	})

	mux.HandleFunc("/repos/canonical/authz/issues/42/comments", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(createdComments)
			return
		}
		if r.Method == http.MethodPost {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			body["html_url"] = "https://github.com/canonical/authz/issues/42#issuecomment-1"
			createdComments = append(createdComments, body)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(body)
			return
		}
	})

	mux.HandleFunc("/repos/canonical/authz/actions/workflows/agent-fix.yml/dispatches", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		dispatchedWorkflows++
		w.WriteHeader(http.StatusNoContent)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	runner := NewRunner("test-token", "canonical/authz", false, nil)
	runner.GitHubAPIBase = srv.URL

	res := checker.Result{
		Blocker: config.Blocker{
			ID:          "kratos-364",
			Description: "Remove DSN workaround",
			Location:    "internal/db/conn.go:28",
			Source: config.SourceConfig{
				Type:  "github-issue",
				Repo:  "ory/kratos",
				Issue: 364,
			},
			OnResolved: []config.ActionConfig{
				{
					Type:   "create_issue",
					Title:  "Unblocked: {{.ID}}",
					Labels: []string{"tech-debt"},
				},
				{
					Type:  "comment_issue",
					Issue: 42,
					Body:  "@claude blocker {{.ID}} is resolved ({{.Resolution}}), please open a PR.",
				},
				{
					Type:     "workflow_dispatch",
					Workflow: "agent-fix.yml",
					Ref:      "main",
					Inputs:   map[string]string{"blocker_id": "{{.ID}}"},
				},
				{
					Type:    "command",
					Command: "echo resolved {{.ID}}",
				},
			},
		},
		Resolved:      true,
		Resolution:    "[ory/kratos#364: Fix DSN](https://github.com/ory/kratos/issues/364) is closed",
		SourceSummary: "ory/kratos#364",
	}

	ctx := context.Background()
	records := runner.ExecuteAll(ctx, res)
	if len(records) != 4 {
		t.Fatalf("got %d records, want 4", len(records))
	}
	for _, rec := range records {
		if rec.Status != "executed" {
			t.Errorf("expected action %s to be executed, got %+v", rec.Type, rec)
		}
	}

	// Second run: create_issue and comment_issue must be skipped as duplicates!
	records2 := runner.ExecuteAll(ctx, res)
	if records2[0].Status != "skipped-duplicate" {
		t.Errorf("expected create_issue to be skipped-duplicate on second run, got %+v", records2[0])
	}
	if records2[1].Status != "skipped-duplicate" {
		t.Errorf("expected comment_issue to be skipped-duplicate on second run, got %+v", records2[1])
	}
}
