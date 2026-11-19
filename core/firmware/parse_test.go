package firmware

import (
	"io/fs"
	"slices"
	"strings"
	"testing"
)

func TestParseRejectsUnknownKeys(t *testing.T) {
	_, err := Parse([]byte("id = \"x\"\nmaintained = \"community\"\nnmae = \"typo\"\n"))
	if err == nil || !strings.Contains(err.Error(), "nmae") {
		t.Fatalf("err=%v", err)
	}
}

func TestParseRejectsUnknownFactKeys(t *testing.T) {
	_, err := Parse([]byte("id = \"x\"\n[paths]\ncard = { valeu = \"/a\" }\n"))
	if err == nil || !strings.Contains(err.Error(), "valeu") {
		t.Fatalf("err=%v", err)
	}
}

func TestParseFactForms(t *testing.T) {
	d, err := Parse([]byte(`
id = "x"
maintained = "community"
[version]
file = { value = "{card}/v.txt" }
[paths]
card = { value = "/mnt/SDCARD" }
roms = { hook = "x_roms" }
bios = [{ value = "{card}/BIOS", until = "2.0" }, { value = "{card}/Bios", since = "2.0" }]
[bios]
default = { value = ["{bios}/{rel}", "{bios}/{name}"] }
[platforms]
gba = { roms = ["GBA"], saves = ["mGBA"], since = "1.2" }
[devices.inputs]
a = "a.json"
b = { hook = "x_input" }
`))
	if err != nil {
		t.Fatal(err)
	}
	if d.Paths.Card.String() != "/mnt/SDCARD" || d.Paths.Roms.Hook != "x_roms" {
		t.Fatalf("paths %+v", d.Paths)
	}
	if len(d.Paths.Bios.Alts) != 2 || d.Paths.Bios.Alts[1].Since != "2.0" {
		t.Fatalf("alternatives %+v", d.Paths.Bios)
	}
	if got := d.Bios.Default.Strings(); len(got) != 2 || got[1] != "{bios}/{name}" {
		t.Fatalf("bios %v", got)
	}
	if d.Platforms["gba"].Since != "1.2" || d.Devices.Inputs["a"].File != "a.json" || d.Devices.Inputs["b"].Hook != "x_input" {
		t.Fatalf("rows %+v inputs %+v", d.Platforms, d.Devices.Inputs)
	}
}

func TestParseVersionForms(t *testing.T) {
	d, err := Parse([]byte("id = \"x\"\n[version]\nfile = { value = \"/v\", src = \"a/b\" }\n"))
	if err != nil || d.Version.File.String() != "/v" || d.Version.File.Src != "a/b" {
		t.Fatalf("d=%+v err=%v", d, err)
	}
	d, err = Parse([]byte("id = \"x\"\n[version]\nhook = \"x_version\"\n"))
	if err != nil || d.Version.Hook != "x_version" {
		t.Fatalf("d=%+v err=%v", d, err)
	}
}

func TestParseRemainingSections(t *testing.T) {
	d, err := Parse([]byte(`
id = "x"
upstream = { repo = "github.com/a/b", ref = "v1" }
[saves]
layout = { value = "per-emulator" }
naming_hook = "x_n"
[art]
naming = "plain"
cover = { value = "{rom_dir}/.media" }
[gamelist]
format = "none"
launcher_entry = { gamelist = "{rom_dir}/g.xml", path = "./Grout" }
[retroarch]
config = { value = "{card}/ra.cfg" }
[platform_sources]
roms = "skeleton/Roms"
[devices]
default = "generic"
rules = [{ device = "d", env = "E", equals = "v" }, { device = "e", file = "/f", contains = "z" }]
[devices.quirks]
d = { rotation = 90, disable_keyboard_joystick = true }
[packaging]
asset = "G.zip"
launch_script = "launch.sh"
install_depth = 1
files = ["launch.sh"]
`))
	if err != nil {
		t.Fatal(err)
	}
	if d.Upstream.Ref != "v1" || d.Saves.NamingHook != "x_n" || d.Art.Cover == nil || d.Art.Video != nil {
		t.Fatalf("%+v", d)
	}
	if d.Gamelist.LauncherEntry == nil || d.Gamelist.LauncherEntry.Path != "./Grout" || d.RetroArch == nil || d.RetroArch.Overrides != nil {
		t.Fatalf("%+v", d)
	}
	if len(d.Devices.Rules) != 2 || d.Devices.Rules[1].Contains != "z" || d.Devices.Quirks["d"].Rotation != 90 || !d.Devices.Quirks["d"].DisableKeyboardJoystick {
		t.Fatalf("%+v", d.Devices)
	}
	if d.Packaging.InstallDepth != 1 || d.PlatformSources["roms"] != "skeleton/Roms" {
		t.Fatalf("%+v", d)
	}
}

func TestLoadEmbeddedPlaceholder(t *testing.T) {
	d, err := Load("testfw")
	if err != nil || d.ID != "testfw" {
		t.Fatalf("d=%v err=%v", d, err)
	}
	if slices.Contains(IDs(), "testfw") {
		t.Fatal("placeholder listed as a firmware")
	}
	if _, err := InputFile("testfw", "test.json"); err != nil {
		t.Fatal(err)
	}
	all, err := LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range all {
		if f.ID == "testfw" {
			t.Fatal("LoadAll returned placeholder")
		}
	}
}

func TestLoadUnknownID(t *testing.T) {
	if _, err := Load("nope"); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseRejectsUnknownKeyInKnownTable(t *testing.T) {
	_, err := Parse([]byte("id = \"x\"\n[paths]\nbogus = \"y\"\n"))
	if err == nil || !strings.Contains(err.Error(), "bogus") {
		t.Fatalf("err=%v", err)
	}
}

func TestParseRejectsMisplacedFactKeys(t *testing.T) {
	cases := map[string]string{
		"src":   "[platforms]\ngba = { roms = [\"a\"], src = \"zz\" }\n",
		"since": "[devices]\nrules = [{ device = \"d\", env = \"E\", equals = \"v\", since = \"1\" }]\n",
		"hook":  "[devices.quirks]\nd = { rotation = 90, hook = \"h\" }\n",
		"value": "[devices.quirks]\nd = { rotation = 90, value = \"h\" }\n",
		"until": "[gamelist]\nlauncher_entry = { gamelist = \"a\", path = \"b\", hook = \"c\" }\n",
	}
	for key, body := range cases {
		_, err := Parse([]byte("id = \"x\"\n" + body))
		if err == nil {
			t.Fatalf("%s: expected error", key)
		}
		want := key
		if key == "until" {
			want = "hook"
		}
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("%s: err=%v", key, err)
		}
	}
}

func TestMockFSWithoutMockDirIsEmptyDirectory(t *testing.T) {
	m, err := MockFS("nomock")
	if err != nil {
		t.Fatal(err)
	}
	entries, err := fs.ReadDir(m, ".")
	if err != nil || len(entries) != 0 {
		t.Fatalf("%v %v", entries, err)
	}
}
