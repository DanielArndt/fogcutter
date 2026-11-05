package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseDependencySpec(t *testing.T) {
	src, err := ParseSpec("dep:goauthentik/authentik@version/2026.5.8:go.mod:golang.org/x/net>=v0.53.0")
	if err != nil {
		t.Fatal(err)
	}
	if src.Type != "dependency" || src.Repo != "goauthentik/authentik" || src.Ref != "version/2026.5.8" ||
		src.Manifest != "go.mod" || src.Package != "golang.org/x/net" || src.Version != ">=v0.53.0" {
		t.Fatalf("unexpected parse: %+v", src)
	}

	src, err = ParseSpec("dep:goauthentik/authentik:web/package-lock.json:vite>=7.1.5")
	if err != nil {
		t.Fatal(err)
	}
	if src.Ref != "" || src.Manifest != "web/package-lock.json" || src.Package != "vite" {
		t.Fatalf("unexpected parse: %+v", src)
	}

	for _, bad := range []string{
		"dep:goauthentik/authentik:go.mod:golang.org/x/net", // no constraint
		"dep:goauthentik/authentik:golang.org/x/net>=1.0",   // no manifest
	} {
		if _, err := ParseSpec(bad); err == nil {
			t.Errorf("ParseSpec(%q) should fail", bad)
		}
	}
}

func TestScanRockcraftDependencyAnnotation(t *testing.T) {
	dir := t.TempDir()
	rock := []byte(`    override-build: |
      # BLOCKER(dep:goauthentik/authentik:go.mod:golang.org/x/net>=v0.53.0): drop x/net override (CVE-2026-0001)
      go get golang.org/x/net@v0.53.0
`)
	if err := os.WriteFile(filepath.Join(dir, "rockcraft.yaml"), rock, 0o644); err != nil {
		t.Fatal(err)
	}
	found, err := ScanFiles([]string{dir}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 {
		t.Fatalf("want 1 blocker, got %d", len(found))
	}
	b := found[0]
	if b.Source.Type != "dependency" || b.Source.Package != "golang.org/x/net" || b.Description != "drop x/net override (CVE-2026-0001)" {
		t.Fatalf("unexpected blocker: %+v", b)
	}
}

func TestDependencyRefFromEnclosingPart(t *testing.T) {
	rock := `name: authentik-server
version: "2026.5.8"
parts:
  web-ui:
    plugin: nil
    source: https://github.com/goauthentik/authentik
    source-tag: version/2026.4.0
  go-server:
    plugin: go
    source: https://github.com/goauthentik/authentik.git
    source-type: git
    source-tag: "version/2026.5.8"
    build-environment:
      - CGO_ENABLED: "1"
    override-build: |
      go mod download
      # BLOCKER(dep:goauthentik/authentik:go.mod:golang.org/x/net>=v0.62.0): x/net
      go get golang.org/x/net@v0.62.0
      # BLOCKER(dep:goauthentik/authentik@latest:go.mod:golang.org/x/sys>=v0.44.0): x/sys
      go get golang.org/x/sys@v0.44.0
      # BLOCKER(dep:goauthentik/authentik@version/2026.8.5:go.mod:golang.org/x/text>=v0.41.0): x/text
      go get golang.org/x/text@v0.41.0
  python-deps:
    # BLOCKER(dep:goauthentik/authentik:uv.lock:urllib3>=2.7.0): urllib3, tag declared below
    override-build: |
      uv pip install "urllib3>=2.7.0"
    source: https://github.com/goauthentik/authentik
    source-tag: version/2026.5.9
  other:
    source: https://github.com/someone/else
    source-tag: v1.0.0
    override-build: |
      # BLOCKER(dep:goauthentik/authentik:go.mod:golang.org/x/crypto>=v0.56.0): wrong repo
      go get golang.org/x/crypto@v0.56.0
  no-source:
    plugin: nil
    override-build: |
      # BLOCKER(dep:goauthentik/authentik:go.mod:github.com/klauspost/compress>=v1.18.7): no tag
      true
`
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "rockcraft.yaml"), []byte(rock), 0o644); err != nil {
		t.Fatal(err)
	}
	// A Go file next to it must not pick up the rock's tag.
	goSrc := "package x\n\n// BLOCKER(dep:goauthentik/authentik:go.mod:golang.org/x/net>=v0.62.0): source file\n"
	if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte(goSrc), 0o644); err != nil {
		t.Fatal(err)
	}

	found, err := ScanFiles([]string{dir}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, b := range found {
		got[b.Description] = b.Source.Ref
	}
	want := map[string]string{
		"x/net":                       "version/2026.5.8", // quoted tag, own part, not web-ui's
		"x/sys":                       "latest",           // explicit opt-out
		"x/text":                      "version/2026.8.5", // explicit ref kept
		"urllib3, tag declared below": "version/2026.5.9", // part keys after the annotation
		"wrong repo":                  "",                 // part builds another repo
		"no tag":                      "",
		"source file":                 "",
	}
	for desc, ref := range want {
		r, ok := got[desc]
		if !ok {
			t.Errorf("%q: blocker not found", desc)
			continue
		}
		if r != ref {
			t.Errorf("%q: ref = %q, want %q", desc, r, ref)
		}
	}
}
