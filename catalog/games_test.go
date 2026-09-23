package catalog

import (
	"slices"
	"testing"

	"grout/romm"
)

func games(names ...string) []romm.Rom {
	out := make([]romm.Rom, 0, len(names))
	for i, n := range names {
		out = append(out, romm.Rom{ID: i + 1, Name: n})
	}
	return out
}

func gameNames(games []romm.Rom) []string {
	out := make([]string, 0, len(games))
	for _, g := range games {
		out = append(out, g.Name)
	}
	return out
}

func TestFilterByName(t *testing.T) {
	all := games("Sonic the Hedgehog", "Super Mario World", "Sonic CD", "Tetris")

	got := gameNames(FilterByName(all, "sonic"))
	want := []string{"Sonic CD", "Sonic the Hedgehog"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v (matches, ordered by name)", got, want)
	}
}

// Search is what a person types, so it cannot be case sensitive.
func TestFilterByName_IgnoresCase(t *testing.T) {
	all := games("Sonic the Hedgehog", "SONIC CD")
	if got := FilterByName(all, "SoNiC"); len(got) != 2 {
		t.Errorf("got %d matches, want 2", len(got))
	}
}

// An empty filter matches everything, which is what clearing the search does.
func TestFilterByName_EmptyFilterKeepsEverything(t *testing.T) {
	all := games("Tetris", "Alpha")
	got := gameNames(FilterByName(all, ""))
	want := []string{"Alpha", "Tetris"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestFilterByName_NoMatchesIsEmpty(t *testing.T) {
	if got := FilterByName(games("Tetris"), "zelda"); len(got) != 0 {
		t.Errorf("got %v, want none", gameNames(got))
	}
}

// The caller still shows the unfiltered list, so filtering must not reorder it.
func TestFilterByName_DoesNotMutateInput(t *testing.T) {
	all := games("Tetris", "Alpha")
	FilterByName(all, "")
	if all[0].Name != "Tetris" {
		t.Errorf("input was reordered: %v", gameNames(all))
	}
}

func TestHasFilterableMetadata(t *testing.T) {
	tests := []struct {
		name  string
		games []romm.Rom
		want  bool
	}{
		{"no games", nil, false},
		{"no metadata", games("Tetris"), false},
		{"genre", []romm.Rom{{Metadatum: romm.RomMetadata{Genres: []string{"Puzzle"}}}}, true},
		{"region", []romm.Rom{{Regions: []string{"USA"}}}, true},
		{"language", []romm.Rom{{Languages: []string{"English"}}}, true},
		{"only the second game has any", []romm.Rom{
			{Name: "Tetris"},
			{Name: "Sonic", Regions: []string{"USA"}},
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasFilterableMetadata(tt.games); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGameSource_IsCollection(t *testing.T) {
	platform := GameSource{Platform: romm.Platform{ID: 1, Name: "Game Boy"}}
	collection := GameSource{Collection: romm.Collection{ID: 7, Name: "Favourites"}}

	if platform.IsCollection() {
		t.Error("a platform source is not a collection")
	}
	if !collection.IsCollection() {
		t.Error("a collection source is a collection")
	}
	if platform.Name() != "Game Boy" || collection.Name() != "Favourites" {
		t.Errorf("names = %q and %q", platform.Name(), collection.Name())
	}
}

// A collection spans platforms, so "does this platform have BIOS files" has no
// answer for one. Asking anyway would offer a BIOS button that leads nowhere.
func TestGameSource_HasBIOS(t *testing.T) {
	tests := []struct {
		name   string
		source GameSource
		want   bool
	}{
		{"platform with firmware", GameSource{Platform: romm.Platform{ID: 1, FirmwareCount: 3}}, true},
		{"platform without firmware", GameSource{Platform: romm.Platform{ID: 1}}, false},
		{"unset platform", GameSource{Platform: romm.Platform{FirmwareCount: 3}}, false},
		{"collection never has BIOS", GameSource{
			Platform:   romm.Platform{ID: 1, FirmwareCount: 3},
			Collection: romm.Collection{ID: 7},
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.source.HasBIOS(); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
