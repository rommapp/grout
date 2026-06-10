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

func TestNormalizeForMatch_FoldsAccents(t *testing.T) {
	if got := normalizeForMatch("Pokémon - Recharged Yellow"); got != "pokemon recharged yellow" {
		t.Errorf("accent folding: got %q", got)
	}
	if normalizeForMatch("Pokémon") != normalizeForMatch("Pokemon") {
		t.Errorf("accented and unaccented should normalize equal")
	}
}

func TestPickLenientMatch_FoldsAccentsAcrossName(t *testing.T) {
	// Local file lacks the accent; the RomM name has it. Normalized-name tier matches.
	fsNames := []string{"rom_303"}
	names := []string{"Pokémon - Recharged Yellow"}
	if got := pickLenientMatch("Pokemon - Recharged Yellow", fsNames, names); got != 0 {
		t.Errorf("expected accent-folded match (idx 0), got %d", got)
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
