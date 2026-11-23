package firmware

import (
	"io/fs"
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/rommapp/grout/core/firmware/host"
)

func loadResolved(t *testing.T, id string, h host.Host) *Firmware {
	t.Helper()
	d, err := Load(id)
	if err != nil {
		t.Fatal(err)
	}
	f, err := Resolve(d, h, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestGamelistFormatsListed(t *testing.T) {
	want := []GamelistFormat{FormatNone, FormatEmulationStation, FormatMiyoo, FormatMuOSText}
	if got := GamelistFormats(); !reflect.DeepEqual(got, want) {
		t.Fatalf("GamelistFormats = %v, want %v", got, want)
	}
}

func TestGamelistFormatDefaultsToNone(t *testing.T) {
	f := exitFW(t, "", Registry{})
	if got := f.GamelistFormat(); got != FormatNone {
		t.Fatalf("GamelistFormat = %q, want none", got)
	}
	if _, ok := f.GamelistTextDir("gba", "/x/Roms/GBA", "Game Boy Advance"); ok {
		t.Fatal("GamelistTextDir set without gamelist.text")
	}
	if _, ok := f.LauncherEntry(); ok {
		t.Fatal("LauncherEntry set without launcher_entry")
	}
	g := exitFW(t, "[gamelist]\nformat = \"miyoo\"\n", Registry{})
	if got := g.GamelistFormat(); got != FormatMiyoo {
		t.Fatalf("GamelistFormat = %q, want miyoo", got)
	}
}

func TestGamelistTextDirExpands(t *testing.T) {
	h := &host.Fake{Files: fstest.MapFS{"mnt/union/ROMS": &fstest.MapFile{Mode: fs.ModeDir | 0o755}}}
	f := loadResolved(t, "muos", h)
	if got := f.GamelistFormat(); got != FormatMuOSText {
		t.Fatalf("GamelistFormat = %q", got)
	}
	got, ok := f.GamelistTextDir("gba", "/mnt/union/ROMS/gba", "Game Boy Advance")
	if !ok || got != "/run/muos/storage/info/catalogue/Nintendo Game Boy Advance/text" {
		t.Fatalf("GamelistTextDir = %q, %v", got, ok)
	}
	got, ok = f.GamelistTextDir("not-a-row", "/mnt/union/ROMS/x", "Fallback Name")
	if !ok || got != "/run/muos/storage/info/catalogue/Fallback Name/text" {
		t.Fatalf("GamelistTextDir fallback = %q, %v", got, ok)
	}
}

func TestLauncherEntryExpands(t *testing.T) {
	f := loadResolved(t, "knulli", &host.Fake{Files: fstest.MapFS{}})
	got, ok := f.LauncherEntry()
	want := Launcher{Gamelist: "/userdata/roms/tools/gamelist.xml", Path: "./Grout/Grout.sh"}
	if !ok || got != want {
		t.Fatalf("LauncherEntry = %+v, %v, want %+v", got, ok, want)
	}
}
