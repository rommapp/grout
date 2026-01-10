package update

import (
	"testing"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected Version
		wantErr  bool
	}{
		{"v1.2.3", Version{Major: 1, Minor: 2, Patch: 3, Prerelease: ""}, false},
		{"1.2.3", Version{Major: 1, Minor: 2, Patch: 3, Prerelease: ""}, false},
		{"v1.2.0-beta.1", Version{Major: 1, Minor: 2, Patch: 0, Prerelease: "beta.1"}, false},
		{"1.2.0-beta.1", Version{Major: 1, Minor: 2, Patch: 0, Prerelease: "beta.1"}, false},
		{"v1.0.0-alpha", Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha"}, false},
		{"1.0", Version{Major: 1, Minor: 0, Patch: 0, Prerelease: ""}, false},
		{"1", Version{Major: 1, Minor: 0, Patch: 0, Prerelease: ""}, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseVersion(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseVersion(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Major != tt.expected.Major || got.Minor != tt.expected.Minor || got.Patch != tt.expected.Patch || got.Prerelease != tt.expected.Prerelease {
					t.Errorf("ParseVersion(%q) = %+v, want %+v", tt.input, got, tt.expected)
				}
			}
		})
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		current  string
		latest   string
		expected int // -1 if current < latest, 0 if equal, 1 if current > latest
		desc     string
	}{
		// Normal version comparisons
		{"1.0.0", "1.0.1", -1, "patch version newer"},
		{"1.0.1", "1.0.0", 1, "patch version older"},
		{"1.0.0", "1.1.0", -1, "minor version newer"},
		{"1.1.0", "1.0.0", 1, "minor version older"},
		{"1.0.0", "2.0.0", -1, "major version newer"},
		{"2.0.0", "1.0.0", 1, "major version older"},
		{"1.0.0", "1.0.0", 0, "same version"},

		// Beta vs full release - THE KEY FIX
		{"v1.2.0-beta.1", "v1.2.0", -1, "beta should recognize full release as newer"},
		{"v1.2.0", "v1.2.0-beta.1", 1, "full release should be newer than beta"},
		{"1.2.0-beta.1", "1.2.0", -1, "beta should recognize full release as newer (no v prefix)"},
		{"1.2.0", "1.2.0-beta.1", 1, "full release should be newer than beta (no v prefix)"},

		// Beta vs beta
		{"v1.2.0-beta.1", "v1.2.0-beta.2", -1, "newer beta should be recognized"},
		{"v1.2.0-beta.2", "v1.2.0-beta.1", 1, "older beta should be recognized"},
		{"v1.2.0-beta.1", "v1.2.0-beta.1", 0, "same beta versions"},

		// Different prerelease types
		{"v1.2.0-alpha", "v1.2.0-beta.1", -1, "alpha < beta"},
		{"v1.2.0-beta.1", "v1.2.0-alpha", 1, "beta > alpha"},

		// Edge cases
		{"v1.2.0-beta.1", "v1.2.1", -1, "beta should recognize higher patch as newer"},
		{"v1.2.1", "v1.2.0-beta.1", 1, "higher patch should be newer than beta"},
		{"v1.2.0-beta.1", "v1.3.0", -1, "beta should recognize higher minor as newer"},
		{"v1.3.0", "v1.2.0-beta.1", 1, "higher minor should be newer than beta"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := CompareVersions(tt.current, tt.latest)
			if got != tt.expected {
				t.Errorf("CompareVersions(%q, %q) = %d, want %d", tt.current, tt.latest, got, tt.expected)
			}
		})
	}
}

func TestIsNewerVersion(t *testing.T) {
	tests := []struct {
		current  string
		latest   string
		expected bool
		desc     string
	}{
		{"v1.2.0-beta.1", "v1.2.0", true, "beta should see full release as newer"},
		{"v1.2.0", "v1.2.0-beta.1", false, "full release should not see beta as newer"},
		{"v1.1.0", "v1.2.0", true, "older version should see newer as newer"},
		{"v1.2.0", "v1.1.0", false, "newer version should not see older as newer"},
		{"v1.2.0", "v1.2.0", false, "same version should not be newer"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := IsNewerVersion(tt.current, tt.latest)
			if got != tt.expected {
				t.Errorf("IsNewerVersion(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.expected)
			}
		})
	}
}

