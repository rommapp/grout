package catalog

import (
	"testing"

	"grout/cfw"
	"grout/romm"
	"grout/settings"
)

// muOS accepts two folder names for the Game Boy Advance, MinUI two tagged
// ones. Both are used below because the tagged and untagged paths through the
// choice logic differ.
var gba = romm.Platform{FSSlug: "gba", Name: "Game Boy Advance"}

func paths(choices []DirectoryChoice) []string {
	out := make([]string, 0, len(choices))
	for _, choice := range choices {
		out = append(out, choice.RelativePath)
	}
	return out
}

func TestDirectoryChoicesFor_OffersMissingFoldersFirst(t *testing.T) {
	got := DirectoryChoicesFor(ChoiceRequest{
		Platform:    gba,
		CFW:         cfw.MuOS,
		Directories: []string{"gba", "SNES"},
	})

	// "gba" is on disk, so only the alias is offered for creation, and the
	// creatable entries come before the ones that already exist.
	want := []string{"Nintendo Game Boy Advance", "gba"}
	if p := paths(got.Directories); len(p) != 2 || p[0] != want[0] || p[1] != want[1] {
		t.Fatalf("choices = %v, want %v", p, want)
	}
	if !got.Directories[0].Create {
		t.Error("the missing folder must be marked for creation")
	}
	if got.Directories[1].Create {
		t.Error("a folder already on disk must not be marked for creation")
	}
}

// A folder whose name matches the platform is what the user almost always
// wants, so a first run picks it without asking.
func TestDirectoryChoicesFor_FirstRunDetectsByName(t *testing.T) {
	got := DirectoryChoicesFor(ChoiceRequest{
		Platform:    gba,
		CFW:         cfw.MuOS,
		Directories: []string{"gba", "SNES"},
	})

	if got.Selected < 0 || got.Directories[got.Selected].RelativePath != "gba" {
		t.Errorf("selected = %d (%v), want the gba folder", got.Selected, paths(got.Directories))
	}
}

// With nothing on disk to match, the screen either offers to create the
// firmware's own folder or leaves the platform on Skip. Setup does the former;
// the settings screen does the latter so a return visit changes nothing on its
// own.
func TestDirectoryChoicesFor_AutoSelectOffersToCreate(t *testing.T) {
	request := ChoiceRequest{Platform: gba, CFW: cfw.MuOS, AutoSelect: true}

	got := DirectoryChoicesFor(request)
	if got.Selected != 0 || got.Directories[0].RelativePath != "gba" {
		t.Errorf("selected = %d (%v), want the first create option", got.Selected, paths(got.Directories))
	}

	request.AutoSelect = false
	if got := DirectoryChoicesFor(request); got.Selected != -1 {
		t.Errorf("selected = %d, want -1 so the platform starts on Skip", got.Selected)
	}
}

// A return visit restores what the user chose, even where the name-based guess
// would have picked something else.
func TestDirectoryChoicesFor_ReturnVisitRestoresChoice(t *testing.T) {
	got := DirectoryChoicesFor(ChoiceRequest{
		Platform:    gba,
		CFW:         cfw.MuOS,
		Directories: []string{"gba", "Nintendo Game Boy Advance"},
		Existing: map[string]settings.DirectoryMapping{
			"gba": {RomMSlug: "gba", RelativePath: "Nintendo Game Boy Advance"},
		},
	})

	if got.Selected < 0 || got.Directories[got.Selected].RelativePath != "Nintendo Game Boy Advance" {
		t.Errorf("selected = %d (%v), want the stored folder", got.Selected, paths(got.Directories))
	}
}

// A path typed by hand matches none of the offered folders. It has to come back
// as Custom or the screen would silently drop it on the next save.
func TestDirectoryChoicesFor_ReturnVisitKeepsTypedPath(t *testing.T) {
	got := DirectoryChoicesFor(ChoiceRequest{
		Platform:    gba,
		CFW:         cfw.MuOS,
		Directories: []string{"gba"},
		Existing: map[string]settings.DirectoryMapping{
			"gba": {RomMSlug: "gba", RelativePath: "roms/handhelds/gba"},
		},
	})

	if got.Custom != "roms/handhelds/gba" {
		t.Errorf("custom = %q, want the typed path", got.Custom)
	}
	if got.Selected != -1 {
		t.Errorf("selected = %d, want -1 so the view lands on the custom entry", got.Selected)
	}
}

// Skipping a platform is a real choice. A return visit must leave it skipped
// rather than re-running the name guess and quietly mapping it.
func TestDirectoryChoicesFor_ReturnVisitKeepsSkip(t *testing.T) {
	got := DirectoryChoicesFor(ChoiceRequest{
		Platform:    gba,
		CFW:         cfw.MuOS,
		Directories: []string{"gba"},
		AutoSelect:  true,
		Existing: map[string]settings.DirectoryMapping{
			"snes": {RomMSlug: "snes", RelativePath: "snes"},
		},
	})

	if got.Selected != -1 {
		t.Errorf("selected = %d, want -1 for a platform left skipped", got.Selected)
	}
}

// MinUI tags its folders, so "Game Boy Advance (MGBA)" belongs to the GBA but
// is not the same folder as "Game Boy Advance (GBA)".
func TestDirectoryChoicesFor_TaggedFolders(t *testing.T) {
	got := DirectoryChoicesFor(ChoiceRequest{
		Platform:    gba,
		CFW:         cfw.MinUI,
		Directories: []string{"Game Boy Advance (MGBA)"},
		AutoSelect:  true,
	})

	want := []string{"Game Boy Advance (GBA)", "Game Boy Advance (MGBA)"}
	if p := paths(got.Directories); len(p) != 2 || p[0] != want[0] || p[1] != want[1] {
		t.Fatalf("choices = %v, want %v", p, want)
	}
	if !got.Directories[0].Create || got.Directories[1].Create {
		t.Error("only the folder missing from disk may be offered for creation")
	}
	// The tag is the only part that differs, so it is what the option shows.
	if got.Directories[0].Display != "GBA" || got.Directories[1].Display != "MGBA" {
		t.Errorf("displays = %q/%q, want GBA/MGBA", got.Directories[0].Display, got.Directories[1].Display)
	}
}

// The server's own slug mapping wins: an admin who told RomM that this platform
// is really another expects grout to look where the server says.
func TestDirectoryChoicesFor_HonoursPlatformsBinding(t *testing.T) {
	got := DirectoryChoicesFor(ChoiceRequest{
		Platform:         romm.Platform{FSSlug: "gameboyadvance"},
		CFW:              cfw.MuOS,
		PlatformsBinding: map[string]string{"gameboyadvance": "gba"},
		AutoSelect:       true,
	})

	if len(got.Directories) == 0 || got.Directories[0].RelativePath != "gba" {
		t.Errorf("choices = %v, want the bound platform's folders", paths(got.Directories))
	}
}

// A platform no firmware table mentions still needs somewhere to go, so it
// falls back to its own slug.
func TestDirectoryChoicesFor_UnknownPlatformUsesItsSlug(t *testing.T) {
	got := DirectoryChoicesFor(ChoiceRequest{
		Platform:   romm.Platform{FSSlug: "not-a-console"},
		CFW:        cfw.MuOS,
		AutoSelect: true,
	})

	if len(got.Directories) != 1 || got.Directories[0].RelativePath != "not-a-console" {
		t.Errorf("choices = %v, want the platform's own slug", paths(got.Directories))
	}
}
