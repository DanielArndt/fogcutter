package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSpec(t *testing.T) {
	tests := []struct {
		spec     string
		wantType string
		wantRepo string
		wantSnap string
		wantVer  string
		wantIss  int
	}{
		{
			spec:     "canonical/kratos#364",
			wantType: "github-issue",
			wantRepo: "canonical/kratos",
			wantIss:  364,
		},
		{
			spec:     "https://github.com/openfga/openfga/issues/912",
			wantType: "github-issue",
			wantRepo: "openfga/openfga",
			wantIss:  912,
		},
		{
			spec:     "github-release:openfga/openfga>=v1.8.0",
			wantType: "github-release",
			wantRepo: "openfga/openfga",
			wantVer:  ">=v1.8.0",
		},
		{
			spec:     "snap:lxd:5.21/stable>=5.21.2",
			wantType: "snapcraft",
			wantSnap: "lxd",
			wantVer:  ">=5.21.2",
		},
		{
			spec:     "snap:go/1.26/stable>=1.26.9",
			wantType: "snapcraft",
			wantSnap: "go",
			wantVer:  ">=1.26.9",
		},
	}

	for _, tc := range tests {
		got, err := ParseSpec(tc.spec)
		if err != nil {
			t.Fatalf("ParseSpec(%q) unexpected error: %v", tc.spec, err)
		}
		if got.Type != tc.wantType {
			t.Errorf("ParseSpec(%q).Type = %q, want %q", tc.spec, got.Type, tc.wantType)
		}
		if tc.wantRepo != "" && got.Repo != tc.wantRepo {
			t.Errorf("ParseSpec(%q).Repo = %q, want %q", tc.spec, got.Repo, tc.wantRepo)
		}
		if tc.wantSnap != "" && got.Snap != tc.wantSnap {
			t.Errorf("ParseSpec(%q).Snap = %q, want %q", tc.spec, got.Snap, tc.wantSnap)
		}
		if tc.wantVer != "" && got.Version != tc.wantVer {
			t.Errorf("ParseSpec(%q).Version = %q, want %q", tc.spec, got.Version, tc.wantVer)
		}
		if tc.wantIss != 0 && got.Issue != tc.wantIss {
			t.Errorf("ParseSpec(%q).Issue = %d, want %d", tc.spec, got.Issue, tc.wantIss)
		}
	}
}

func TestScanFilesAndIssues(t *testing.T) {
	dir := t.TempDir()
	sampleFile := filepath.Join(dir, "workaround.go")
	content := `package sample
// BLOCKER(canonical/kratos#364): remove multiStatements workaround once upstream closes
// TODO(blocked-by:snap:juju:3.5/stable>=3.5.4): upgrade juju client
`
	if err := os.WriteFile(sampleFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	blockers, err := ScanFiles([]string{dir}, nil, nil)
	if err != nil {
		t.Fatalf("ScanFiles failed: %v", err)
	}
	if len(blockers) != 2 {
		t.Fatalf("got %d blockers, want 2", len(blockers))
	}
	if blockers[0].Source.Type != "github-issue" || blockers[0].Source.Issue != 364 {
		t.Errorf("unexpected first blocker: %+v", blockers[0])
	}
	if blockers[1].Source.Type != "snapcraft" || blockers[1].Source.Snap != "juju" || blockers[1].Source.Channel != "3.5/stable" {
		t.Errorf("unexpected second blocker: %+v", blockers[1])
	}

	issueBody := `We cannot complete this feature yet.
Blocked by https://github.com/canonical/hydra/issues/42
Blocked on snap:go/1.26/stable>=1.26.9
- BLOCKER(github-release:openfga/openfga>=v1.8.0): Needs batch check API
`
	fromIssue := ExtractFromIssueBody("canonical/authz", 19, "Support batch check", issueBody)
	if len(fromIssue) != 3 {
		t.Fatalf("got %d issue blockers, want 3", len(fromIssue))
	}
	if fromIssue[1].Source.Snap != "go" || fromIssue[1].Source.Channel != "1.26/stable" || fromIssue[1].Source.Version != ">=1.26.9" {
		t.Errorf("unexpected go snap blocker: %+v", fromIssue[1].Source)
	}
	if fromIssue[0].OnResolved[0].Type != "comment_issue" || fromIssue[0].OnResolved[0].Issue != 19 {
		t.Errorf("unexpected action on issue blocker: %+v", fromIssue[0].OnResolved)
	}
}
