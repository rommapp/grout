package library

import (
	"context"
	"reflect"
	"testing"

	"grout/core/config"
	"grout/core/firmware"
	"grout/core/firmware/host"
	"grout/core/romm"
)

func newPlatformsLibrary(t *testing.T, cfg *config.Config, fw *firmware.Firmware, ps ...romm.Platform) *Library {
	t.Helper()
	l := New(openTestStore(t), nil, cfg, fw)
	tx, err := l.Store.DB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err := savePlatforms(tx, ps); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	return l
}

func mappings(slugs ...string) map[string]config.DirectoryMapping {
	m := map[string]config.DirectoryMapping{}
	for _, s := range slugs {
		m[s] = config.DirectoryMapping{RomMSlug: s}
	}
	return m
}

func platformNames(ps []Platform) []string {
	var out []string
	for _, p := range ps {
		out = append(out, p.Name)
	}
	return out
}

func nextUIFirmware(t *testing.T) *firmware.Firmware {
	t.Helper()
	d, err := firmware.Load("nextui")
	if err != nil {
		t.Fatal(err)
	}
	fw, err := firmware.Resolve(d, &host.Fake{}, firmware.Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	return fw
}

func TestPlatformsEnabledWithGamesOnly(t *testing.T) {
	cfg := &config.Config{DirectoryMappings: mappings("gba", "gb")}
	l := newPlatformsLibrary(t, cfg, nil,
		romm.Platform{ID: 1, Slug: "gba", FSSlug: "gba", Name: "Game Boy Advance", RomCount: 4},
		romm.Platform{ID: 2, Slug: "gb", FSSlug: "gb", Name: "Game Boy", RomCount: 0},
		romm.Platform{ID: 3, Slug: "snes", FSSlug: "snes", Name: "SNES", RomCount: 9},
	)
	got, err := l.Platforms(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].FSSlug != "gba" || !got[0].Enabled || got[0].RomCount != 4 {
		t.Fatalf("got %+v", got)
	}
	all, err := l.AllPlatforms(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("all = %+v", all)
	}
}

func TestPlatformsOrder(t *testing.T) {
	cfg := &config.Config{DirectoryMappings: mappings("snes", "gb", "atari2600"), PlatformOrder: []string{"snes"}}
	l := newPlatformsLibrary(t, cfg, nil,
		romm.Platform{ID: 1, Slug: "gb", FSSlug: "gb", Name: "game boy", RomCount: 1},
		romm.Platform{ID: 2, Slug: "atari2600", FSSlug: "atari2600", Name: "Atari 2600", RomCount: 1},
		romm.Platform{ID: 3, Slug: "snes", FSSlug: "snes", Name: "Super Nintendo", RomCount: 1},
	)
	got, err := l.Platforms(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Super Nintendo", "Atari 2600", "game boy"}
	if !reflect.DeepEqual(platformNames(got), want) {
		t.Fatalf("got %v want %v", platformNames(got), want)
	}
}

func TestPlatformsCustomName(t *testing.T) {
	cfg := &config.Config{DirectoryMappings: mappings("gb")}
	l := newPlatformsLibrary(t, cfg, nil,
		romm.Platform{ID: 1, Slug: "gb", FSSlug: "gb", Name: "Game Boy", CustomName: "GB Classic", RomCount: 1},
	)
	got, err := l.Platforms(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Name != "GB Classic" {
		t.Fatalf("got %q", got[0].Name)
	}
}

func TestPlatformsDisambiguate(t *testing.T) {
	cfg := &config.Config{DirectoryMappings: mappings("fbneo", "mame", "gb")}
	l := newPlatformsLibrary(t, cfg, nil,
		romm.Platform{ID: 1, Slug: "arcade", FSSlug: "fbneo", Name: "Arcade", RomCount: 1},
		romm.Platform{ID: 2, Slug: "arcade", FSSlug: "mame", Name: "Arcade", RomCount: 1},
		romm.Platform{ID: 3, Slug: "gb", FSSlug: "gb", Name: "Game Boy", RomCount: 1},
	)
	got, err := l.Platforms(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Arcade (fbneo)", "Arcade (mame)", "Game Boy"}
	if !reflect.DeepEqual(platformNames(got), want) {
		t.Fatalf("got %v want %v", platformNames(got), want)
	}
}

func TestRomDirsMappingThenFirmware(t *testing.T) {
	fw := nextUIFirmware(t)
	cfg := &config.Config{DirectoryMappings: map[string]config.DirectoryMapping{
		"gba": {RomMSlug: "gba", RelativePath: "GBA Custom"},
	}}
	l := New(openTestStore(t), nil, cfg, fw)
	p := Platform{Slug: "gba", FSSlug: "gba"}
	want := []string{fw.Roms + "/GBA Custom"}
	if got := l.RomDirs(p); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	cfg.DirectoryMappings["gba"] = config.DirectoryMapping{RomMSlug: "gba"}
	want = []string{fw.Roms + "/Game Boy Advance (GBA)", fw.Roms + "/Game Boy Advance (MGBA)"}
	if got := l.RomDirs(p); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestProposedFollowsFirmware(t *testing.T) {
	fw := nextUIFirmware(t)
	cfg := &config.Config{DirectoryMappings: mappings("gba", "3ds")}
	l := newPlatformsLibrary(t, cfg, fw,
		romm.Platform{ID: 1, Slug: "gba", FSSlug: "gba", Name: "Game Boy Advance", RomCount: 1},
		romm.Platform{ID: 2, Slug: "3ds", FSSlug: "3ds", Name: "Nintendo 3DS", RomCount: 1},
	)
	got, err := l.Platforms(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	proposed := map[string]bool{}
	for _, p := range got {
		proposed[p.Slug] = p.Proposed
	}
	if !proposed["gba"] || proposed["3ds"] {
		t.Fatalf("proposed = %v", proposed)
	}
}
