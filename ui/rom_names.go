package ui

import "grout/romm"

// romArtFileName returns the file name used for artwork naming, or "" when the
// rom has no file list. See cfw.ArtFileName.
func romArtFileName(g romm.Rom) string {
	if len(g.Files) > 0 {
		return g.Files[0].FileName
	}
	return ""
}
