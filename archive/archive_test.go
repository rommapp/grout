package archive

import "testing"

func TestIsArchive(t *testing.T) {
	tests := map[string]bool{
		"Game.zip": true,
		"Game.7z":  true,
		"Game.ZIP": true,
		"Game.chd": false,
		"Game":     false,
		"":         false,
	}

	for name, want := range tests {
		if got := IsArchive(name); got != want {
			t.Errorf("IsArchive(%q) = %v, want %v", name, got, want)
		}
	}
}
