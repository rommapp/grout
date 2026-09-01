package stringutil

import "testing"

func TestStripExtension(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Sonic the Hedgehog.gba", "Sonic the Hedgehog"},
		{"Game.tar.gz", "Game.tar"},
		{"no-extension", "no-extension"},
		{"", ""},
		{".gitignore", ""},
		{"Sonic (USA).gba", "Sonic (USA)"},
	}
	for _, tt := range tests {
		if got := StripExtension(tt.in); got != tt.want {
			t.Errorf("StripExtension(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
		{5 * 1024 * 1024 * 1024, "5.0 GB"},
	}
	for _, tt := range tests {
		if got := FormatBytes(tt.in); got != tt.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestParseTag(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"single tag", "Sonic (USA).gba", "USA"},
		{"multiple tags are joined", "Sonic (USA) (Rev 1).gba", "USA Rev 1"},
		{"no tag", "Sonic.gba", ""},
		{"empty input", "", ""},
		{"brackets are not tags", "Sonic [!].gba", ""},
		{"tag with inner punctuation", "Sonic (USA, Europe).gba", "USA, Europe"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseTag(tt.in); got != tt.want {
				t.Errorf("ParseTag(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// PrepareRomName produces the string shown to a person. It is not an identity:
// it folds in the region, rewrites punctuation, and varies with configuration,
// so nothing may key off it. See internal/gamelist, which used to.
func TestPrepareRomName(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		regions []string
		want    string
	}{
		{"no regions is unchanged", "Sonic the Hedgehog", nil, "Sonic the Hedgehog"},
		{"one region is appended", "Sonic the Hedgehog", []string{"USA"}, "Sonic the Hedgehog (USA)"},
		{"several regions are joined", "Sonic", []string{"USA", "Europe"}, "Sonic (USA, Europe)"},
		{"colon becomes a dash", "Metroid: Zero Mission", nil, "Metroid - Zero Mission"},
		{"existing tag is stripped before the region is added", "Sonic (Japan)", []string{"USA"}, "Sonic (USA)"},
		{"ordered folder prefix is removed", "1) Sonic", nil, "Sonic"},
		{"empty regions slice behaves like none", "Sonic", []string{}, "Sonic"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PrepareRomName(tt.in, tt.regions); got != tt.want {
				t.Errorf("PrepareRomName(%q, %v) = %q, want %q", tt.in, tt.regions, got, tt.want)
			}
		})
	}
}

// Stripping a tag leaves the spaces that surrounded it behind.
func TestPrepareRomName_CollapsesSpaceLeftByAStrippedTag(t *testing.T) {
	got := PrepareRomName("Chrono Trigger (Japan) Special", nil)
	want := "Chrono Trigger Special"
	if got != want {
		t.Errorf("PrepareRomName = %q, want %q (a stripped tag must not leave a double space)", got, want)
	}
}

func TestNameCleaner(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		stripTag  bool
		wantClean string
		wantTag   string
	}{
		{"keeps the tag when not stripping", "Sonic (USA)", false, "Sonic (USA)", "USA"},
		{"strips the tag when asked", "Sonic (USA)", true, "Sonic", "USA"},
		{"reports no tag when there is none", "Sonic", true, "Sonic", ""},
		{"rewrites a colon", "Metroid: Zero Mission", true, "Metroid - Zero Mission", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotClean, gotTag := nameCleaner(tt.in, tt.stripTag)
			if gotClean != tt.wantClean {
				t.Errorf("cleaned = %q, want %q", gotClean, tt.wantClean)
			}
			if gotTag != tt.wantTag {
				t.Errorf("tag = %q, want %q", gotTag, tt.wantTag)
			}
		})
	}
}
