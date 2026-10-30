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
