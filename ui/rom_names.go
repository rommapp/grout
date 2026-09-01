package ui

import (
	"slices"
	"strings"

	"grout/internal/stringutil"
	"grout/romm"
)

// prepareRomNames fills in each rom's DisplayName and sorts the slice by name.
//
// DisplayName is presentation only. It folds in the region, rewrites
// punctuation, and changes with user settings, so nothing may key off it --
// identity comes from the rom's file name. This lives in ui rather than
// stringutil because it is the one thing that made a pure string package depend
// on the RomM wire types.
func prepareRomNames(games []romm.Rom) []romm.Rom {
	for i := range games {
		games[i].DisplayName = stringutil.PrepareRomName(games[i].Name, games[i].Regions)
	}

	slices.SortFunc(games, func(a, b romm.Rom) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})

	return games
}
