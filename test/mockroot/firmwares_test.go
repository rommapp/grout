package mockroot

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"grout/core/firmware"
	"grout/core/firmware/host"
)

func TestFirmwareMockRoots(t *testing.T) {
	for _, tc := range []struct {
		firmware string
		device   string
	}{
		{"nextui", ""},
		{"nextui", "generic"},
		{"nextui", "trimui"},
		{"nextui", "miyooflip"},
		{"muos", ""},
		{"muos", "anbernic"},
		{"muos", "trimui"},
		{"muos", "trimui-smart-pro"},
		{"knulli", ""},
		{"knulli", "generic"},
		{"rocknix", ""},
		{"rocknix", "generic"},
		{"rocknix", "rgds"},
		{"spruce", ""},
		{"spruce", "generic"},
		{"spruce", "a30"},
		{"spruce", "miyoomini"},
		{"spruce", "miyooflip"},
		{"spruce", "trimui"},
		{"spruce", "pixel"},
		{"spruce", "h700"},
		{"allium", ""},
		{"allium", "miyoo"},
		{"onion", ""},
		{"onion", "miyoo"},
		{"leaf", ""},
		{"leaf", "mlp1"},
	} {
		name := tc.firmware + "/" + tc.device
		if tc.device == "" {
			name = tc.firmware + "/default"
		}
		t.Run(name, func(t *testing.T) {
			d, err := firmware.Load(tc.firmware)
			if err != nil {
				t.Fatal(err)
			}
			want := tc.device
			if want == "" {
				want = d.Devices.Default
			}
			dir := filepath.Join(t.TempDir(), "root")
			env, err := Build(dir, Options{Firmware: tc.firmware, Device: tc.device})
			if err != nil {
				t.Fatal(err)
			}
			f, err := firmware.Resolve(d, host.OS(envLookup(env)), firmware.Overrides{})
			if err != nil {
				t.Fatal(err)
			}
			if f.Device != want {
				t.Errorf("device = %q, want %q", f.Device, want)
			}
			for slug := range f.Rows() {
				dirs := f.RomDirs(slug)
				if len(dirs) == 0 {
					continue
				}
				exists(t, filepath.Join(dir, filepath.FromSlash(dirs[0])))
			}
		})
	}
}

func TestNextUIMockRootSaveNaming(t *testing.T) {
	d, err := firmware.Load("nextui")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "root")
	env, err := Build(dir, Options{Firmware: "nextui"})
	if err != nil {
		t.Fatal(err)
	}
	f, err := firmware.Resolve(d, host.OS(envLookup(env)), firmware.Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if f.Card != "/mnt/SDCARD" || f.Roms != "/mnt/SDCARD/Roms" || f.Saves != "/mnt/SDCARD/Saves" || f.Bios != "/mnt/SDCARD/Bios" {
		t.Errorf("paths = %q %q %q %q", f.Card, f.Roms, f.Saves, f.Bios)
	}
	if got := f.SaveName("Game.gba", "/mnt/SDCARD/Saves/GBA"); got != "Game.gba.sav" {
		t.Errorf("SaveName = %q, want Game.gba.sav", got)
	}
	locs, _ := f.SaveLocations("gba", "/mnt/SDCARD/Roms/Game Boy Advance (MGBA)/Game.gba")
	if len(locs) == 0 || locs[0].Dir != "/mnt/SDCARD/Saves/MGBA" {
		t.Errorf("SaveLocations = %+v, want MGBA first", locs)
	}
}

func TestMuOSMockRoot(t *testing.T) {
	d, err := firmware.Load("muos")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "root")
	env, err := Build(dir, Options{Firmware: "muos"})
	if err != nil {
		t.Fatal(err)
	}
	f, err := firmware.Resolve(d, host.OS(envLookup(env)), firmware.Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if f.Version != "2606.0_ANDROMEDA" {
		t.Errorf("version = %q", f.Version)
	}
	if f.Card != "/run/muos/storage" || f.Roms != "/mnt/mmc/ROMS" || f.Saves != "/run/muos/storage/save" || f.Bios != "/run/muos/storage/bios" {
		t.Errorf("paths = %q %q %q %q", f.Card, f.Roms, f.Saves, f.Bios)
	}
	locs, _ := f.SaveLocations("gba", "/mnt/mmc/ROMS/gba/Game.gba")
	if len(locs) == 0 || locs[0].Dir != "/run/muos/storage/pickles/sram/mgba/gba" || !locs[0].Preferred || locs[0].Label != "Pickles (mgba)" {
		t.Errorf("SaveLocations = %+v, want the Pickles mgba folder first", locs)
	}
	for _, l := range locs {
		if l.Dir == "/run/muos/storage/save/file/mGBA" && l.Label != "mGBA" {
			t.Errorf("label for %s = %q, want mGBA", l.Dir, l.Label)
		}
	}
	if dir, ok := f.ArtDir(firmware.ArtCover, "gba", "/mnt/mmc/ROMS/gba", "Game Boy Advance"); !ok || dir != "/run/muos/storage/info/catalogue/Nintendo Game Boy Advance/box" {
		t.Errorf("cover dir = %q %v", dir, ok)
	}
	mapping, err := f.InputMapping()
	if err != nil || len(mapping) == 0 {
		t.Errorf("input mapping = %d bytes, err %v", len(mapping), err)
	}
}

