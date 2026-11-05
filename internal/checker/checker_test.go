// SPDX-FileCopyrightText: 2026 Daniel Arndt <dan@arndt.ca>
// SPDX-License-Identifier: Apache-2.0

package checker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DanielArndt/fogcutter/internal/config"
)

func TestEvaluators(t *testing.T) {
	mux := http.NewServeMux()

	// Mock GitHub Issue (closed)
	mux.HandleFunc("/repos/canonical/kratos/issues/364", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"state": "closed",
			"state_reason": "completed",
			"title": "Fix multiStatements handling",
			"html_url": "https://github.com/canonical/kratos/issues/364"
		}`))
	})

	// Mock GitHub Issue (still open)
	mux.HandleFunc("/repos/canonical/kratos/issues/365", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"state": "open",
			"title": "WIP feature",
			"html_url": "https://github.com/canonical/kratos/issues/365"
		}`))
	})

	// Mock GitHub Releases
	mux.HandleFunc("/repos/openfga/openfga/releases", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"tag_name": "v1.8.2", "name": "v1.8.2", "draft": false, "prerelease": false, "html_url": "https://github.com/openfga/openfga/releases/tag/v1.8.2"}
		]`))
	})

	// Mock Snapcraft v2 info endpoint
	mux.HandleFunc("/v2/snaps/info/lxd", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Snap-Device-Series") != "16" {
			http.Error(w, "missing header", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"name": "lxd",
			"channel-map": [
				{
					"channel": {"architecture": "amd64", "name": "5.21/stable", "track": "5.21", "risk": "stable"},
					"revision": 29100,
					"version": "5.21.2"
				}
			]
		}`))
	})

	// Mock custom HTTP JSON endpoint
	mux.HandleFunc("/status.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"service": {"version": "2.4.1", "ready": true}}`))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	eval := NewEvaluator("test-token")
	eval.GitHubAPIBase = srv.URL
	eval.SnapAPIBase = srv.URL

	ctx := context.Background()

	// 1. Closed GitHub issue -> resolved
	r1 := eval.Check(ctx, config.Blocker{
		ID: "kratos-364",
		Source: config.SourceConfig{
			Type:  "github-issue",
			Repo:  "canonical/kratos",
			Issue: 364,
		},
	})
	if !r1.Resolved || r1.Error != "" {
		t.Fatalf("expected kratos-364 resolved, got %+v", r1)
	}

	// 2. Open GitHub issue -> not resolved
	r2 := eval.Check(ctx, config.Blocker{
		ID: "kratos-365",
		Source: config.SourceConfig{
			Type:  "github-issue",
			Repo:  "canonical/kratos",
			Issue: 365,
		},
	})
	if r2.Resolved || r2.Error != "" {
		t.Fatalf("expected kratos-365 unresolved, got %+v", r2)
	}

	// 3. GitHub release >= v1.8.0 -> resolved
	r3 := eval.Check(ctx, config.Blocker{
		ID: "openfga-1.8",
		Source: config.SourceConfig{
			Type:    "github-release",
			Repo:    "openfga/openfga",
			Version: ">=v1.8.0",
		},
	})
	if !r3.Resolved || r3.Error != "" {
		t.Fatalf("expected openfga-1.8 resolved, got %+v", r3)
	}

	// 4. Snapcraft channel & version -> resolved
	r4 := eval.Check(ctx, config.Blocker{
		ID: "lxd-snap",
		Source: config.SourceConfig{
			Type:     "snapcraft",
			Snap:     "lxd",
			Channel:  "5.21/stable",
			Version:  ">=5.21.1",
			Revision: ">=29000",
		},
	})
	if !r4.Resolved || r4.Error != "" {
		t.Fatalf("expected lxd-snap resolved, got %+v", r4)
	}

	// 5. Generic HTTP JSON check -> resolved
	r5 := eval.Check(ctx, config.Blocker{
		ID: "http-json",
		Source: config.SourceConfig{
			Type:     "http",
			URL:      srv.URL + "/status.json",
			JSONPath: "service.version",
			Version:  ">=2.4.0",
		},
	})
	if !r5.Resolved || r5.Error != "" {
		t.Fatalf("expected http-json resolved, got %+v", r5)
	}

	// 6. Command check -> resolved
	r6 := eval.Check(ctx, config.Blocker{
		ID: "cmd-test",
		Source: config.SourceConfig{
			Type:    "command",
			Command: "echo 3.2.1",
			Version: ">=3.2.0",
		},
	})
	if !r6.Resolved || r6.Error != "" {
		t.Fatalf("expected cmd-test resolved, got %+v", r6)
	}
}
