package download

import (
	"grout/cfw"
	"grout/files"
	"grout/gamelist"
	"grout/library"
	"grout/romm"
	"grout/settings"
	"grout/textmatch"
)

// LocalGame is a game already on the device and the file that holds it.
//
// The caller finds the file: knowing what is downloaded takes the library
// cache, which this package must not depend on.
type LocalGame struct {
	Rom  romm.Rom
	Path string
}

// MetadataEntries rebuilds the metadata entries of the games already on the
// device, from what the server knows about them now. Games without a local
// file are left out.
// Artwork is pointed at only where its file is present, so a refresh never
// leaves an entry naming art that was not downloaded.
func MetadataEntries(config settings.Config, platform romm.Platform, games []LocalGame) []gamelist.RomGameEntry {
	activeCFW := cfw.GetCFW()
	isESBased := activeCFW.IsBasedOnEmulationStation()

	entries := make([]gamelist.RomGameEntry, 0, len(games))
	for _, local := range games {
		game, path := local.Rom, local.Path
		if path == "" {
			continue
		}
		gamePlatform := platformFor(platform, game)

		regions := game.Regions
		if config.GamelistOmitsRegion {
			regions = nil
		}

		entries = append(entries, gamelist.RomGameEntry{
			Game:         game.ToGame(textmatch.PrepareRomName(game.Name, regions), path, presentArt(config, game, gamePlatform, activeCFW, isESBased)),
			Platform:     gamePlatform.ToPlatform(),
			RomDirectory: cfw.PlatformRomDirectory(config, gamePlatform.FSSlug),
		})
	}
	return entries
}

// PlatformGames is a platform and some of its games on the device.
type PlatformGames struct {
	Platform romm.Platform
	Games    []LocalGame
}

// MetadataResult is how a metadata refresh went.
type MetadataResult struct {
	// Updated games had their metadata written.
	Updated int
	// Failed games were on the device but could not be written.
	Failed int
}

// RefreshMetadata rewrites the metadata of the games already on the device.
// What the frontend recorded about each game, such as play count or
// favourites, is kept.
//
// The result counts only what was actually written. A failure is returned
// alongside it rather than instead of it, since the rest of the run may have
// landed.
func RefreshMetadata(config settings.Config, batches []PlatformGames) (MetadataResult, error) {
	var entries []gamelist.RomGameEntry
	for _, batch := range batches {
		entries = append(entries, MetadataEntries(config, batch.Platform, batch.Games)...)
	}
	if len(entries) == 0 {
		return MetadataResult{}, nil
	}

	written, err := cfw.RefreshGamesMetadata(entries)
	return MetadataResult{Updated: written, Failed: len(entries) - written}, err
}

// presentArt records the art of a game whose file is on the device, wherever
// the firmware would look for it
func presentArt(config settings.Config, game romm.Rom, platform romm.Platform, activeCFW cfw.CFW, isESBased bool) library.ArtPaths {
	var paths library.ArtPaths
	for _, spec := range artSpecs {
		location := spec.location(config, game, platform, activeCFW)
		if location != "" && files.FileExists(location) {
			spec.record(&paths, location, isESBased)
		}
	}
	return paths
}
