package ui

import (
	"strings"
	"testing"

	"grout/catalog"
	"grout/settings"

	gabaconst "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
)

// The markers say two different things at once: how much of a game is on the
// device, and whether it is one file or several. A game can need both.
func TestEntryText(t *testing.T) {
	tests := []struct {
		name  string
		entry catalog.GameEntry
		want  string
	}{
		{
			"plain game",
			catalog.GameEntry{Name: "Mario"},
			"Mario",
		},
		{
			"downloaded",
			catalog.GameEntry{Name: "Mario", Downloaded: catalog.FullyDownloaded},
			gabaconst.Download + " Mario",
		},
		{
			"several files, none downloaded",
			catalog.GameEntry{Name: "Final Fantasy VII", MultipleFiles: true},
			settings.MultipleFilesIcon + " Final Fantasy VII",
		},
		{
			"several files, some downloaded",
			catalog.GameEntry{Name: "Final Fantasy VII", MultipleFiles: true, Downloaded: catalog.PartlyDownloaded},
			gabaconst.Download + " " + settings.MultipleFilesIcon + " Final Fantasy VII",
		},
		{
			"several files, all downloaded",
			catalog.GameEntry{Name: "Final Fantasy VII", MultipleFiles: true, Downloaded: catalog.FullyDownloaded},
			settings.MultipleDownloadedIcon + " " + settings.MultipleFilesIcon + " Final Fantasy VII",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := entryText(tt.entry); got != tt.want {
				t.Errorf("entryText = %q, want %q", got, tt.want)
			}
		})
	}
}

// A partly downloaded game must not read the same as a finished one, or the
// marker stops telling the user anything.
func TestEntryText_PartlyDiffersFromFully(t *testing.T) {
	entry := catalog.GameEntry{Name: "Game", MultipleFiles: true}

	entry.Downloaded = catalog.PartlyDownloaded
	partly := entryText(entry)
	entry.Downloaded = catalog.FullyDownloaded
	fully := entryText(entry)

	if partly == fully {
		t.Errorf("both read %q", partly)
	}
}

// The name is the last thing on the row, so a marker can never be mistaken for
// part of the title.
func TestEntryText_NameComesLast(t *testing.T) {
	entry := catalog.GameEntry{Name: "Mario", MultipleFiles: true, Downloaded: catalog.FullyDownloaded}

	if got := entryText(entry); !strings.HasSuffix(got, "Mario") {
		t.Errorf("entryText = %q, want it to end with the game's name", got)
	}
}
