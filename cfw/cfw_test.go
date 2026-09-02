package cfw

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// All is what the conformance tables range over, so a firmware missing from it
// is a firmware nothing checks.
func TestAll_ContainsEverySupportedFirmware(t *testing.T) {
	declared := []CFW{
		NextUI, MuOS, Knulli, Spruce, ROCKNIX, Trimui,
		Allium, Onion, Koriki, ArkOS, Batocera, MinUI,
	}

	for _, c := range declared {
		if !slices.Contains(All, c) {
			t.Errorf("%s is a supported firmware but is missing from All", c)
		}
	}
	if len(All) != len(declared) {
		t.Errorf("All has %d entries, want %d", len(All), len(declared))
	}

	seen := map[CFW]bool{}
	for _, c := range All {
		if seen[c] {
			t.Errorf("%s appears in All more than once", c)
		}
		seen[c] = true
		if !c.Supported() {
			t.Errorf("%s is in All but Supported reports false", c)
		}
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		in   string
		want CFW
	}{
		{"MUOS", MuOS},
		{"muos", MuOS},
		{"MuOS", MuOS},
		{"  rocknix  ", ROCKNIX},
		{"NEXTUI", NextUI},
		{"batocera", Batocera},
	}

	for _, tt := range tests {
		got, err := Parse(tt.in)
		if err != nil {
			t.Errorf("Parse(%q): %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("Parse(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// The error has to tell a person what to put in the environment variable,
// since it is set by a launch script they may have to edit by hand.
func TestParse_UnsupportedNamesTheOptions(t *testing.T) {
	for _, in := range []string{"", "NOPE", "windows", "muo"} {
		got, err := Parse(in)
		if err == nil {
			t.Errorf("Parse(%q) = %q, want an error", in, got)
			continue
		}
		if !errors.Is(err, ErrUnsupported) {
			t.Errorf("Parse(%q) error should wrap ErrUnsupported, got %v", in, err)
		}
		for _, c := range All {
			if !strings.Contains(err.Error(), string(c)) {
				t.Errorf("Parse(%q) error should list %s; got %v", in, c, err)
				break
			}
		}
	}
}

func TestActive(t *testing.T) {
	t.Setenv(EnvVar, "SPRUCE")
	got, err := Active()
	if err != nil {
		t.Fatalf("Active: %v", err)
	}
	if got != Spruce {
		t.Errorf("Active() = %q, want %q", got, Spruce)
	}

	t.Setenv(EnvVar, "NOT_A_FIRMWARE")
	if _, err = Active(); err == nil {
		t.Error("expected an error for an unrecognised firmware")
	}
}

// GetCFW used to call log.Fatalf, so an unset or wrong environment variable
// terminated the process -- including from path helpers that run while drawing
// a screen, and from any test that forgot to set it. It now returns the empty
// firmware, which every switch already handles as "no such thing here".
func TestGetCFW_UnknownFirmwareDoesNotTerminate(t *testing.T) {
	t.Setenv(EnvVar, "NOT_A_FIRMWARE")
	if got := GetCFW(); got != "" {
		t.Errorf("GetCFW() = %q, want empty", got)
	}

	t.Setenv(EnvVar, "")
	if got := GetCFW(); got != "" {
		t.Errorf("GetCFW() with an unset variable = %q, want empty", got)
	}

	// And the empty firmware must be safe to pass around.
	if ArtDirectory("", ArtCover, "/roms/gba", "gba", "Game Boy Advance") != "" {
		t.Error("the empty firmware should have no art directory")
	}
	if CFW("").IsBasedOnEmulationStation() {
		t.Error("the empty firmware is not EmulationStation based")
	}
}

func TestIsBasedOnEmulationStation(t *testing.T) {
	es := []CFW{Knulli, ROCKNIX, ArkOS, Batocera}
	for _, c := range All {
		want := slices.Contains(es, c)
		if got := c.IsBasedOnEmulationStation(); got != want {
			t.Errorf("%s.IsBasedOnEmulationStation() = %v, want %v", c, got, want)
		}
	}
}
