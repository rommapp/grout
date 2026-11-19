package firmware

import (
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"grout/core/firmware/host"
)

func locDef(extra string) string {
	return `
id = "x"
[paths]
card = { value = "/mnt/SDCARD" }
roms = { value = "{card}/Roms" }
bios = { value = "{card}/Bios" }
saves = { value = "{card}/Saves" }
[saves]
layout = { value = "per-emulator" }
naming = { value = "retroarch" }
` + extra + `
[platforms]
gba = { roms = ["Game Boy Advance (GBA)", "Game Boy Advance (MGBA)"], saves = ["GBA", "MGBA"] }
psx = { roms = ["Sony PlayStation (PS)"], saves = ["PS"] }
"3do" = { roms = [], saves = [] }
`
}

func locFirmware(t *testing.T, def string, files fstest.MapFS, ov Overrides, reg Registry) *Firmware {
	t.Helper()
	f, err := resolveWith(mustParse(t, def), &host.Fake{Files: files}, ov, reg)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func dirs(locs []SaveLocation) []string {
	var out []string
	for _, l := range locs {
		out = append(out, l.Dir)
	}
	return out
}

func TestTag(t *testing.T) {
	for in, want := range map[string]string{
		"Game Boy Advance (MGBA)": "MGBA",
		"Foo (A) (B)":             "AB",
		"Plain":                   "",
		"":                        "",
	} {
		if got := Tag(in); got != want {
			t.Errorf("Tag(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRomDirsOverrideAndFallbacks(t *testing.T) {
	f := locFirmware(t, locDef(""), nil, Overrides{RomFolders: map[string]string{"psx": "Custom PS"}}, Registry{})
	if got := f.RomDirs("psx"); !reflect.DeepEqual(got, []string{"/mnt/SDCARD/Roms/Custom PS"}) {
		t.Fatalf("override: %v", got)
	}
	want := []string{"/mnt/SDCARD/Roms/Game Boy Advance (GBA)", "/mnt/SDCARD/Roms/Game Boy Advance (MGBA)"}
	if got := f.RomDirs("gba"); !reflect.DeepEqual(got, want) {
		t.Fatalf("row: %v", got)
	}
	if got := f.RomDirs("3do"); got != nil {
		t.Fatalf("empty roms: %v", got)
	}
	if got := f.RomDirs("NES"); !reflect.DeepEqual(got, []string{"/mnt/SDCARD/Roms/nes"}) {
		t.Fatalf("no row: %v", got)
	}
}

func TestSaveLocationsTagPicksFolder(t *testing.T) {
	f := locFirmware(t, locDef(""), nil, Overrides{}, Registry{})
	locs, dropped := f.SaveLocations("gba", "/mnt/SDCARD/Roms/Game Boy Advance (MGBA)/x.gba")
	want := []string{"/mnt/SDCARD/Saves/MGBA", "/mnt/SDCARD/Saves/GBA"}
	if !reflect.DeepEqual(dirs(locs), want) || len(dropped) != 0 {
		t.Fatalf("%+v %+v", locs, dropped)
	}
	if locs[0].Source != "definition" || locs[0].Label != "MGBA" {
		t.Fatalf("%+v", locs[0])
	}
}

func TestSaveLocationsBesideRoms(t *testing.T) {
	def := locDef("")
	def = replaceOnce(def, `value = "per-emulator"`, `value = "beside-roms"`)
	f := locFirmware(t, def, nil, Overrides{}, Registry{})
	locs, _ := f.SaveLocations("gba", "/mnt/SDCARD/Roms/Game Boy Advance (GBA)/x.gba")
	if !reflect.DeepEqual(dirs(locs), []string{"/mnt/SDCARD/Roms/Game Boy Advance (GBA)"}) {
		t.Fatalf("%+v", locs)
	}
}

func replaceOnce(s, old, repl string) string {
	for i := 0; i+len(old) <= len(s); i++ {
		if s[i:i+len(old)] == old {
			return s[:i] + repl + s[i+len(old):]
		}
	}
	return s
}

func TestSaveLocationsOverrideExists(t *testing.T) {
	files := fstest.MapFS{"mnt/SDCARD/Saves/Custom/keep": {}}
	f := locFirmware(t, locDef(""), files, Overrides{SaveFolders: map[string]string{"gba": "Custom"}}, Registry{})
	locs, dropped := f.SaveLocations("gba", "/mnt/SDCARD/Roms/Game Boy Advance (GBA)/x.gba")
	want := []string{"/mnt/SDCARD/Saves/Custom", "/mnt/SDCARD/Saves/GBA", "/mnt/SDCARD/Saves/MGBA"}
	if !reflect.DeepEqual(dirs(locs), want) || len(dropped) != 0 {
		t.Fatalf("%+v %+v", locs, dropped)
	}
	if locs[0].Source != "override" {
		t.Fatalf("%+v", locs[0])
	}
}

func TestSaveLocationsOverrideMissingIsDropped(t *testing.T) {
	f := locFirmware(t, locDef(""), nil, Overrides{SaveFolders: map[string]string{"gba": "Custom"}}, Registry{})
	locs, dropped := f.SaveLocations("gba", "/mnt/SDCARD/Roms/Game Boy Advance (GBA)/x.gba")
	for _, l := range locs {
		if l.Dir == "/mnt/SDCARD/Saves/Custom" {
			t.Fatalf("missing override returned: %+v", locs)
		}
	}
	want := []Dropped{{Slug: "gba", Dir: "/mnt/SDCARD/Saves/Custom", Reason: "override folder does not exist"}}
	if !reflect.DeepEqual(dropped, want) {
		t.Fatalf("%+v", dropped)
	}
}

func TestSaveLocationsPreferredHookLeads(t *testing.T) {
	var gotFolder string
	reg := Registry{Folders: map[string]FoldersHook{
		"pref": func(h host.Host, romFolder string) ([]string, bool) {
			gotFolder = romFolder
			return []string{"../pickles/sram/mGBA/GBA"}, true
		},
	}}
	files := fstest.MapFS{"mnt/SDCARD/Saves/Custom/keep": {}}
	def := locDef(`extra_folders_hook = "pref"`)
	f := locFirmware(t, def, files, Overrides{SaveFolders: map[string]string{"gba": "Custom"}}, reg)
	locs, _ := f.SaveLocations("gba", "/mnt/SDCARD/Roms/Game Boy Advance (GBA)/x.gba")
	if gotFolder != "Game Boy Advance (GBA)" {
		t.Fatalf("romFolder %q", gotFolder)
	}
	want := []string{"/mnt/SDCARD/pickles/sram/mGBA/GBA", "/mnt/SDCARD/Saves/Custom", "/mnt/SDCARD/Saves/GBA", "/mnt/SDCARD/Saves/MGBA"}
	if !reflect.DeepEqual(dirs(locs), want) {
		t.Fatalf("%v", dirs(locs))
	}
	if locs[0].Source != "hook" {
		t.Fatalf("%+v", locs[0])
	}
}

func TestSaveLocationsLabelHook(t *testing.T) {
	reg := Registry{Label: map[string]LabelHook{"lab": func(s string) string { return "L-" + s }}}
	f := locFirmware(t, locDef(`label_hook = "lab"`), nil, Overrides{}, reg)
	locs, _ := f.SaveLocations("psx", "/mnt/SDCARD/Roms/Sony PlayStation (PS)/x.cue")
	if len(locs) != 1 || locs[0].Label != "L-PS" {
		t.Fatalf("%+v", locs)
	}
}

func TestSaveNameFromHook(t *testing.T) {
	var vars map[string]string
	reg := Registry{Naming: map[string]NamingHook{
		"nm": func(h host.Host, v map[string]string) (bool, string, bool) {
			vars = v
			return true, "sav", true
		},
	}}
	files := fstest.MapFS{"mnt/SDCARD/Saves/GBA/Other.srm": {}}
	f := locFirmware(t, locDef(`naming_hook = "nm"`), files, Overrides{}, reg)
	if got := f.SaveName("Game.gba", "/mnt/SDCARD/Saves/GBA"); got != "Game.gba.sav" {
		t.Fatalf("%q", got)
	}
	if vars["card"] != "/mnt/SDCARD" || vars["roms"] != "/mnt/SDCARD/Roms" || vars["bios"] != "/mnt/SDCARD/Bios" || vars["saves"] != "/mnt/SDCARD/Saves" {
		t.Fatalf("%v", vars)
	}
}

func TestSaveNameInferredFromDisk(t *testing.T) {
	files := fstest.MapFS{
		"mnt/SDCARD/Saves/GBA/Other.gba.sav": {},
		"mnt/SDCARD/Saves/PS/Other.srm":      {},
		"mnt/SDCARD/Saves/PS/notes.txt":      {},
	}
	f := locFirmware(t, locDef(""), files, Overrides{}, Registry{})
	if got := f.SaveName("Game.gba", "/mnt/SDCARD/Saves/GBA"); got != "Game.gba.sav" {
		t.Fatalf("%q", got)
	}
	if got := f.SaveName("Game.cue", "/mnt/SDCARD/Saves/PS"); got != "Game.srm" {
		t.Fatalf("%q", got)
	}
}

func TestSaveNameFallback(t *testing.T) {
	f := locFirmware(t, locDef(""), nil, Overrides{}, Registry{})
	if got := f.SaveName("Game.gba", "/mnt/SDCARD/Saves/GBA"); got != "Game.srm" {
		t.Fatalf("%q", got)
	}
	def := replaceOnce(locDef(""), `value = "retroarch"`, `value = "keep-rom-ext"`)
	f = locFirmware(t, def, nil, Overrides{}, Registry{})
	if got := f.SaveName("Game.gba", "/mnt/SDCARD/Saves/GBA"); got != "Game.gba.sav" {
		t.Fatalf("%q", got)
	}
}

func TestSaveLocationsLabelHookGetsRelativeFolder(t *testing.T) {
	var got []string
	reg := Registry{
		Label: map[string]LabelHook{"lab": func(s string) string { got = append(got, s); return s }},
		Folders: map[string]FoldersHook{
			"pref": func(h host.Host, romFolder string) ([]string, bool) {
				return []string{"../pickles/sram/mGBA/GBA"}, false
			},
		},
	}
	def := `
id = "x"
[paths]
card = { value = "/mnt/SDCARD" }
roms = { value = "{card}/Roms" }
bios = { value = "{card}/Bios" }
saves = { value = "{card}/Saves" }
[saves]
layout = { value = "per-emulator" }
naming = { value = "retroarch" }
label_hook = "lab"
extra_folders_hook = "pref"
[platforms]
nds = { roms = ["NDS"], saves = ["drastic/backup"] }
gba = { roms = ["GBA"], saves = [] }
`
	f := locFirmware(t, def, nil, Overrides{}, reg)
	f.SaveLocations("nds", "/mnt/SDCARD/Roms/NDS/x.nds")
	f.SaveLocations("gba", "/mnt/SDCARD/Roms/GBA/x.gba")
	want := []string{"drastic/backup", "../pickles/sram/mGBA/GBA", "../pickles/sram/mGBA/GBA"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%q", got)
	}
}

func TestResolveUnregisteredSaveHooks(t *testing.T) {
	for _, line := range []string{`naming_hook = "n"`, `extra_folders_hook = "n"`, `label_hook = "n"`} {
		_, err := resolveWith(mustParse(t, locDef(line)), &host.Fake{}, Overrides{}, Registry{})
		if err == nil || !strings.Contains(err.Error(), "not registered") {
			t.Errorf("%s: %v", line, err)
		}
	}
}

func TestSaveNameIgnoresNonRomInnerExtension(t *testing.T) {
	files := fstest.MapFS{"mnt/SDCARD/Saves/GBA/Pokemon Hack v1.1.srm": {}}
	f := locFirmware(t, locDef(""), files, Overrides{}, Registry{})
	if got := f.SaveName("Game.gba", "/mnt/SDCARD/Saves/GBA"); got != "Game.srm" {
		t.Fatalf("%q", got)
	}
}

func TestSaveNameKeepsRomExtensionCaseInsensitively(t *testing.T) {
	files := fstest.MapFS{"mnt/SDCARD/Saves/GBA/Other.GBA.sav": {}}
	f := locFirmware(t, locDef(""), files, Overrides{}, Registry{})
	if got := f.SaveName("Game.gba", "/mnt/SDCARD/Saves/GBA"); got != "Game.gba.sav" {
		t.Fatalf("%q", got)
	}
}
