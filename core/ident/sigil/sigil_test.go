package sigil

import "testing"

func TestVersionIsThePinnedRelease(t *testing.T) {
	if got := Version(); got != "1.0.0" {
		t.Fatalf("Version() = %q, want 1.0.0", got)
	}
}
