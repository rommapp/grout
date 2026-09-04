package download

import (
	"log/slog"

	"go.uber.org/atomic"

	"grout/cfw"
	"grout/files"
	"grout/library"
	"grout/romm"
	"grout/settings"
)

// FetchArt downloads the artwork belonging to the games whose rom actually
// arrived, reporting progress from 0 to 1.
//
// Art for a game whose rom failed is skipped: a cover with no game to go with
// it is clutter the frontend has to be told to ignore. Individual failures are
// logged and passed over, since a missing cover is not worth losing a download
// run to.
//
// The fetcher is built by the caller because normalising an image needs the
// display, which this package must not know about.
func FetchArt(fetcher *romm.ArtFetcher, items []Item, games []romm.Rom, progress *atomic.Float64) {
	arrived := make(map[string]bool, len(games))
	for _, game := range games {
		arrived[game.Name] = true
	}

	wanted := make([]Item, 0, len(items))
	for _, item := range items {
		if arrived[item.GameName] {
			wanted = append(wanted, item)
		}
	}
	if len(wanted) == 0 {
		return
	}

	var succeeded, failed int
	for i, item := range wanted {
		save := fetcher.SaveRaw
		if item.IsImage {
			save = fetcher.Save
		}

		if err := save(item.URL, item.Location); err != nil {
			slog.Default().Warn("Failed to download art",
				"game", item.GameName, "url", item.URL, "location", item.Location, "error", err)
			failed++
		} else {
			succeeded++
		}

		if progress != nil {
			progress.Store(float64(i+1) / float64(len(wanted)))
		}
	}

	slog.Default().Debug("Art download complete", "succeeded", succeeded, "failed", failed)
}

// ArtFor lists the artwork a game should have on the device, given what the
// config asks to be downloaded.
//
// This is the same list a download run fetches, so a screen that backfills
// missing art and a screen that downloads a game agree on which files should
// exist and what each one is called. Deciding that separately is how art comes
// to be fetched again on every sync forever: one side writes a name the other
// never looks for.
func ArtFor(config settings.Config, host settings.Host, game romm.Rom, platform romm.Platform) []Item {
	if !hasCoverArt(game) {
		return nil
	}

	activeCFW := cfw.GetCFW()
	return artItems(config, host, game, platform, activeCFW, activeCFW.IsBasedOnEmulationStation(), &library.ArtPaths{})
}

// Missing keeps the items whose file is not on the device yet.
func Missing(items []Item) []Item {
	absent := make([]Item, 0, len(items))
	for _, item := range items {
		if !files.FileExists(item.Location) {
			absent = append(absent, item)
		}
	}
	return absent
}

// Images keeps the artwork and drops the videos and manuals. They arrive with
// a game, but fetching one per game across a whole library is not what a
// backfill of artwork offers.
func Images(items []Item) []Item {
	images := make([]Item, 0, len(items))
	for _, item := range items {
		if item.IsImage {
			images = append(images, item)
		}
	}
	return images
}
