package romm

import (
	"testing"
	"time"

	"grout/domain/library"
)

func TestToGame_MapsFields(t *testing.T) {
	r := Rom{
		FsName:                "Sonic the Hedgehog.gba",
		FsNameNoExt:           "Sonic the Hedgehog",
		Name:                  "Sonic the Hedgehog",
		Summary:               "A blue hedgehog runs fast.",
		Regions:               []string{"USA"},
		Languages:             []string{"English"},
		Md5Hash:               "abc123",
		ScreenScraperID:       42,
		RetroAchievementsID:   7,
		RetroAchievementsHash: "deadbeef",
		Metadatum: RomMetadata{
			Genres:    []string{"Platform"},
			Companies: []string{"Sega"},
		},
	}

	art := library.ArtPaths{Cover: "/roms/gba/media/Sonic.png"}
	got := r.ToGame("Sonic the Hedgehog (USA)", "/roms/gba/Sonic the Hedgehog.gba", art)

	checks := []struct {
		field     string
		got, want any
	}{
		{"FileName", got.FileName, "Sonic the Hedgehog.gba"},
		{"BaseName", got.BaseName, "Sonic the Hedgehog"},
		{"Path", got.Path, "/roms/gba/Sonic the Hedgehog.gba"},
		{"DisplayName", got.DisplayName, "Sonic the Hedgehog (USA)"},
		{"Summary", got.Summary, "A blue hedgehog runs fast."},
		{"MD5", got.MD5, "abc123"},
		{"ScreenScraperID", got.ScreenScraperID, 42},
		{"RetroAchievementsID", got.RetroAchievementsID, 7},
		{"RetroAchievementsHash", got.RetroAchievementsHash, "deadbeef"},
		{"Art.Cover", got.Art.Cover, "/roms/gba/media/Sonic.png"},
		{"MaxPlayers", got.MaxPlayers, 1},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.field, c.got, c.want)
		}
	}
}

// The display name is supplied by the caller. Nothing downstream derives it, so
// it can differ from the RomM name entirely without affecting identity.
func TestToGame_DisplayNameIsCallerSuppliedAndNotIdentity(t *testing.T) {
	r := Rom{FsName: "Sonic.gba", FsNameNoExt: "Sonic", Name: "Sonic the Hedgehog", Regions: []string{"USA"}}

	withRegion := r.ToGame("Sonic the Hedgehog (USA)", "", library.ArtPaths{})
	withoutRegion := r.ToGame("Sonic the Hedgehog", "", library.ArtPaths{})

	if withRegion.FileName != withoutRegion.FileName {
		t.Errorf("identity changed with the display name: %q vs %q", withRegion.FileName, withoutRegion.FileName)
	}
	if withRegion.DisplayName == withoutRegion.DisplayName {
		t.Error("expected the caller-supplied display name to be used verbatim")
	}
}

func TestToGame_ReleaseDate(t *testing.T) {
	// RomM reports milliseconds since the epoch.
	r := Rom{Metadatum: RomMetadata{FirstReleaseDate: 646790400000}}
	got := r.ToGame("", "", library.ArtPaths{})

	if !got.HasReleaseDate() {
		t.Fatal("expected a release date")
	}
	want := time.Unix(646790400, 0).UTC()
	if !got.ReleaseDate.Equal(want) {
		t.Errorf("ReleaseDate = %v, want %v", got.ReleaseDate, want)
	}

	// Zero means unknown, not the epoch.
	none := Rom{}.ToGame("", "", library.ArtPaths{})
	if none.HasReleaseDate() {
		t.Errorf("expected no release date, got %v", none.ReleaseDate)
	}
}

func TestToGame_Rating(t *testing.T) {
	rated := Rom{Metadatum: RomMetadata{AverageRating: 85}}.ToGame("", "", library.ArtPaths{})
	if !rated.HasRating() {
		t.Fatal("expected a rating")
	}
	if rated.Rating != 0.85 {
		t.Errorf("Rating = %v, want 0.85", rated.Rating)
	}

	unrated := Rom{}.ToGame("", "", library.ArtPaths{})
	if unrated.HasRating() {
		t.Errorf("expected no rating, got %v", unrated.Rating)
	}
}

// ScreenScraper companies are the fallback when RomM has none of its own.
func TestToGame_DevelopersFallBackToScreenScraper(t *testing.T) {
	r := Rom{ScreenScraperMetadata: ScreenScrapper{Companies: []string{"Sonic Team"}}}
	if got := r.ToGame("", "", library.ArtPaths{}).Developers; len(got) != 1 || got[0] != "Sonic Team" {
		t.Errorf("Developers = %v, want [Sonic Team]", got)
	}

	r.Metadatum.Companies = []string{"Sega"}
	if got := r.ToGame("", "", library.ArtPaths{}).Developers; len(got) != 1 || got[0] != "Sega" {
		t.Errorf("Developers = %v, want [Sega] when RomM supplies its own", got)
	}
}

// A rom with no FsName still needs an identity, so fall back to its file list.
func TestToGame_FileNameFallsBackToFileList(t *testing.T) {
	r := Rom{FsNameNoExt: "Sonic", Files: []RomFile{{FileName: "Sonic.gba"}}}
	if got := r.ToGame("", "", library.ArtPaths{}).FileName; got != "Sonic.gba" {
		t.Errorf("FileName = %q, want %q", got, "Sonic.gba")
	}
}

func TestToPlatform(t *testing.T) {
	got := Platform{ID: 3, FSSlug: "gba", Name: "Game Boy Advance"}.ToPlatform()
	if got.FSSlug != "gba" || got.Name != "Game Boy Advance" {
		t.Errorf("ToPlatform() = %+v", got)
	}
}
