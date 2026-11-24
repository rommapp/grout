package ident

import (
	"maps"
	"testing"

	"github.com/rommapp/grout/core/firmware"
)

func TestPlatformMapsRommSlugsToSigil(t *testing.T) {
	want := map[string]string{
		"psx": "psx", "ps2": "ps2", "psp": "psp", "psvita": "psvita", "ps3": "ps3",
		"switch": "switch", "3ds": "3ds", "new-nintendo-3ds": "3ds",
		"wii": "wii", "wiiu": "wiiu", "ngc": "gamecube", "dc": "dreamcast",
		"xbox": "xbox", "xbox360": "xbox360",
		"gb": "gb", "gbc": "gbc", "snes": "snes", "sfam": "snes", "n64": "n64",
	}
	if got := Platforms(); !maps.Equal(got, want) {
		t.Fatalf("Platforms() = %v, want %v", got, want)
	}
	for _, s := range []string{"gba", "nes", "64dd", "genesis", ""} {
		if _, ok := Platform(s); ok {
			t.Errorf("Platform(%q) mapped, want none", s)
		}
	}
}

func TestPlatformsReturnsACopy(t *testing.T) {
	p := Platforms()
	p["psx"] = "changed"
	if got, _ := Platform("psx"); got != "psx" {
		t.Fatalf("Platform(psx) = %q after mutating the copy", got)
	}
}

func TestEveryMappedSlugIsARommSlug(t *testing.T) {
	known, err := firmware.KnownSlugs()
	if err != nil {
		t.Fatal(err)
	}
	for slug := range Platforms() {
		if !known[slug] {
			t.Errorf("%s is not a RomM slug", slug)
		}
	}
}

func TestStatuses(t *testing.T) {
	want := []Status{StatusOK, StatusNotFound, StatusNeedsKey, StatusUnsupported, StatusIO, StatusFailed}
	got := Statuses()
	if len(got) != len(want) {
		t.Fatalf("Statuses() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Statuses() = %v, want %v", got, want)
		}
	}
}