func TestMuOSMockRootSD2Storage(t *testing.T) {
	d, err := firmware.Load("muos")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "root")
	env, err := Build(dir, Options{Firmware: "muos"})
	if err != nil {
		t.Fatal(err)
	}
	exists(t, filepath.Join(dir, "mnt", "mmc", "ROMS"))
	if err := os.MkdirAll(filepath.Join(dir, "mnt", "sdcard", "ROMS"), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := firmware.Resolve(d, host.OS(envLookup(env)), firmware.Overrides{MuOSRomStorage: "sdcard"})
	if err != nil {
		t.Fatal(err)
	}
	if f.Roms != "/mnt/sdcard/ROMS" {
		t.Errorf("roms = %q, want the chosen SD2", f.Roms)
	}
}

func TestKnulliMockRoot(t *testing.T) {
	d, err := firmware.Load("knulli")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "root")
	env, err := Build(dir, Options{Firmware: "knulli"})
	if err != nil {
		t.Fatal(err)
	}
	f, err := firmware.Resolve(d, host.OS(envLookup(env)), firmware.Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if f.Version != "scarab 2026/05/11 22:15" {
		t.Errorf("version = %q", f.Version)
	}
	if f.Card != "/userdata" || f.Roms != "/userdata/roms" || f.Saves != "/userdata/saves" || f.Bios != "/userdata/bios" {
		t.Errorf("paths = %q %q %q %q", f.Card, f.Roms, f.Saves, f.Bios)
	}
	locs, _ := f.SaveLocations("psp", "/userdata/roms/psp/Game.iso")
	if len(locs) == 0 || locs[0].Dir != "/userdata/saves/psp/SAVEDATA" {
		t.Errorf("SaveLocations = %+v, want psp/SAVEDATA first", locs)
	}
	if got := f.SaveName("Game.gba", "/userdata/saves/gba"); got != "Game.srm" {
		t.Errorf("SaveName = %q, want Game.srm", got)
	}
	if dir, ok := f.ArtDir(firmware.ArtCover, "gba", "/userdata/roms/gba", "Game Boy Advance"); !ok || dir != "/userdata/roms/gba/images" {
		t.Errorf("cover dir = %q %v", dir, ok)
	}
}

func TestROCKNIXMockRoot(t *testing.T) {
	d, err := firmware.Load("rocknix")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "root")
	env, err := Build(dir, Options{Firmware: "rocknix", Device: "rgds"})
	if err != nil {
		t.Fatal(err)
	}
	f, err := firmware.Resolve(d, host.OS(envLookup(env)), firmware.Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if f.Version != "RK3566.aarch64-20261101" {
		t.Errorf("version = %q", f.Version)
	}
	if f.Card != "/storage" || f.Roms != "/storage/roms" || f.Saves != "/storage/roms" || f.Bios != "/storage/roms/bios" {
		t.Errorf("paths = %q %q %q %q", f.Card, f.Roms, f.Saves, f.Bios)
	}
	locs, _ := f.SaveLocations("gba", "/storage/roms/gba/Game.gba")
	if len(locs) != 1 || locs[0].Dir != "/storage/roms/gba" {
		t.Errorf("SaveLocations = %+v, want the ROM folder", locs)
	}
	if got := f.SaveName("Game.gba", "/storage/roms/gba"); got != "Game.srm" {
		t.Errorf("SaveName = %q, want Game.srm", got)
	}
	if dir, ok := f.ArtDir(firmware.ArtCover, "gba", "/storage/roms/gba", "Game Boy Advance"); !ok || dir != "/storage/roms/gba/images" {
		t.Errorf("cover dir = %q %v", dir, ok)
	}
	mapping, err := f.InputMapping()
	if err != nil || len(mapping) == 0 {
		t.Errorf("input mapping = %d bytes, err %v", len(mapping), err)
	}
}

