package catalog

import (
	"testing"

	"grout/romm"
	"grout/settings"
)

// A game counts as missing unless every file it needs is on the card, so a
// half-arrived multi-disc game is fetched again rather than left broken.
func TestMissing(t *testing.T) {
	complete, partial := romsOnCard(t)
	absent := romm.Rom{
		ID: 3, Name: "Zelda", PlatformFSSlug: "snes", FsNameNoExt: "Zelda",
		Files: []romm.RomFile{{FileName: "Zelda.zip"}},
	}

	missing := Missing(settings.Config{}, []romm.Rom{complete, partial, absent})

	var got []int
	for _, game := range missing {
		got = append(got, game.ID)
	}
	if len(got) != 2 || got[0] != partial.ID || got[1] != absent.ID {
		t.Errorf("missing = %v, want [%d %d]", got, partial.ID, absent.ID)
	}
}
