package firmware

import (
	"slices"
	"strings"
	"testing"
)

func TestExpand(t *testing.T) {
	got, err := Expand("{card}/Roms/{rom_folder}", map[string]string{"card": "/mnt/SDCARD", "rom_folder": "GBA"})
	if err != nil || got != "/mnt/SDCARD/Roms/GBA" {
		t.Fatalf("%q %v", got, err)
	}
	if _, err := Expand("{card}/{nope}", map[string]string{"card": "/x"}); err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("err=%v", err)
	}
}

func TestExpandLeavesOtherBracesAlone(t *testing.T) {
	got, err := Expand("a{b c}{}{x", map[string]string{})
	if err != nil || got != "a{b c}{}{x" {
		t.Fatalf("%q %v", got, err)
	}
}

func TestVariables(t *testing.T) {
	got := Variables("{bios}/{save_folder}/{name}/{bios}")
	if !slices.Equal(got, []string{"bios", "save_folder", "name"}) {
		t.Fatalf("%v", got)
	}
	if len(Variables("/plain")) != 0 {
		t.Fatal("plain")
	}
}
