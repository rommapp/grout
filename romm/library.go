package romm

import (
	"time"

	"grout/domain/library"
)

// ToGame converts a rom as RomM described it into the domain type the metadata
// writers work with.
//
// The caller supplies the three things RomM cannot know: how the name should
// read, where the rom ended up on disk, and where its artwork was written.
// Rendering the display name here rather than downstream is deliberate -- it
// keeps text transformation at the boundary, so nothing further down can
// confuse a rendered name for identity.
func (r Rom) ToGame(displayName, path string, art library.ArtPaths) library.Game {
	fileName := r.FsName
	if fileName == "" && len(r.Files) > 0 {
		fileName = r.Files[0].FileName
	}

	developers := r.Metadatum.Companies
	if len(developers) == 0 {
		developers = r.ScreenScraperMetadata.Companies
	}

	var released time.Time
	if r.Metadatum.FirstReleaseDate != 0 {
		released = time.Unix(r.Metadatum.FirstReleaseDate/1000, 0).UTC()
	}

	var rating float64
	if r.Metadatum.AverageRating != 0 {
		rating = r.Metadatum.AverageRating / 100
	}

	return library.Game{
		FileName:              fileName,
		BaseName:              r.FsNameNoExt,
		Path:                  path,
		DisplayName:           displayName,
		Summary:               r.Summary,
		Regions:               r.Regions,
		Languages:             r.Languages,
		Genres:                r.Metadatum.Genres,
		Developers:            developers,
		ReleaseDate:           released,
		Rating:                rating,
		MaxPlayers:            r.MaxPlayerCount(),
		MD5:                   r.Md5Hash,
		ScreenScraperID:       r.ScreenScraperID,
		RetroAchievementsID:   r.RetroAchievementsID,
		RetroAchievementsHash: r.RetroAchievementsHash,
		Art:                   art,
	}
}

// ToPlatform converts a platform into the domain type.
func (p Platform) ToPlatform() library.Platform {
	return library.Platform{FSSlug: p.FSSlug, Name: p.Name}
}
