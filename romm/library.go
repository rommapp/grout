package romm

import (
	"time"

	"grout/library"
)

// ToGame converts a rom into the domain type. The caller supplies what RomM
// cannot know: how the name should read, where the rom landed, and where its
// artwork went.
// EntryFileName is the name a game's metadata entry goes by: fs_name, or its
// first file's name when a cached row has none. Anything looking an entry up
// again has to use the same name.
func (r Rom) EntryFileName() string {
	if r.FsName == "" && len(r.Files) > 0 {
		return r.Files[0].FileName
	}
	return r.FsName
}

func (r Rom) ToGame(displayName, path string, art library.ArtPaths) library.Game {
	fileName := r.EntryFileName()

	developers := r.Metadatum.Companies
	if len(developers) == 0 {
		developers = r.ScreenScraperMetadata.Companies
	}

	var released time.Time
	if r.Metadatum.FirstReleaseDate != 0 {
		released = time.Unix(r.Metadatum.FirstReleaseDate/1000, 0).UTC()
	}

	// Without game modes the count is a guess, which must not pass for one the
	// server knows.
	var maxPlayers int
	if len(r.Metadatum.GameModes) > 0 {
		maxPlayers = r.MaxPlayerCount()
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
		MaxPlayers:            maxPlayers,
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
