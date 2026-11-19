package muos

import "testing"

func TestLabel(t *testing.T) {
	for _, tc := range []struct {
		folder string
		want   string
	}{
		{"file/mGBA", "mGBA"},
		{"file/PPSSPP-Ext", "PPSSPP-Ext"},
		{"file/CPC/GX4000 (Caprice)", "GX4000 (Caprice)"},
		{"drastic/backup", "drastic"},
		{"drastic-legacy/backup", "drastic-legacy"},
		{"pico8", "pico8"},
		{"../pickles/sram/gpsp/gba", "Pickles (gpsp)"},
		{"../pickles/sram/mgba/Game Boy Advance", "Pickles (mgba)"},
		{"romfile/Thing", "Thing"},
		{"file/backup/Thing", "Thing"},
	} {
		if got := Label(tc.folder); got != tc.want {
			t.Errorf("Label(%q) = %q, want %q", tc.folder, got, tc.want)
		}
	}
}
