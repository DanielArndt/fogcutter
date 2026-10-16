package version

import "testing"

func TestMatch(t *testing.T) {
	tests := []struct {
		actual     string
		constraint string
		want       bool
	}{
		{"v1.2.0", ">=1.2.0", true},
		{"1.2.3", ">=v1.2.0", true},
		{"1.1.9", ">=1.2.0", false},
		{"5.21.2", "5.21.1", true}, // bare version defaults to >=
		{"5.21.0", "5.21.1", false},
		{"1.0.0-alpha.1", ">=1.0.0", false},
		{"1.0.0", ">=1.0.0-alpha.1", true},
		{"24.04", "==24.04", true},
		{"24.04.1", "==24.04", false},
		{"125", ">=120", true},
		{"119", ">=120", false},
	}

	for _, tc := range tests {
		got, err := Match(tc.actual, tc.constraint)
		if err != nil {
			t.Fatalf("Match(%q, %q) unexpected error: %v", tc.actual, tc.constraint, err)
		}
		if got != tc.want {
			t.Errorf("Match(%q, %q) = %v, want %v", tc.actual, tc.constraint, got, tc.want)
		}
	}
}