func TestSpruceMockRoot(t *testing.T) {
	d, err := firmware.Load("spruce")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "root")
	env, err := Build(dir, Options{Firmware: "spruce", Device: "a30"})
	if err != nil {
		t.Fatal(err)
	}
	f, err := firmware.Resolve(d, host.OS(envLookup(env)), firmware.Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if f.Version != "4.6.0" {
		t.Errorf("version = %q", f.Version)
	}
	if f.Card != "/mnt/SDCARD" || f.Roms != "/mnt/SDCARD/Roms" || f.Saves != "/mnt/SDCARD/Saves/saves" || f.Bios != "/mnt/SDCARD/BIOS" {
		t.Errorf("paths = %q %q %q %q", f.Card, f.Roms, f.Saves, f.Bios)
	}
	locs, _ := f.SaveLocations("psp", "/mnt/SDCARD/Roms/PSP/Game.iso")
	if len(locs) == 0 || locs[0].Dir != "/mnt/SDCARD/Saves/.config/ppsspp/PSP/SAVEDATA" {
		t.Errorf("SaveLocations = %+v, want the PPSSPP SAVEDATA folder first", locs)
	}
	arch := "64"
	if runtime.GOARCH == "arm" {
		arch = "32"
	}
	locs, _ = f.SaveLocations("psx", "/mnt/SDCARD/Roms/PS/Game.chd")
	if len(locs) == 0 || locs[0].Dir != "/mnt/SDCARD/Saves/saves/PCSX-ReARMed-"+arch || !locs[0].Preferred {
		t.Errorf("SaveLocations = %+v, want PCSX-ReARMed-%s first", locs, arch)
	}
	if got := f.SaveName("Game.gba", "/mnt/SDCARD/Saves/saves/mGBA"); got != "Game.srm" {
		t.Errorf("SaveName = %q, want Game.srm", got)
	}
	if dir, ok := f.ArtDir(firmware.ArtCover, "gba", "/mnt/SDCARD/Roms/GBA", "Game Boy Advance"); !ok || dir != "/mnt/SDCARD/Roms/GBA/Imgs" {
		t.Errorf("cover dir = %q %v", dir, ok)
	}
	if q := f.Quirks(); q.Rotation != 270 {
		t.Errorf("quirks = %+v, want rotation 270", q)
	}
	mapping, err := f.InputMapping()
	if err != nil || len(mapping) == 0 {
		t.Errorf("input mapping = %d bytes, err %v", len(mapping), err)
	}
}

func TestAlliumMockRoot(t *testing.T) {
	d, err := firmware.Load("allium")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "root")
	env, err := Build(dir, Options{Firmware: "allium"})
	if err != nil {
		t.Fatal(err)
	}
	f, err := firmware.Resolve(d, host.OS(envLookup(env)), firmware.Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if f.Version != "v1.0.2" {
		t.Errorf("version = %q", f.Version)
	}
	if f.Card != "/mnt/SDCARD" || f.Roms != "/mnt/SDCARD/Roms" || f.Saves != "/mnt/SDCARD/Saves/CurrentProfile/saves" || f.Bios != "/mnt/SDCARD/BIOS" {
		t.Errorf("paths = %q %q %q %q", f.Card, f.Roms, f.Saves, f.Bios)
	}
	locs, _ := f.SaveLocations("gba", "/mnt/SDCARD/Roms/GBA/Game.gba")
	if len(locs) == 0 || locs[0].Dir != "/mnt/SDCARD/Saves/CurrentProfile/saves/gpSP" {
		t.Errorf("SaveLocations = %+v, want gpSP first", locs)
	}
	if got := f.SaveName("Game.gba", "/mnt/SDCARD/Saves/CurrentProfile/saves/gpSP"); got != "Game.srm" {
		t.Errorf("SaveName = %q, want Game.srm", got)
	}
	if dir, ok := f.ArtDir(firmware.ArtCover, "gba", "/mnt/SDCARD/Roms/GBA", "Game Boy Advance"); !ok || dir != "/mnt/SDCARD/Roms/GBA/Imgs" {
		t.Errorf("cover dir = %q %v", dir, ok)
	}
	mapping, err := f.InputMapping()
	if err != nil || len(mapping) == 0 {
		t.Errorf("input mapping = %d bytes, err %v", len(mapping), err)
	}
}

