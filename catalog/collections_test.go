package catalog

import (
	"testing"

	"grout/romm"
	"grout/settings"
)

func TestMapped(t *testing.T) {
	config := settings.Config{DirectoryMappings: testMappings("snes")}

	if !Mapped(config, "snes") {
		t.Error("snes has a folder and must count as mapped")
	}
	if Mapped(config, "genesis") {
		t.Error("genesis has no folder and must not")
	}
}

// A game on a platform with no folder has nowhere to download to, so offering
// it would be offering something that cannot be done.
func TestGamesOnMappedPlatforms(t *testing.T) {
	games := []romm.Rom{
		{ID: 1, Name: "Mario", PlatformFSSlug: "snes"},
		{ID: 2, Name: "Sonic", PlatformFSSlug: "genesis"},
	}

	kept := GamesOnMappedPlatforms(settings.Config{DirectoryMappings: testMappings("snes")}, games)

	if len(kept) != 1 || kept[0].ID != 1 {
		t.Errorf("kept %v, want only the mapped platform's game", kept)
	}
}

func TestPlatformsIn(t *testing.T) {
	games := []romm.Rom{
		{ID: 1, Name: "Zelda", PlatformFSSlug: "snes", PlatformDisplayName: "Super Nintendo"},
		{ID: 2, Name: "Sonic", PlatformFSSlug: "genesis", PlatformDisplayName: "Mega Drive"},
		{ID: 3, Name: "Mario", PlatformFSSlug: "snes", PlatformDisplayName: "Super Nintendo"},
	}
	config := settings.Config{DirectoryMappings: testMappings("snes", "genesis")}

	groups := PlatformsIn(config, games)

	if len(groups) != 2 {
		t.Fatalf("got %d platforms, want one per mapped platform", len(groups))
	}
	if groups[0].Name != "Mega Drive" || groups[1].Name != "Super Nintendo" {
		t.Errorf("order = %q, %q, want them alphabetical", groups[0].Name, groups[1].Name)
	}
	// The screen shows a count beside each platform, which is the group size.
	if len(groups[1].Games) != 2 {
		t.Errorf("Super Nintendo holds %d games, want 2", len(groups[1].Games))
	}
}

// Ordering is what the reader scans down, so it ignores case rather than
// filing every lowercase name after every uppercase one.
func TestPlatformsIn_OrdersIgnoringCase(t *testing.T) {
	games := []romm.Rom{
		{ID: 1, PlatformFSSlug: "a", PlatformDisplayName: "amiga"},
		{ID: 2, PlatformFSSlug: "b", PlatformDisplayName: "Nintendo 64"},
		{ID: 3, PlatformFSSlug: "c", PlatformDisplayName: "ZX Spectrum"},
	}
	config := settings.Config{DirectoryMappings: testMappings("a", "b", "c")}

	groups := PlatformsIn(config, games)

	want := []string{"amiga", "Nintendo 64", "ZX Spectrum"}
	for i, name := range want {
		if groups[i].Name != name {
			t.Fatalf("order = %v, want %v", []string{groups[0].Name, groups[1].Name, groups[2].Name}, want)
		}
	}
}

func TestFilterCollectionsByName(t *testing.T) {
	collections := []romm.Collection{
		{ID: 1, Name: "Favourites"},
		{ID: 2, Name: "RPGs"},
		{ID: 3, Name: "favourite platformers"},
	}

	matched := FilterCollectionsByName(collections, "FAVOURITE")

	if len(matched) != 2 {
		t.Fatalf("matched %d, want both spellings of favourite", len(matched))
	}
}

func TestFilterCollectionsByName_NoMatches(t *testing.T) {
	collections := []romm.Collection{{ID: 1, Name: "Favourites"}}

	if got := FilterCollectionsByName(collections, "shooters"); len(got) != 0 {
		t.Errorf("matched %v, want nothing", got)
	}
}

// Dragging the list and then backing out still saves the order, so the compare
// has to be against what the user started with.
func TestReordered(t *testing.T) {
	original := []romm.Platform{{FSSlug: "snes"}, {FSSlug: "genesis"}, {FSSlug: "gba"}}

	if got := Reordered(original, original); got != nil {
		t.Errorf("Reordered = %v, want nothing when nothing moved", got)
	}

	shuffled := []romm.Platform{{FSSlug: "gba"}, {FSSlug: "snes"}, {FSSlug: "genesis"}}
	got := Reordered(original, shuffled)
	if len(got) != 3 || got[0].FSSlug != "gba" {
		t.Errorf("Reordered = %v, want the new order", got)
	}
}

// A list that came back a different length is not a reordering of the one that
// went in, and saving it would drop platforms.
func TestReordered_LengthMismatch(t *testing.T) {
	original := []romm.Platform{{FSSlug: "snes"}, {FSSlug: "genesis"}}
	short := []romm.Platform{{FSSlug: "snes"}}

	if got := Reordered(original, short); got != nil {
		t.Errorf("Reordered = %v, want nothing for a list of a different length", got)
	}
}

// The result is the caller's to keep, so it must not alias the list the screen
// still holds.
func TestReordered_CopiesTheResult(t *testing.T) {
	original := []romm.Platform{{FSSlug: "snes"}, {FSSlug: "genesis"}}
	shuffled := []romm.Platform{{FSSlug: "genesis"}, {FSSlug: "snes"}}

	got := Reordered(original, shuffled)
	shuffled[0] = romm.Platform{FSSlug: "changed"}

	if got[0].FSSlug != "genesis" {
		t.Error("the saved order changed when the screen's list did")
	}
}
