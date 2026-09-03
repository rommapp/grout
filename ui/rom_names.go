package ui

import (
	"slices"
	"strings"

	"grout/romm"
	"grout/textmatch"
)

// prepareRomNames fills in each rom's DisplayName and sorts by name.
// DisplayName is presentation only; nothing may key off it.
func prepareRomNames(games []romm.Rom) []romm.Rom {
	for i := range games {
		games[i].DisplayName = textmatch.PrepareRomName(games[i].Name, games[i].Regions)
	}

	slices.SortFunc(games, func(a, b romm.Rom) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})

	return games
}

// romArtFileName returns the file name used for artwork naming, or "" when the
// rom has no file list. See cfw.ArtFileName.
func romArtFileName(g romm.Rom) string {
	if len(g.Files) > 0 {
		return g.Files[0].FileName
	}
	return ""
}
