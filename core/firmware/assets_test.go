package firmware

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/rommapp/grout/core/firmware/host"
)

func biosFW(t *testing.T, biosRoot, body string) *Firmware {
	t.Helper()
	d := mustParse(t, `
id = "x"
[paths]
card = { value = "/x" }
roms = { value = "{card}/Roms" }
bios = { value = "`+biosRoot+`" }
saves = { value = "{card}/Saves" }
`+body)
	f, err := ResolveWith(d, &host.Fake{}, Overrides{}, Registry{})
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func eq(t *testing.T, got, want []string) {
	t.Helper()
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestBiosPathsSaveFolderExpansion(t *testing.T) {
	f := biosFW(t, "/x/Bios", `
[bios]
default = { value = ["{bios}/{save_folder}/{name}"] }
[platforms]
gba = { roms = ["GBA"], saves = ["GBA", "MGBA"] }
psx = { roms = ["PS"], saves = [] }
`)
	eq(t, f.BiosPaths("gba", "gba_bios.bin"), []string{"/x/Bios/GBA/gba_bios.bin", "/x/Bios/MGBA/gba_bios.bin"})
	eq(t, f.BiosPaths("psx", "scph1001.bin"), []string{"/x/Bios/scph1001.bin"})
}

func TestBiosPathsRowOverride(t *testing.T) {
	f := biosFW(t, "/x/BIOS", `
[bios]
default = { value = ["{bios}/{name}"] }
[platforms]
amiga = { roms = ["AMIGA"], saves = ["PUAE"], bios = ["{bios}/puae/{name}"] }
`)
	eq(t, f.BiosPaths("amiga", "kick34005.A500"), []string{"/x/BIOS/puae/kick34005.A500"})
}

func TestBiosPathsDefault(t *testing.T) {
	f := biosFW(t, "/x/Bios", `
[platforms]
gba = { roms = ["GBA"], saves = ["GBA"] }
`)
	eq(t, f.BiosPaths("gba", "sub/gba_bios.bin"), []string{"/x/Bios/sub/gba_bios.bin"})
	eq(t, f.BiosPaths("nope", "a.bin"), []string{"/x/Bios/a.bin"})

	g := biosFW(t, "/x/Bios", `
[bios]
default = { value = ["{bios}/{name}", "{bios}/x/{name}", "{bios}/{name}"] }
`)
	eq(t, g.BiosPaths("gba", "sub/a.bin"), []string{"/x/Bios/a.bin", "/x/Bios/x/a.bin"})
}

func TestArtDirKinds(t *testing.T) {
	f := biosFW(t, "/x/Bios", `
[art]
cover = { value = "{card}/info/catalogue/{system}/box" }
preview = { value = "{rom_dir}/.media/{rom_folder}" }
splash = { value = "{card}/{nope}" }
[platforms]
gba = { roms = ["GBA"], saves = ["GBA"], system = "Nintendo Game Boy Advance" }
nes = { roms = ["NES"], saves = ["NES"] }
`)
	got, ok := f.ArtDir(ArtCover, "gba", "/x/Roms/GBA", "Game Boy Advance")
	if !ok || got != "/x/info/catalogue/Nintendo Game Boy Advance/box" {
		t.Fatalf("%q %v", got, ok)
	}
	got, ok = f.ArtDir(ArtCover, "nes", "/x/Roms/NES", "Nintendo Entertainment System")
	if !ok || got != "/x/info/catalogue/Nintendo Entertainment System/box" {
		t.Fatalf("%q %v", got, ok)
	}
	got, ok = f.ArtDir(ArtPreview, "nes", "/x/Roms/NES", "n")
	if !ok || got != "/x/Roms/NES/.media/NES" {
		t.Fatalf("%q %v", got, ok)
	}
	if _, ok := f.ArtDir(ArtVideo, "gba", "/x/Roms/GBA", "n"); ok {
		t.Fatal("video should be absent")
	}
	if _, ok := f.ArtDir(ArtSplash, "gba", "/x/Roms/GBA", "n"); ok {
		t.Fatal("bad template should give false")
	}
}

func TestInputMappingFileAndNone(t *testing.T) {
	d, err := Load("testfw")
	if err != nil {
		t.Fatal(err)
	}
	f, err := ResolveWith(d, &host.Fake{}, Overrides{}, Registry{})
	if err != nil {
		t.Fatal(err)
	}
	want, err := InputFile("testfw", "test.json")
	if err != nil {
		t.Fatal(err)
	}
	got, err := f.InputMapping()
	if err != nil || string(got) != string(want) {
		t.Fatalf("%q %v", got, err)
	}

	f.Device = "other"
	got, err = f.InputMapping()
	if got != nil || err != nil {
		t.Fatalf("%q %v", got, err)
	}

	hd := mustParse(t, `
id = "x"
[paths]
card = { value = "/x" }
roms = { value = "{card}/Roms" }
bios = { value = "{card}/Bios" }
saves = { value = "{card}/Saves" }
[devices]
default = "d"
[devices.inputs]
d = { hook = "x_input" }
`)
	reg := Registry{Input: map[string]InputHook{"x_input": func(host.Host) ([]byte, error) { return []byte("hooked"), nil }}}
	hf, err := ResolveWith(hd, &host.Fake{}, Overrides{}, reg)
	if err != nil {
		t.Fatal(err)
	}
	if b, err := hf.InputMapping(); err != nil || string(b) != "hooked" {
		t.Fatalf("%q %v", b, err)
	}
	hf.reg = Registry{}
	if _, err := hf.InputMapping(); err == nil || !strings.Contains(err.Error(), "devices.inputs.d") {
		t.Fatalf("%v", err)
	}
	hf.reg = Registry{Input: map[string]InputHook{"x_input": func(host.Host) ([]byte, error) { return nil, errors.New("boom") }}}
	if _, err := hf.InputMapping(); err == nil {
		t.Fatal("hook error expected")
	}
}

func TestQuirksForDevice(t *testing.T) {
	d := mustParse(t, `
id = "x"
[paths]
card = { value = "/x" }
roms = { value = "{card}/Roms" }
bios = { value = "{card}/Bios" }
saves = { value = "{card}/Saves" }
[devices]
default = "generic"
[devices.quirks]
flip = { rotation = 90, disable_keyboard_joystick = true }
`)
	f, err := ResolveWith(d, &host.Fake{}, Overrides{}, Registry{})
	if err != nil {
		t.Fatal(err)
	}
	if f.Quirks() != (Quirks{}) {
		t.Fatalf("%+v", f.Quirks())
	}
	f.Device = "flip"
	if q := f.Quirks(); q.Rotation != 90 || !q.DisableKeyboardJoystick {
		t.Fatalf("%+v", q)
	}
}

func layoutFW(t *testing.T, body string) *Firmware {
	t.Helper()
	d := mustParse(t, `
id = "x"
[paths]
card = { value = "/x" }
roms = { value = "{card}/Roms" }
bios = { value = "{card}/Bios" }
saves = { value = "{card}/Saves" }
`+body)
	f, err := ResolveWith(d, &host.Fake{}, Overrides{}, Hooks())
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestPlainMultiLayout(t *testing.T) {
	f := layoutFW(t, "")
	m3u := []byte("a.chd\nb.chd\n")
	got := f.MultiLayout("Game", m3u)
	if got.Folder != "Game" || got.Beside || string(got.Playlist) != string(m3u) {
		t.Fatalf("%+v", got)
	}
	got = f.MultiLayout("Game", nil)
	if got.Folder != "Game" || got.Beside || got.Playlist != nil {
		t.Fatalf("%+v", got)
	}
}

func TestMuosMultiLayoutThroughFirmware(t *testing.T) {
	f := layoutFW(t, `
[rom_folders]
multi_layout = "muos_hidden_layout"
`)
	got := f.MultiLayout("Game", []byte("a.chd\nb.chd\n"))
	if got.Folder != "_Game" || !got.Beside || string(got.Playlist) != "_Game/a.chd\n_Game/b.chd\n" {
		t.Fatalf("%+v", got)
	}
	got = f.MultiLayout("Game", nil)
	if got.Folder != "_Game" || !got.Beside || got.Playlist != nil {
		t.Fatalf("%+v", got)
	}
}

func TestMultiLayoutHookMustBeRegistered(t *testing.T) {
	d := mustParse(t, `
id = "x"
[paths]
card = { value = "/x" }
roms = { value = "{card}/Roms" }
bios = { value = "{card}/Bios" }
saves = { value = "{card}/Saves" }
[rom_folders]
multi_layout = "nope"
`)
	if _, err := ResolveWith(d, &host.Fake{}, Overrides{}, Registry{}); err == nil || !strings.Contains(err.Error(), "rom_folders.multi_layout") {
		t.Fatalf("%v", err)
	}
}

func TestArtKindsCoversSchema(t *testing.T) {
	var body strings.Builder
	body.WriteString("[art]\n")
	for _, k := range ArtKinds() {
		body.WriteString(string(k) + ` = { value = "{rom_dir}/` + string(k) + `" }` + "\n")
	}
	f := biosFW(t, "/x/Bios", body.String())
	seen := map[string]bool{}
	for _, k := range ArtKinds() {
		fact := f.artFact(k)
		if fact == nil {
			t.Fatalf("%s has no artFact case", k)
		}
		if fact.String() != "{rom_dir}/"+string(k) || seen[fact.String()] {
			t.Fatalf("%s reads %q", k, fact.String())
		}
		seen[fact.String()] = true
	}
	art := reflect.TypeOf(f.Def.Art)
	facts := 0
	for i := 0; i < art.NumField(); i++ {
		if art.Field(i).Type == reflect.TypeOf(&Fact{}) {
			facts++
		}
	}
	if facts != len(ArtKinds()) {
		t.Fatalf("schema has %d art facts, ArtKinds has %d", facts, len(ArtKinds()))
	}
}

func TestArtFileNamePlain(t *testing.T) {
	f := biosFW(t, "/x/Bios", "[art]\nnaming = \"plain\"\n")
	want := map[ArtKind]string{
		ArtVideo:  "Sonic.mp4",
		ArtManual: "Sonic.pdf",
	}
	for _, k := range ArtKinds() {
		w, ok := want[k]
		if !ok {
			w = "Sonic.png"
		}
		if got := f.ArtFileName(k, "Sonic"); got != w {
			t.Errorf("%s: got %q want %q", k, got, w)
		}
	}
	g := biosFW(t, "/x/Bios", "")
	if got := g.ArtFileName(ArtThumbnail, "Sonic"); got != "Sonic.png" {
		t.Fatalf("%q", got)
	}
}

func TestArtFileNameESSuffix(t *testing.T) {
	f := biosFW(t, "/x/Bios", "[art]\nnaming = \"es-suffix\"\n")
	want := map[ArtKind]string{
		ArtCover:     "Sonic.png",
		ArtPreview:   "Sonic.png",
		ArtSplash:    "Sonic.png",
		ArtVideo:     "Sonic.mp4",
		ArtManual:    "Sonic.pdf",
		ArtBezel:     "Sonic.png",
		ArtMarquee:   "Sonic-marquee.png",
		ArtThumbnail: "Sonic-thumb.png",
		ArtBoxback:   "Sonic-boxback.png",
		ArtFanart:    "Sonic-fanart.png",
	}
	if len(want) != len(ArtKinds()) {
		t.Fatalf("table has %d kinds, ArtKinds has %d", len(want), len(ArtKinds()))
	}
	for _, k := range ArtKinds() {
		w, ok := want[k]
		if !ok {
			t.Fatalf("%s missing from table", k)
		}
		if got := f.ArtFileName(k, "Sonic"); got != w {
			t.Errorf("%s: got %q want %q", k, got, w)
		}
	}
}

func TestBiosPathsWithRel(t *testing.T) {
	rels := BiosRels("DC_BOOT.BIN")
	if len(rels) != 1 {
		t.Fatalf("%v", rels)
	}
	muos := biosFW(t, "/x/bios", `
[bios]
default = { value = ["{bios}/{rel}"] }
[platforms]
dc = { roms = ["dc"], saves = ["file/Flycast"] }
`)
	eq(t, muos.BiosPaths("dc", rels[0]), []string{"/x/bios/dc/dc_boot.bin"})
	nextui := biosFW(t, "/x/Bios", `
[bios]
default = { value = ["{bios}/{save_folder}/{name}"] }
[platforms]
dc = { roms = ["Dreamcast (DC)"], saves = ["DC"] }
`)
	eq(t, nextui.BiosPaths("dc", rels[0]), []string{"/x/Bios/DC/dc_boot.bin"})
}
