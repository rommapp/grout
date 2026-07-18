package ui

import (
	"testing"

	"grout/romm"
)

func TestHasFilterableMetadata(t *testing.T) {
	tests := []struct {
		name  string
		games []romm.Rom
		want  bool
	}{
		{"nil", nil, false},
		{"no metadata", []romm.Rom{{ID: 1, Name: "Homebrew"}}, false},
		{"genres present", []romm.Rom{{ID: 1, Metadatum: romm.RomMetadata{Genres: []string{"Action"}}}}, true},
		{"only companies", []romm.Rom{{ID: 1, Metadatum: romm.RomMetadata{Companies: []string{"Nintendo"}}}}, true},
		{"only region (top-level field)", []romm.Rom{{ID: 1, Regions: []string{"USA"}}}, true},
		{"only language", []romm.Rom{{ID: 1, Languages: []string{"En"}}}, true},
		{
			"metadata on a later game",
			[]romm.Rom{{ID: 1, Name: "A"}, {ID: 2, Metadatum: romm.RomMetadata{GameModes: []string{"Single player"}}}},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasFilterableMetadata(tt.games); got != tt.want {
				t.Errorf("hasFilterableMetadata() = %v, want %v", got, tt.want)
			}
		})
	}
}
