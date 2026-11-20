package main

import (
	"strings"
	"testing"

	"github.com/rommapp/grout/core/firmware"
)

const minimal = `
id = "nextui"
name = "NextUI"
maintained = "upstream"
upstream = { repo = "github.com/LoveRetro/NextUI", ref = "v6.4.1" }

[paths]
card = { value = "/mnt/SDCARD", src = "skeleton/SYSTEM" }
roms = { value = "{card}/Roms", src = "skeleton/BASE/Roms" }
bios = { value = "{card}/Bios", src = "skeleton/BASE/Bios" }
saves = { value = "{card}/Saves", src = "skeleton/BASE/Saves" }

[platforms]
gba = { roms = ["Game Boy Advance (GBA)", "Game Boy Advance (MGBA)"], saves = ["GBA", "MGBA"] }
"3do" = { roms = [], saves = [] }

[packaging]
asset = "Grout.pak.zip"
launch_script = "launch.sh"
install_depth = 1
`

func mustParse(t *testing.T, src string) *firmware.Definition {
	t.Helper()
	d, err := firmware.Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestRenderStableAndLinked(t *testing.T) {
	d := mustParse(t, minimal)
	a, b := render([]*firmware.Definition{d}), render([]*firmware.Definition{d})
	if a != b {
		t.Fatal("unstable output")
	}
	if !strings.Contains(a, "https://github.com/LoveRetro/NextUI/blob/v6.4.1/skeleton/BASE/Roms") {
		t.Fatal("src not linked")
	}
	if !strings.HasSuffix(a, "\n") || strings.HasSuffix(a, "\n\n") {
		t.Fatal("must end with a single newline")
	}
	if !strings.Contains(a, "Game Boy Advance (GBA), Game Boy Advance (MGBA)<br>saves: GBA, MGBA") {
		t.Fatal("platform cell missing")
	}
	if !strings.Contains(a, "| 3do | - |") {
		t.Fatal("explicit empty roms should render -")
	}
}

func TestRenderHookAndRange(t *testing.T) {
	d := mustParse(t, `
id = "muos"
name = "muOS"
maintained = "upstream"
upstream = { repo = "github.com/example/muos", ref = "v1" }

[paths]
card = { value = "/mnt/SDCARD" }
roms = { hook = "x_roms" }
bios = [{ value = "{card}/a|b", src = "p", until = "2.0" }, { value = "{card}/B", src = "q", since = "2.0" }]
saves = { value = "{card}/Saves" }

[packaging]
asset = "a.zip"
launch_script = "launch.sh"
install_depth = 1
`)
	out := render([]*firmware.Definition{d})
	if !strings.Contains(out, "hook: x_roms") {
		t.Fatal("hook not rendered")
	}
	if !strings.Contains(out, "(until 2.0)<br>") || !strings.Contains(out, "(since 2.0)") {
		t.Fatalf("ranges not rendered:\n%s", out)
	}
	if !strings.Contains(out, `a\|b`) {
		t.Fatal("pipe not escaped")
	}
}

func TestRenderCommunityPlain(t *testing.T) {
	d := mustParse(t, `
id = "leaf"
name = "Leaf"
maintained = "community"

[paths]
card = { value = "/mnt/SDCARD" }
roms = { value = "{card}/Roms" }
bios = { value = "{card}/Bios" }
saves = { value = "{card}/Saves" }

[packaging]
asset = "a.zip"
launch_script = "launch.sh"
install_depth = 1
`)
	out := render([]*firmware.Definition{d})
	if strings.Contains(out, "](https://") {
		t.Fatalf("community firmware must not link:\n%s", out)
	}
}

func TestRenderEmpty(t *testing.T) {
	out := render(nil)
	if !strings.Contains(out, "No firmware definitions yet.") {
		t.Fatal("empty placeholder missing")
	}
}

const full = `
id = "muos"
name = "muOS"
maintained = "upstream"
upstream = { repo = "github.com/example/muos", ref = "v2" }

[paths]
card = { value = "/mnt/SDCARD" }
roms = { value = "{card}/Roms" }
bios = { value = "{card}/Bios" }
saves = { value = "{card}/Saves" }

[bios]
default = { value = ["{bios}/{name}", "{bios}/x/{name}"], src = "bios/path" }

[retroarch]
config = { value = "/opt/retroarch.cfg", src = "ra/cfg" }
overrides = { value = "/opt/overrides", src = "ra/ov" }

[gamelist]
format = "muos-text"
launcher_entry = { gamelist = "{roms}/gl.txt", path = "Grout" }

[platform_sources]
roms = "extras/Roms"
saves = "extras/Saves"

[platforms]
nes = { roms = ["Nintendo"], saves = ["NES"], system = "Nintendo NES", bios = ["{bios}/nes/{name}"], since = "1.0" }
saveonly = { roms = [], saves = ["SO"] }
none = { roms = [], saves = [] }

[devices]
default = "generic"
rules = [
  { device = "a", env = "DEV", equals = "x" },
  { device = "b", env = "DEV", prefix = "y" },
  { device = "c", file = "/proc/cpuinfo", contains = "z" },
]

[devices.inputs]
a = "a.json"
b = { hook = "muos_b" }

[devices.quirks]
a = { rotation = 90, disable_keyboard_joystick = true }

[packaging]
asset = "a.zip"
launch_script = "launch.sh"
install_depth = 1
`

func TestRenderEveryFact(t *testing.T) {
	out := render([]*firmware.Definition{mustParse(t, full)})
	for _, want := range []string{
		"| nes | Nintendo<br>saves: NES<br>system: Nintendo NES<br>bios: {bios}/nes/{name} (since 1.0) |",
		"| saveonly | -<br>saves: SO |",
		"| none | - |",
		"| sources | roms: [extras/Roms](https://github.com/example/muos/blob/v2/extras/Roms)<br>saves: [extras/Saves](https://github.com/example/muos/blob/v2/extras/Saves) |",
		"| default | generic |",
		"a: env DEV = x<br>b: env DEV prefix y<br>c: file /proc/cpuinfo contains z",
		"a: a.json<br>b: hook: muos_b",
		"a: rotation 90, disable_keyboard_joystick",
		"{roms}/gl.txt in Grout",
		"[/opt/retroarch.cfg](https://github.com/example/muos/blob/v2/ra/cfg)",
		"[/opt/overrides](https://github.com/example/muos/blob/v2/ra/ov)",
		"[{bios}/{name}, {bios}/x/{name}](https://github.com/example/muos/blob/v2/bios/path)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestRenderEscapesSrcPaths(t *testing.T) {
	src := strings.Replace(minimal, `src = "skeleton/BASE/Roms"`, `src = "skeleton/My Dir (x)/Roms"`, 1)
	src = strings.Replace(src, "[platforms]", "[platform_sources]\nroms = \"static/Some Pack (v2)/cfg\"\nsaves = \"static/Some Pack (v2)/saves\"\n\n[platforms]", 1)
	out := render([]*firmware.Definition{mustParse(t, src)})
	for _, want := range []string{
		"/blob/v6.4.1/skeleton/My%20Dir%20%28x%29/Roms)",
		"/blob/v6.4.1/static/Some%20Pack%20%28v2%29/cfg)",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q", want)
		}
	}
}
