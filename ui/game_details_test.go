package ui

import (
	"strings"
	"testing"

	"grout/romm"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

func labelled(facts []gaba.MetadataItem, label string) (string, bool) {
	for _, fact := range facts {
		if fact.Label == label {
			return fact.Value, true
		}
	}
	return "", false
}

// A game RomM knows little about must not show a column of blank rows.
func TestGameFacts_SkipsWhatIsNotKnown(t *testing.T) {
	facts := gameFacts(romm.Rom{Name: "Homebrew"})

	if len(facts) != 0 {
		t.Errorf("facts = %v, want none for a game with no metadata", facts)
	}
}

func TestGameFacts_Values(t *testing.T) {
	game := romm.Rom{
		Name:        "Mario",
		Regions:     []string{"USA", "EUR"},
		Languages:   []string{"En"},
		FsSizeBytes: 4 * 1024 * 1024,
	}
	game.Metadatum.Genres = []string{"Platform", "Adventure"}
	game.Metadatum.AverageRating = 92.4

	facts := gameFacts(game)

	want := map[string]string{
		"Genres":         "Platform, Adventure",
		"Regions":        "USA, EUR",
		"Languages":      "En",
		"Average Rating": "92.4/100",
	}
	for label, value := range want {
		got, ok := labelled(facts, label)
		if !ok {
			t.Errorf("no %q row", label)
			continue
		}
		if got != value {
			t.Errorf("%s = %q, want %q", label, got, value)
		}
	}

	if size, ok := labelled(facts, "File Size"); !ok || !strings.Contains(size, "MB") {
		t.Errorf("File Size = %q, want it in readable units", size)
	}
}

// The rows keep a fixed order so the screen does not rearrange itself
// depending on which metadata a game happens to carry.
func TestGameFacts_Order(t *testing.T) {
	game := romm.Rom{Regions: []string{"USA"}, Languages: []string{"En"}, FsSizeBytes: 1024}
	game.Metadatum.Genres = []string{"Platform"}

	facts := gameFacts(game)

	order := make([]string, 0, len(facts))
	for _, fact := range facts {
		order = append(order, fact.Label)
	}

	want := []string{"Genres", "Regions", "Languages", "File Size"}
	for i, label := range want {
		if i >= len(order) || order[i] != label {
			t.Fatalf("order = %v, want %v", order, want)
		}
	}
}

// A multi-file game says so, and one that is not does not carry an empty row
// for it.
func TestGameFacts_MultiFile(t *testing.T) {
	if _, ok := labelled(gameFacts(romm.Rom{Name: "Mario"}), "Type"); ok {
		t.Error("a single-file game must have no Type row")
	}

	facts := gameFacts(romm.Rom{Name: "FF7", HasMultipleFiles: true})
	if value, ok := labelled(facts, "Type"); !ok || value == "" {
		t.Errorf("Type = %q, want a multi-file game to say so", value)
	}
}

func TestDownloadLabel(t *testing.T) {
	if downloadLabel(true) == downloadLabel(false) {
		t.Error("having a game already and not having it must read differently")
	}
}
