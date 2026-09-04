package download

import (
	"log/slog"

	"go.uber.org/atomic"

	"grout/romm"
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
