package cfw

import "testing"

// SaveBasename returns the on-disk basename (before the save-file extension) an emulator
// uses for a ROM's saves, given whether the device keeps the ROM extension. minarch saves
// (NextUI/MinUI default) keep the FULL ROM filename incl. extension (e.g. "Game.sfc.sav");
// the RetroArch convention uses the ROM basename without extension (e.g. "Game.srm").
// NextUI exposes BOTH as a user setting, so the style is resolved per-device, not per-CFW
// (issue #245).
func TestSaveBasename(t *testing.T) {
	cases := []struct {
		name        string
		keepRomExt  bool
		romFile     string
		want        string
	}{
		{"keep retains full ROM filename", true, "Donkey Kong Country (USA) (Rev 2).sfc", "Donkey Kong Country (USA) (Rev 2).sfc"},
		{"keep with short name", true, "Mario.gb", "Mario.gb"},
		{"strip removes the ROM extension", false, "Pokemon - Emerald Version (USA, Europe).gba", "Pokemon - Emerald Version (USA, Europe)"},
		{"strip removes only the final extension", false, "Final Fantasy IV (V1.1).sfc", "Final Fantasy IV (V1.1)"},
		{"no-extension ROM unchanged when stripping", false, "Doom", "Doom"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SaveBasename(tc.keepRomExt, tc.romFile); got != tc.want {
				t.Errorf("SaveBasename(%v, %q) = %q, want %q", tc.keepRomExt, tc.romFile, got, tc.want)
			}
		})
	}
}

// DefaultKeepsRomExt is the per-CFW default used only when the on-device convention can't
// be detected. The minarch CFWs (NextUI, MinUI) default to keeping the ROM extension;
// everything else defaults to RetroArch-style stripping.
func TestDefaultKeepsRomExt(t *testing.T) {
	for _, c := range []CFW{NextUI, MinUI} {
		if !DefaultKeepsRomExt(c) {
			t.Errorf("%s should default to keeping the ROM extension", c)
		}
	}
	for _, c := range []CFW{MuOS, Knulli, Spruce, ROCKNIX, Onion, ArkOS, Batocera, Trimui, Allium, Koriki} {
		if DefaultKeepsRomExt(c) {
			t.Errorf("%s should default to RetroArch-style stripping", c)
		}
	}
}
