package firmware

import (
	"errors"
	"sort"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/rommapp/grout/core/firmware/host"
)

func mustParse(t *testing.T, s string) *Definition {
	t.Helper()
	d, err := Parse([]byte(s))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestResolvePathsInOrder(t *testing.T) {
	d := mustParse(t, `
id = "x"
[paths]
card = { value = "/mnt/SDCARD" }
roms = { value = "{card}/Roms" }
bios = { value = "{card}/Bios" }
saves = { value = "{card}/Saves" }
`)
	f, err := Resolve(d, &host.Fake{}, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if f.Card != "/mnt/SDCARD" || f.Roms != "/mnt/SDCARD/Roms" || f.Bios != "/mnt/SDCARD/Bios" || f.Saves != "/mnt/SDCARD/Saves" {
		t.Fatalf("%+v", f)
	}
}

func TestResolveVersionFromFile(t *testing.T) {
	d := mustParse(t, `
id = "x"
[version]
file = { value = "{card}/v.txt" }
[paths]
card = { value = "/mnt/SDCARD" }
roms = { value = "{card}/Roms" }
bios = { value = "{card}/Bios" }
saves = { value = "{card}/Saves" }
`)
	h := &host.Fake{Files: fstest.MapFS{"mnt/SDCARD/v.txt": {Data: []byte("6.4.1\nbuild")}}}
	f, err := Resolve(d, h, Overrides{})
	if err != nil || f.Version != "6.4.1" {
		t.Fatalf("%v %+v", err, f)
	}
	f, err = Resolve(d, &host.Fake{}, Overrides{})
	if err != nil || f.Version != "" {
		t.Fatalf("%v %+v", err, f)
	}
}

const altDef = `
id = "x"
[version]
file = { value = "{card}/v.txt" }
[paths]
card = { value = "/mnt/SDCARD" }
roms = [{ value = "{card}/ROMS", until = "2.0" }, { value = "{card}/Roms", since = "2.0" }]
bios = { value = "{card}/Bios" }
saves = { value = "{card}/Saves" }
`

func TestResolvePathAlternatives(t *testing.T) {
	d := mustParse(t, altDef)
	for ver, want := range map[string]string{"2.1": "/mnt/SDCARD/Roms", "1.9": "/mnt/SDCARD/ROMS"} {
		h := &host.Fake{Files: fstest.MapFS{"mnt/SDCARD/v.txt": {Data: []byte(ver)}}}
		f, err := Resolve(d, h, Overrides{})
		if err != nil || f.Roms != want {
			t.Fatalf("%s: %v %+v", ver, err, f)
		}
	}
	_, err := Resolve(d, &host.Fake{}, Overrides{})
	if err == nil || !strings.Contains(err.Error(), "paths.roms") {
		t.Fatalf("err=%v", err)
	}
}

func TestResolveDropsRowsOutsideVersion(t *testing.T) {
	d := mustParse(t, altDef+`
[platforms]
a = { roms = ["A"], saves = ["A"], since = "2.0" }
b = { roms = ["B"], saves = ["B"], until = "2.0" }
c = { roms = ["C"], saves = ["C"] }
`)
	h := &host.Fake{Files: fstest.MapFS{"mnt/SDCARD/v.txt": {Data: []byte("2.1")}}}
	f, err := Resolve(d, h, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for k := range f.Rows() {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if strings.Join(keys, ",") != "a,c" || len(d.Platforms) != 3 {
		t.Fatalf("%v", keys)
	}
}

func TestResolveDeviceRules(t *testing.T) {
	d := mustParse(t, `
id = "x"
[paths]
card = { value = "/c" }
roms = { value = "{card}/r" }
bios = { value = "{card}/b" }
saves = { value = "{card}/s" }
[devices]
default = "generic"
rules = [
  { device = "eq", env = "DEV", equals = "tg5040" },
  { device = "pre", env = "DEV", prefix = "tg" },
  { device = "file", file = "/proc/cpuinfo", contains = "RK3566" },
]
`)
	cases := []struct {
		name string
		h    *host.Fake
		want string
	}{
		{"equals", &host.Fake{Env: map[string]string{"DEV": "tg5040"}}, "eq"},
		{"prefix", &host.Fake{Env: map[string]string{"DEV": "tg3040"}}, "pre"},
		{"file", &host.Fake{Files: fstest.MapFS{"proc/cpuinfo": {Data: []byte("Hardware RK3566")}}}, "file"},
		{"none", &host.Fake{Env: map[string]string{"DEV": "zz"}}, "generic"},
	}
	for _, c := range cases {
		f, err := Resolve(d, c.h, Overrides{})
		if err != nil || f.Device != c.want {
			t.Fatalf("%s: %v %q", c.name, err, f.Device)
		}
	}
}

func TestResolvePathHook(t *testing.T) {
	d := mustParse(t, `
id = "x"
[paths]
card = { value = "/c" }
roms = { hook = "x_roms" }
bios = { value = "{card}/b" }
saves = { value = "{card}/s" }
`)
	reg := Registry{Path: map[string]PathHook{"x_roms": func(h host.Host, vars map[string]string) (string, error) {
		return vars["card"] + "/hooked/" + vars["muos_rom_storage"], nil
	}}}
	f, err := ResolveWith(d, &host.Fake{}, Overrides{MuOSRomStorage: "sd2"}, reg)
	if err != nil || f.Roms != "/c/hooked/sd2" {
		t.Fatalf("%v %+v", err, f)
	}
	_, err = ResolveWith(d, &host.Fake{}, Overrides{}, Registry{})
	if err == nil || !strings.Contains(err.Error(), "paths.roms") {
		t.Fatalf("err=%v", err)
	}
	boom := Registry{Path: map[string]PathHook{"x_roms": func(host.Host, map[string]string) (string, error) {
		return "", errors.New("boom")
	}}}
	if _, err = ResolveWith(d, &host.Fake{}, Overrides{}, boom); err == nil {
		t.Fatal("hook error swallowed")
	}
}

func TestActiveFirmware(t *testing.T) {
	get := func(v string) func(string) string {
		return func(k string) string {
			if k == "CFW" {
				return v
			}
			return ""
		}
	}
	id, err := activeIn(get(" TESTFW "), []string{"testfw"})
	if err != nil || id != "testfw" {
		t.Fatalf("%v %q", err, id)
	}
	_, err = activeIn(get("batocera"), []string{"testfw", "other"})
	if err == nil || !strings.Contains(err.Error(), "testfw, other") {
		t.Fatalf("err=%v", err)
	}
	if _, err = activeIn(get(""), []string{"testfw"}); err == nil {
		t.Fatal("empty accepted")
	}
	if _, err = Active(get("nope")); err == nil {
		t.Fatal("unknown accepted")
	}
}

func TestResolveEnvCard(t *testing.T) {
	d := mustParse(t, `
id = "x"
[paths]
card = { value = "{env.ROOT}" }
roms = { value = "{card}/Roms" }
bios = { value = "{card}/Bios" }
saves = { value = "{card}/Saves" }
`)
	f, err := Resolve(d, &host.Fake{Env: map[string]string{"ROOT": "/x"}}, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if f.Card != "/x" || f.Roms != "/x/Roms" {
		t.Fatalf("%+v", f)
	}
}

const envTemplateBase = `
id = "x"
[paths]
card = { value = "/x" }
roms = { value = "{card}/Roms" }
bios = { value = "{card}/Bios" }
saves = { value = "{card}/Saves" }
`

func TestResolveRequiresEnvInBiosRow(t *testing.T) {
	d := mustParse(t, envTemplateBase+`
[platforms]
gba = { roms = ["GBA"], saves = ["GBA"], bios = ["{env.CANNOLI_BIOS}/{rel}"] }
`)
	_, err := Resolve(d, &host.Fake{}, Overrides{})
	if err == nil || !strings.Contains(err.Error(), "environment variable CANNOLI_BIOS is not set") {
		t.Fatalf("got %v", err)
	}
	f, err := Resolve(d, &host.Fake{Env: map[string]string{"CANNOLI_BIOS": "/b"}}, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if got := f.BiosPaths("gba", "a.bin"); len(got) != 1 || got[0] != "/b/a.bin" {
		t.Fatalf("%v", got)
	}
}

func TestResolveRequiresEnvInArtTemplate(t *testing.T) {
	d := mustParse(t, envTemplateBase+`
[art]
cover = { value = "{env.ART_ROOT}/{system}" }
`)
	_, err := Resolve(d, &host.Fake{}, Overrides{})
	if err == nil || !strings.Contains(err.Error(), "environment variable ART_ROOT is not set") {
		t.Fatalf("got %v", err)
	}
	if _, err := Resolve(d, &host.Fake{Env: map[string]string{"ART_ROOT": "/a"}}, Overrides{}); err != nil {
		t.Fatal(err)
	}
}
