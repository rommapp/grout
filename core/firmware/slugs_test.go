package firmware

import "testing"

func TestKnownSlugs(t *testing.T) {
	known, err := KnownSlugs()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"gba", "psx", "atari8bit", "pico-8", "ports"} {
		if !known[s] {
			t.Errorf("%s not known", s)
		}
	}
	if known["gameboy-advance"] {
		t.Error("made-up slug known")
	}
}
