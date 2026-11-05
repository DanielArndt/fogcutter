// Package manifest extracts resolved dependency versions from lockfiles and
// module manifests (go.mod, uv.lock, poetry.lock, Cargo.lock, package-lock.json,
// requirements*.txt).
package manifest

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"strings"
)

// Versions returns every version of pkg recorded in the manifest file named
// filename (only its base name is used to pick the format). Lockfiles can hold
// several copies of one package, so all of them are returned; callers should
// require every copy to satisfy a constraint. A nil slice with a nil error means
// the package is not in the manifest.
func Versions(filename string, data []byte, pkg string) ([]string, error) {
	base := path.Base(filename)
	switch {
	case base == "go.mod":
		return goMod(data, pkg), nil
	case base == "uv.lock", base == "poetry.lock", base == "pdm.lock":
		return tomlPackages(data, pkg, normalizePython), nil
	case base == "Cargo.lock":
		return tomlPackages(data, pkg, func(s string) string { return s }), nil
	case base == "package-lock.json", base == "npm-shrinkwrap.json":
		return npmLock(data, pkg)
	case strings.HasPrefix(base, "requirements") && strings.HasSuffix(base, ".txt"):
		return requirements(data, pkg), nil
	default:
		return nil, fmt.Errorf("unsupported manifest %q (expected go.mod, uv.lock, poetry.lock, pdm.lock, Cargo.lock, package-lock.json or requirements*.txt)", base)
	}
}

// goMod reads require and replace directives. A replace that pins the module to
// another version wins over the require, as it does for the go command.
func goMod(data []byte, pkg string) []string {
	var required, replaced []string
	block := ""
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := sc.Text()
		if i := strings.Index(line, "//"); i != -1 {
			line = line[:i]
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if block != "" {
			if fields[0] == ")" {
				block = ""
				continue
			}
			fields = append([]string{block}, fields...)
		} else if len(fields) == 2 && fields[1] == "(" {
			block = fields[0]
			continue
		}
		switch fields[0] {
		case "require":
			if len(fields) >= 3 && fields[1] == pkg {
				required = append(required, fields[2])
			}
		case "replace":
			// replace old [v] => new [v]
			arrow := indexOf(fields, "=>")
			if arrow < 2 || fields[1] != pkg {
				continue
			}
			rhs := fields[arrow+1:]
			if len(rhs) == 2 {
				replaced = append(replaced, rhs[1])
			}
		}
	}
	if len(replaced) > 0 {
		return replaced
	}
	return required
}

func indexOf(s []string, v string) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return -1
}

var (
	tomlNameRe    = regexp.MustCompile(`^name\s*=\s*"([^"]+)"`)
	tomlVersionRe = regexp.MustCompile(`^version\s*=\s*"([^"]+)"`)
)

// tomlPackages reads the name/version pairs of [[package]] tables, the layout
// shared by uv.lock, poetry.lock, pdm.lock and Cargo.lock.
func tomlPackages(data []byte, pkg string, norm func(string) string) []string {
	want := norm(pkg)
	var out []string
	inPackage := false
	name, ver := "", ""
	flush := func() {
		if inPackage && name != "" && ver != "" && norm(name) == want {
			out = append(out, ver)
		}
		name, ver = "", ""
	}
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "[") {
			if line == "[[package]]" {
				flush()
				inPackage = true
				continue
			}
			// Sub-tables such as [package.metadata] or [[package.wheels]]
			// belong to the current package; any other table ends it.
			if !strings.HasPrefix(line, "[package.") && !strings.HasPrefix(line, "[[package.") {
				flush()
				inPackage = false
			}
			continue
		}
		if !inPackage {
			continue
		}
		if m := tomlNameRe.FindStringSubmatch(line); m != nil && name == "" {
			name = m[1]
		} else if m := tomlVersionRe.FindStringSubmatch(line); m != nil && ver == "" {
			ver = m[1]
		}
	}
	flush()
	return out
}

var pyNormRe = regexp.MustCompile(`[-_.]+`)

// normalizePython applies PEP 503 name normalization (PyJWT == pyjwt, zope.interface == zope-interface).
func normalizePython(s string) string {
	return pyNormRe.ReplaceAllString(strings.ToLower(strings.TrimSpace(s)), "-")
}

func npmLock(data []byte, pkg string) ([]string, error) {
	var lock struct {
		Packages map[string]struct {
			Version string `json:"version"`
		} `json:"packages"`
		Dependencies map[string]json.RawMessage `json:"dependencies"`
	}
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, fmt.Errorf("decoding package-lock.json: %w", err)
	}
	var out []string
	if len(lock.Packages) > 0 {
		suffix := "node_modules/" + pkg
		for key, p := range lock.Packages {
			if p.Version != "" && (key == suffix || strings.HasSuffix(key, "/"+suffix)) {
				out = append(out, p.Version)
			}
		}
		return out, nil
	}
	// lockfileVersion 1: nested "dependencies" trees.
	var walk func(map[string]json.RawMessage)
	walk = func(deps map[string]json.RawMessage) {
		for name, raw := range deps {
			var d struct {
				Version      string                     `json:"version"`
				Dependencies map[string]json.RawMessage `json:"dependencies"`
			}
			if json.Unmarshal(raw, &d) != nil {
				continue
			}
			if name == pkg && d.Version != "" {
				out = append(out, d.Version)
			}
			walk(d.Dependencies)
		}
	}
	walk(lock.Dependencies)
	return out, nil
}

var reqRe = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9._-]*)(?:\[[^\]]*\])?\s*===?\s*([^\s;#,\\]+)`)

func requirements(data []byte, pkg string) []string {
	want := normalizePython(pkg)
	var out []string
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		if m := reqRe.FindStringSubmatch(strings.TrimSpace(sc.Text())); m != nil && normalizePython(m[1]) == want {
			out = append(out, m[2])
		}
	}
	return out
}
