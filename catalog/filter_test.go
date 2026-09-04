package catalog

import (
	"slices"
	"testing"

	"grout/romm"
	"grout/settings"
)

// The Category and Family filters are built from the values present across
// platforms. When RomM does not populate a field the list is empty, which is
// the signal to hide the filter rather than show an "All"-only picker (#247).
func TestDistinctValues(t *testing.T) {
	platforms := []romm.Platform{
		{Category: "console", Family: "Nintendo", Generation: 4},
		{Category: "handheld", Family: "Nintendo", Generation: 4},
		{Category: "console", Family: "Sega", Generation: 3},
		{Category: "", Family: "", Generation: 0},
	}

	if got, want := Categories(platforms), []string{"console", "handheld"}; !slices.Equal(got, want) {
		t.Errorf("Categories = %v, want %v (sorted, deduped, no empties)", got, want)
	}
	if got, want := Families(platforms), []string{"Nintendo", "Sega"}; !slices.Equal(got, want) {
		t.Errorf("Families = %v, want %v", got, want)
	}
	if got, want := Generations(platforms), []int{3, 4}; !slices.Equal(got, want) {
		t.Errorf("Generations = %v, want %v (zero means unknown and is dropped)", got, want)
	}
}

func TestDistinctValues_NoMetadataHidesFilter(t *testing.T) {
	platforms := []romm.Platform{
		{Category: "", Family: "", Generation: 3},
		{Category: "", Family: "", Generation: 4},
	}

	if got := Categories(platforms); len(got) != 0 {
		t.Errorf("expected no categories so the filter is hidden, got %v", got)
	}
	if got := Families(platforms); len(got) != 0 {
		t.Errorf("expected no families so the filter is hidden, got %v", got)
	}
}

// An unset filter has to show the whole library rather than nothing, because it
// is what the screen opens with.
func TestPlatformFilter_ZeroValueAcceptsEverything(t *testing.T) {
	var filter PlatformFilter
	p := romm.Platform{FSSlug: "gba", ROMCount: 0, Category: "handheld", Generation: 6}

	if !filter.Match(p, false) || !filter.Match(p, true) {
		t.Error("the zero filter must accept every platform")
	}
}

func TestPlatformFilter_Match(t *testing.T) {
	p := romm.Platform{ROMCount: 5, Category: "console", Family: "Sega", Generation: 4}

	tests := []struct {
		name   string
		filter PlatformFilter
		mapped bool
		want   bool
	}{
		{"all accepts", PlatformFilter{Status: StatusAll}, false, true},
		{"mapped keeps mapped", PlatformFilter{Status: StatusMapped}, true, true},
		{"mapped drops unmapped", PlatformFilter{Status: StatusMapped}, false, false},
		{"unmapped keeps unmapped", PlatformFilter{Status: StatusUnmapped}, false, true},
		{"unmapped drops mapped", PlatformFilter{Status: StatusUnmapped}, true, false},
		{"category matches", PlatformFilter{Category: "console"}, false, true},
		{"category excludes", PlatformFilter{Category: "handheld"}, false, false},
		{`category "all" accepts`, PlatformFilter{Category: "all"}, false, true},
		{"family excludes", PlatformFilter{Family: "Nintendo"}, false, false},
		{"generation matches", PlatformFilter{Generation: 4}, false, true},
		{"generation excludes", PlatformFilter{Generation: 3}, false, false},
		{"generation zero accepts", PlatformFilter{Generation: 0}, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.filter.Match(p, tt.mapped); got != tt.want {
				t.Errorf("Match = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPlatformFilter_WithGamesOnly(t *testing.T) {
	filter := PlatformFilter{WithGamesOnly: true}

	if filter.Match(romm.Platform{ROMCount: 0}, false) {
		t.Error("an empty platform must be hidden")
	}
	if !filter.Match(romm.Platform{ROMCount: 1}, false) {
		t.Error("a platform with roms must be shown")
	}
}

// A platform mapped to Skip is stored with an empty path. That is still
// unmapped as far as the filter is concerned, or Skip would hide the platform
// from the very filter meant to find it again.
func TestFilterPlatforms_SkipCountsAsUnmapped(t *testing.T) {
	platforms := []romm.Platform{{FSSlug: "gba"}, {FSSlug: "snes"}}
	mappings := map[string]settings.DirectoryMapping{
		"gba":  {RomMSlug: "gba", RelativePath: ""},
		"snes": {RomMSlug: "snes", RelativePath: "SNES"},
	}

	unmapped := FilterPlatforms(platforms, PlatformFilter{Status: StatusUnmapped}, mappings)
	if len(unmapped) != 1 || unmapped[0].FSSlug != "gba" {
		t.Errorf("unmapped = %v, want just gba", unmapped)
	}

	mapped := FilterPlatforms(platforms, PlatformFilter{Status: StatusMapped}, mappings)
	if len(mapped) != 1 || mapped[0].FSSlug != "snes" {
		t.Errorf("mapped = %v, want just snes", mapped)
	}
}

func TestFilterPlatforms_KeepsOrder(t *testing.T) {
	platforms := []romm.Platform{{FSSlug: "c"}, {FSSlug: "a"}, {FSSlug: "b"}}

	got := FilterPlatforms(platforms, PlatformFilter{}, nil)
	for i, want := range []string{"c", "a", "b"} {
		if got[i].FSSlug != want {
			t.Fatalf("order = %v, want the input order", got)
		}
	}
}
