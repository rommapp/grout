package firmware

import (
	"errors"
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
