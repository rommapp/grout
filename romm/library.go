package romm

import (
	"time"

	"grout/domain/library"
)

// ToGame converts a rom into the domain type. The caller supplies what RomM
// cannot know: how the name should read, where the rom landed, and where its
// artwork went.
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

func (p Platform) ToPlatform() library.Platform {
	return library.Platform{FSSlug: p.FSSlug, Name: p.Name}
}
