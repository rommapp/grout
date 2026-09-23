package ui

import (
	"testing"

	"grout/romm"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

// The Collections row sits among the platforms but is not one. Reading the
// order back has to skip it, or it would be saved as a platform.
func TestShownPlatforms_SkipsTheCollectionsRow(t *testing.T) {
	items := []gaba.MenuItem{
		{Text: "Collections", Metadata: collectionsRow{}},
		{Text: "SNES", Metadata: romm.Platform{FSSlug: "snes"}},
		{Text: "Mega Drive", Metadata: romm.Platform{FSSlug: "genesis"}},
	}

	got := shownPlatforms(items)

	if len(got) != 2 || got[0].FSSlug != "snes" || got[1].FSSlug != "genesis" {
		t.Errorf("shownPlatforms = %v, want just the platforms in order", got)
	}
}

// The row used to be a platform with the reserved slug "collections", which a
// real platform could have taken. Its own type cannot be mistaken for one.
func TestCollectionsRow_IsNotAPlatform(t *testing.T) {
	var metadata any = collectionsRow{}

	if _, isPlatform := metadata.(romm.Platform); isPlatform {
		t.Error("the collections row must not read as a platform")
	}

	// And a platform that happens to be called collections still is one.
	metadata = romm.Platform{FSSlug: "collections"}
	if _, isPlatform := metadata.(romm.Platform); !isPlatform {
		t.Error("a real platform must read as a platform whatever its slug")
	}
}

func TestShownPlatforms_Empty(t *testing.T) {
	if got := shownPlatforms(nil); len(got) != 0 {
		t.Errorf("shownPlatforms = %v, want none", got)
	}
}