func TestOnionMockRoot(t *testing.T) {
	d, err := firmware.Load("onion")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "root")
	env, err := Build(dir, Options{Firmware: "onion"})
	if err != nil {
		t.Fatal(err)
	}
	f, err := firmware.Resolve(d, host.OS(envLookup(env)), firmware.Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if f.Version != "v4.3.1-1" {
		t.Errorf("version = %q", f.Version)
	}
	if f.Card != "/mnt/SDCARD" || f.Roms != "/mnt/SDCARD/Roms" || f.Saves != "/mnt/SDCARD/Saves/CurrentProfile/saves" || f.Bios != "/mnt/SDCARD/BIOS" {
		t.Errorf("paths = %q %q %q %q", f.Card, f.Roms, f.Saves, f.Bios)
	}
	locs, _ := f.SaveLocations("gba", "/mnt/SDCARD/Roms/GBA/Game.gba")
	if len(locs) == 0 || locs[0].Dir != "/mnt/SDCARD/Saves/CurrentProfile/saves/mGBA" {
		t.Errorf("SaveLocations = %+v, want mGBA first", locs)
	}
	if got := f.SaveName("Game.gba", "/mnt/SDCARD/Saves/CurrentProfile/saves/mGBA"); got != "Game.srm" {
		t.Errorf("SaveName = %q, want Game.srm", got)
	}
	if dirs := f.RomDirs("psx"); len(dirs) == 0 || dirs[0] != "/mnt/SDCARD/Roms/PS" {
		t.Errorf("psx RomDirs = %v, want Roms/PS first", dirs)
	}
	if dir, ok := f.ArtDir(firmware.ArtCover, "gba", "/mnt/SDCARD/Roms/GBA", "Game Boy Advance"); !ok || dir != "/mnt/SDCARD/Roms/GBA/Imgs" {
		t.Errorf("cover dir = %q %v", dir, ok)
	}
	mapping, err := f.InputMapping()
	if err != nil || len(mapping) == 0 {
		t.Errorf("input mapping = %d bytes, err %v", len(mapping), err)
	}
}

func TestLeafMockRoot(t *testing.T) {
	d, err := firmware.Load("leaf")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "root")
	env, err := Build(dir, Options{Firmware: "leaf"})
	if err != nil {
		t.Fatal(err)
	}
	f, err := firmware.Resolve(d, host.OS(envLookup(env)), firmware.Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if f.Card != "/mnt/sdcard" || f.Roms != "/mnt/sdcard/Roms" || f.Saves != "/mnt/sdcard/Saves" || f.Bios != "/mnt/sdcard/BIOS" {
		t.Errorf("paths = %q %q %q %q", f.Card, f.Roms, f.Saves, f.Bios)
	}
	locs, _ := f.SaveLocations("gba", "/mnt/sdcard/Roms/GBA/Game.gba")
	if len(locs) == 0 || locs[0].Dir != "/mnt/sdcard/Saves/mGBA" {
		t.Errorf("SaveLocations = %+v, want mGBA first", locs)
	}
	if got := f.SaveName("Game.gba", "/mnt/sdcard/Saves/mGBA"); got != "Game.srm" {
		t.Errorf("SaveName = %q, want Game.srm", got)
	}
	if dirs := f.RomDirs("nes"); len(dirs) == 0 || dirs[0] != "/mnt/sdcard/Roms/NES" {
		t.Errorf("nes RomDirs = %v, want Roms/NES first", dirs)
	}
	if dir, ok := f.ArtDir(firmware.ArtCover, "nes", "/mnt/sdcard/Roms/FC", "Nintendo Entertainment System"); !ok || dir != "/mnt/sdcard/Images/FC" {
		t.Errorf("cover dir = %q %v", dir, ok)
	}
	if got := f.BiosPaths("amiga", "kick34005.A500"); len(got) != 1 || got[0] != "/mnt/sdcard/BIOS/puae/kick34005.A500" {
		t.Errorf("amiga BiosPaths = %v", got)
	}
	if got := f.BiosPaths("neogeoaes", "fbneo/neogeo.zip"); len(got) != 2 || got[0] != "/mnt/sdcard/BIOS/neogeo.zip" || got[1] != "/mnt/sdcard/BIOS/fbneo/neogeo.zip" {
		t.Errorf("neogeoaes BiosPaths = %v", got)
	}
	mapping, err := f.InputMapping()
	if err != nil || len(mapping) == 0 {
		t.Errorf("input mapping = %d bytes, err %v", len(mapping), err)
	}
}
