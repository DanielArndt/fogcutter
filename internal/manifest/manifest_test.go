// SPDX-FileCopyrightText: 2026 Daniel Arndt <dan@arndt.ca>
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"reflect"
	"testing"
)

func TestGoMod(t *testing.T) {
	data := []byte(`module goauthentik.io

go 1.26.0

require github.com/gorilla/mux v1.8.1

require (
	golang.org/x/net v0.52.0 // indirect
	golang.org/x/text v0.35.0 // indirect
)

replace golang.org/x/text => golang.org/x/text v0.36.0
`)
	cases := map[string][]string{
		"golang.org/x/net":       {"v0.52.0"},
		"github.com/gorilla/mux": {"v1.8.1"},
		"golang.org/x/text":      {"v0.36.0"}, // replace wins
		"golang.org/x/crypto":    nil,
		"golang.org/x/net/http2": nil, // no prefix matching
	}
	for pkg, want := range cases {
		got, err := Versions("go.mod", data, pkg)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %v, want %v", pkg, got, want)
		}
	}
}

func TestUVLock(t *testing.T) {
	data := []byte(`version = 1
requires-python = ">=3.14"

[[package]]
name = "anyio"
version = "4.13.0"
source = { registry = "https://pypi.org/simple" }
dependencies = [
    { name = "idna" },
]
wheels = [
    { url = "https://example/anyio.whl", hash = "sha256:00" },
]

[package.metadata]
requires-dist = [{ name = "idna", specifier = ">=2.8" }]

[[package]]
name = "PyJWT"
version = "2.12.1"
source = { registry = "https://pypi.org/simple" }

[[package]]
name = "idna"
version = "3.13"
`)
	for pkg, want := range map[string][]string{
		"anyio": {"4.13.0"},
		"pyjwt": {"2.12.1"}, // PEP 503 normalisation
		"idna":  {"3.13"},
		"h2":    nil,
	} {
		got, err := Versions("uv.lock", data, pkg)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %v, want %v", pkg, got, want)
		}
	}
}

func TestCargoLockMultipleVersions(t *testing.T) {
	data := []byte(`[[package]]
name = "rustls"
version = "0.21.12"

[[package]]
name = "rustls"
version = "0.23.31"
`)
	got, _ := Versions("Cargo.lock", data, "rustls")
	if want := []string{"0.21.12", "0.23.31"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestPackageLock(t *testing.T) {
	data := []byte(`{"lockfileVersion": 3, "packages": {
		"": {"name": "web"},
		"node_modules/vite": {"version": "7.1.2"},
		"node_modules/foo/node_modules/vite": {"version": "6.0.0"},
		"node_modules/vite-plugin": {"version": "1.0.0"}
	}}`)
	got, err := Versions("web/package-lock.json", data, "vite")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("got %v, want two vite copies", got)
	}
}

func TestRequirements(t *testing.T) {
	data := []byte("Django==5.2.17 \\\n    --hash=sha256:00\nurllib3[socks]==2.6.3 ; python_version >= '3.9'\n# idna==1.0\n")
	for pkg, want := range map[string][]string{"django": {"5.2.17"}, "urllib3": {"2.6.3"}, "idna": nil} {
		got, _ := Versions("requirements.txt", data, pkg)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %v, want %v", pkg, got, want)
		}
	}
}

func TestUnsupported(t *testing.T) {
	if _, err := Versions("Gemfile.lock", nil, "rails"); err == nil {
		t.Fatal("expected error for unsupported manifest")
	}
}
