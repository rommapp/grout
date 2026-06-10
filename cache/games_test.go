package cache

import "testing"

func TestPickLenientMatch(t *testing.T) {
	fsNames := []string{"Pokemon Yellow", "rom_001", "Metroid"}
	names := []string{"Pokemon Yellow", "Zelda - Links Awakening", "Metroid"}

	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"exact", "Metroid", 2},
		{"case-insensitive fs name", "pokemon yellow", 0},
		{"normalized fs name (punctuation/spacing)", "Pokemon - Yellow!", 0},
		{"normalized rom name fallback", "Zelda Links Awakening", 1},
		{"no match", "Final Fantasy", -1},
		{"all-punctuation input does not match", "()[]", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pickLenientMatch(tt.input, fsNames, names); got != tt.want {
				t.Errorf("pickLenientMatch(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestPickLenientMatch_CaseInsensitiveBeatsNormalizedName(t *testing.T) {
	// candidate 0 matches only on normalized name; candidate 1 matches on
	// case-insensitive fs name. The case-insensitive fs match must win.
	fsNames := []string{"other_file", "metroid"}
	names := []string{"Metroid", "Something Else"}

	if got := pickLenientMatch("Metroid", fsNames, names); got != 1 {
		t.Errorf("expected case-insensitive fs match (idx 1) to win, got %d", got)
	}
}
