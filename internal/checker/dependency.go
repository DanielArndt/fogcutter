// SPDX-FileCopyrightText: 2026 Daniel Arndt <dan@arndt.ca>
// SPDX-License-Identifier: Apache-2.0

package checker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/DanielArndt/fogcutter/internal/config"
	"github.com/DanielArndt/fogcutter/internal/manifest"
	"github.com/DanielArndt/fogcutter/internal/version"
)

// checkDependency resolves when every copy of src.Package in src.Manifest of
// src.Repo at the chosen ref satisfies src.Version. The typical blocker is a
// local override such as `go get golang.org/x/net@v0.53.0` for a CVE: once the
// upstream release you build pins that version itself, the override is dead
// weight and can be dropped.
func (e *Evaluator) checkDependency(ctx context.Context, src config.SourceConfig) (bool, string, error) {
	ref, err := e.resolveDependencyRef(ctx, src)
	if err != nil {
		return false, "", err
	}

	data, err := e.fetchRepoFile(ctx, src.Repo, ref, src.Manifest)
	if err != nil {
		return false, "", err
	}

	versions, err := manifest.Versions(src.Manifest, data, src.Package)
	if err != nil {
		return false, "", err
	}
	where := fmt.Sprintf("%s@%s:%s", src.Repo, ref, src.Manifest)
	if len(versions) == 0 {
		// Upstream dropped the dependency entirely; an override can no longer be needed
		// for it, but that is worth a human look rather than a silent "resolved".
		return false, fmt.Sprintf("%s not found in %s (dependency removed upstream?)", src.Package, where), nil
	}

	for _, v := range versions {
		ok, err := version.Match(v, src.Version)
		if err != nil {
			return false, "", err
		}
		if !ok {
			return false, fmt.Sprintf("%s pins %s %s (waiting for %s)", where, src.Package, strings.Join(versions, ", "), src.Version), nil
		}
	}
	return true, fmt.Sprintf("%s pins %s %s, which satisfies %s", where, src.Package, strings.Join(versions, ", "), src.Version), nil
}

func (e *Evaluator) resolveDependencyRef(ctx context.Context, src config.SourceConfig) (string, error) {
	if src.RefFile != "" {
		data, err := os.ReadFile(src.RefFile)
		if err != nil {
			return "", fmt.Errorf("reading ref_file: %w", err)
		}
		re, err := regexp.Compile(src.RefRegex)
		if err != nil {
			return "", fmt.Errorf("invalid ref_regex %q: %w", src.RefRegex, err)
		}
		m := re.FindSubmatch(data)
		if m == nil {
			return "", fmt.Errorf("ref_regex %q did not match %s", src.RefRegex, src.RefFile)
		}
		if len(m) > 1 {
			return string(m[1]), nil
		}
		return string(m[0]), nil
	}
	if src.Ref != "" && src.Ref != "latest" {
		return src.Ref, nil
	}
	return e.latestReleaseTag(ctx, src.Repo, src.IncludePre)
}

// latestReleaseTag returns the highest-versioned release tag rather than the most
// recently published one: projects that maintain several series (authentik ships
// 2026.5.x backports after 2026.8.0) would otherwise flip between series.
func (e *Evaluator) latestReleaseTag(ctx context.Context, repo string, includePre bool) (string, error) {
	u := fmt.Sprintf("%s/repos/%s/releases?per_page=50", strings.TrimRight(e.GitHubAPIBase, "/"), repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	e.githubHeaders(req, "application/vnd.github+json")
	resp, err := e.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github releases API %s returned HTTP %d", u, resp.StatusCode)
	}
	var releases []struct {
		TagName    string `json:"tag_name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return "", err
	}
	best := ""
	for _, r := range releases {
		if r.Draft || (r.Prerelease && !includePre) {
			continue
		}
		if best == "" || version.Compare(tagVersion(r.TagName), tagVersion(best)) > 0 {
			best = r.TagName
		}
	}
	if best == "" {
		return "", fmt.Errorf("no releases found for %s", repo)
	}
	return best, nil
}

// tagVersion strips a non-numeric tag prefix such as "version/" or "release-".
func tagVersion(tag string) string {
	if i := strings.IndexAny(tag, "0123456789"); i > 0 {
		return tag[i:]
	}
	return tag
}

func (e *Evaluator) fetchRepoFile(ctx context.Context, repo, ref, path string) ([]byte, error) {
	escaped := make([]string, 0)
	for _, seg := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		escaped = append(escaped, url.PathEscape(seg))
	}
	u := fmt.Sprintf("%s/repos/%s/contents/%s?ref=%s",
		strings.TrimRight(e.GitHubAPIBase, "/"), repo, strings.Join(escaped, "/"), url.QueryEscape(ref))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	e.githubHeaders(req, "application/vnd.github.raw")
	resp, err := e.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching %s from %s@%s: HTTP %d", path, repo, ref, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 64*1024*1024))
}

func (e *Evaluator) githubHeaders(req *http.Request, accept string) {
	req.Header.Set("Accept", accept)
	if e.GitHubToken != "" {
		req.Header.Set("Authorization", "Bearer "+e.GitHubToken)
	}
}
