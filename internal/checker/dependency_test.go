// SPDX-FileCopyrightText: 2026 Daniel Arndt <dan@arndt.ca>
// SPDX-License-Identifier: Apache-2.0

package checker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DanielArndt/fogcutter/internal/config"
)

// fakeUpstream serves an authentik-like repo: two release series, where only the
// newer one ships the fixed golang.org/x/net.
func fakeUpstream(t *testing.T) *Evaluator {
	t.Helper()
	gomods := map[string]string{
		"version/2026.5.8": "module goauthentik.io\n\nrequire (\n\tgolang.org/x/net v0.52.0 // indirect\n)\n",
		"version/2026.8.5": "module goauthentik.io\n\nrequire (\n\tgolang.org/x/net v0.53.0 // indirect\n)\n",
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/goauthentik/authentik/releases", func(w http.ResponseWriter, r *http.Request) {
		// Published order: the 2026.5.8 backport is newer by date than 2026.8.5.
		_, _ = w.Write([]byte(`[
			{"tag_name": "version/2026.5.8"},
			{"tag_name": "version/2026.8.5"},
			{"tag_name": "version/2026.11.0-rc1", "prerelease": true}
		]`))
	})
	mux.HandleFunc("/repos/goauthentik/authentik/contents/go.mod", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/vnd.github.raw" {
			http.Error(w, "want raw accept header", http.StatusBadRequest)
			return
		}
		body, ok := gomods[r.URL.Query().Get("ref")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	e := NewEvaluator("")
	e.GitHubAPIBase = srv.URL
	return e
}

func depBlocker(src config.SourceConfig) config.Blocker {
	src.Type = "dependency"
	src.Repo = "goauthentik/authentik"
	src.Manifest = "go.mod"
	src.Package = "golang.org/x/net"
	if src.Version == "" {
		src.Version = ">=v0.53.0"
	}
	return config.Blocker{ID: "x-net", Source: src}
}

func TestDependencyPinnedRef(t *testing.T) {
	e := fakeUpstream(t)
	ctx := context.Background()

	r := e.Check(ctx, depBlocker(config.SourceConfig{Ref: "version/2026.5.8"}))
	if r.Error != "" || r.Resolved {
		t.Fatalf("2026.5.8 should still need the override: %+v", r)
	}
	if !strings.Contains(r.Resolution, "v0.52.0") {
		t.Errorf("resolution should report the upstream version, got %q", r.Resolution)
	}

	r = e.Check(ctx, depBlocker(config.SourceConfig{Ref: "version/2026.8.5"}))
	if r.Error != "" || !r.Resolved {
		t.Fatalf("2026.8.5 ships v0.53.0, override is redundant: %+v", r)
	}
}

func TestDependencyLatestPicksHighestVersion(t *testing.T) {
	e := fakeUpstream(t)
	r := e.Check(context.Background(), depBlocker(config.SourceConfig{}))
	if r.Error != "" || !r.Resolved || !strings.Contains(r.Resolution, "2026.8.5") {
		t.Fatalf("latest should be 2026.8.5 (highest, not newest-by-date, no prereleases): %+v", r)
	}
}

func TestDependencyRefFromLocalFile(t *testing.T) {
	e := fakeUpstream(t)
	dir := t.TempDir()
	rock := filepath.Join(dir, "rockcraft.yaml")
	if err := os.WriteFile(rock, []byte("parts:\n  go-server:\n    source-tag: version/2026.5.8\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b := depBlocker(config.SourceConfig{RefFile: rock, RefRegex: `source-tag: (\S+)`})

	r := e.Check(context.Background(), b)
	if r.Error != "" || r.Resolved {
		t.Fatalf("rock builds 2026.5.8, override still needed: %+v", r)
	}

	// Renovate bumps the source-tag; the same blocker now resolves.
	if err := os.WriteFile(rock, []byte("parts:\n  go-server:\n    source-tag: version/2026.8.5\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r = e.Check(context.Background(), b)
	if r.Error != "" || !r.Resolved {
		t.Fatalf("rock now builds 2026.8.5, override is redundant: %+v", r)
	}
}

func TestDependencyMissingPackageIsNotResolved(t *testing.T) {
	e := fakeUpstream(t)
	b := depBlocker(config.SourceConfig{Ref: "version/2026.8.5"})
	b.Source.Package = "golang.org/x/crypto"
	r := e.Check(context.Background(), b)
	if r.Error != "" || r.Resolved || !strings.Contains(r.Resolution, "not found") {
		t.Fatalf("missing package should be reported, not resolved: %+v", r)
	}
}

func TestDependencyValidation(t *testing.T) {
	b := depBlocker(config.SourceConfig{RefFile: "rockcraft.yaml"})
	if err := b.Validate(); err == nil {
		t.Fatal("ref_file without ref_regex should be rejected")
	}
	b = config.Blocker{ID: "x", Source: config.SourceConfig{Type: "dep", Repo: "a/b"}}
	if err := b.Validate(); err == nil {
		t.Fatal("missing manifest/package/version should be rejected")
	}
}
