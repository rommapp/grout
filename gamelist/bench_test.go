package gamelist

import (
	"fmt"
	"strings"
	"testing"
)

// libraryGamelist returns a gamelist of n entries and the entries refreshing
// all of them, in the order a library refresh would send them.
func libraryGamelist(b *testing.B, n int) (string, []RomGameEntry) {
	b.Helper()
	var xml strings.Builder
	xml.WriteString("<gameList>")
	entries := make([]RomGameEntry, n)
	for i := range n {
		fileName := fmt.Sprintf("Game %04d.gba", i)
		fmt.Fprintf(&xml, "<game><path>./%s</path><name>Game %d</name><playcount>%d</playcount></game>", fileName, i, i)
		r := rom(fmt.Sprintf("Game %d", i), fileName, "USA")
		r.Summary = "Refreshed"
		r.Genres = []string{"Platform"}
		entries[i] = entry(r, "/roms/gba")
	}
	xml.WriteString("</gameList>")
	return xml.String(), entries
}

// A refresh of a whole library looks every game up in a file holding as many.
// The file must be indexed once for the batch, not scanned once per game.
func BenchmarkRefreshRomGame_Library(b *testing.B) {
	xml, entries := libraryGamelist(b, 3000)
	for b.Loop() {
		b.StopTimer()
		gl := New()
		if err := gl.Parse([]byte(xml)); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
		gl.RefreshRomGames(entries)
	}
}
