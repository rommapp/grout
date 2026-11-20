package mockroot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rommapp/grout/core/firmware"
	"github.com/rommapp/grout/core/firmware/host"
)

func envLookup(env map[string]string) func(string) string {
	return func(k string) string { return env[k] }
}

func exists(t *testing.T, p string) {
	t.Helper()
	if _, err := os.Stat(p); err != nil {
		t.Errorf("missing %s: %v", p, err)
	}
}

func TestBuildResolvesToRequestedDevice(t *testing.T) {
	d, err := firmware.Load("testfw")
	if err != nil {
		t.Fatal(err)
	}
	for _, device := range []string{"envdev", "filedev", "generic"} {
		t.Run(device, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "root")
			env, err := Build(dir, Options{Firmware: "testfw", Device: device})
			if err != nil {
				t.Fatal(err)
			}
			f, err := firmware.Resolve(d, host.OS(envLookup(env)), firmware.Overrides{})
			if err != nil {
				t.Fatal(err)
			}
			if f.Device != device {
				t.Errorf("device = %q, want %q", f.Device, device)
			}
			exists(t, filepath.Join(dir, "etc", "testfw-release"))
			if env["CFW"] != "TESTFW" || env["TESTFW_MODE"] != "mock" {
				t.Errorf("env = %v", env)
			}
			if abs, _ := filepath.Abs(dir); env["GROUT_ROOT"] != abs {
				t.Errorf("GROUT_ROOT = %q, want %q", env["GROUT_ROOT"], abs)
			}
		})
	}
}

func TestBuildCreatesPlatformFolders(t *testing.T) {
	d, err := firmware.Load("testfw")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "root")
	env, err := Build(dir, Options{Firmware: "testfw"})
	if err != nil {
		t.Fatal(err)
	}
	f, err := firmware.Resolve(d, host.OS(envLookup(env)), firmware.Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	exists(t, filepath.Join(dir, "mnt", "SDCARD"))
	exists(t, filepath.Join(dir, filepath.FromSlash(f.Bios)))
	exists(t, filepath.Join(dir, filepath.FromSlash(f.Roms)))
	exists(t, filepath.Join(dir, filepath.FromSlash(f.Saves)))
	for slug := range f.Rows() {
		romDir := f.RomDirs(slug)[0]
		exists(t, filepath.Join(dir, filepath.FromSlash(romDir)))
		locs, _ := f.SaveLocations(slug, romDir+"/x")
		for _, l := range locs {
			exists(t, filepath.Join(dir, filepath.FromSlash(l.Dir)))
		}
	}

	only := filepath.Join(t.TempDir(), "root")
	if _, err := Build(only, Options{Firmware: "testfw", Platforms: []string{"gba"}}); err != nil {
		t.Fatal(err)
	}
	exists(t, filepath.Join(only, "mnt", "SDCARD", "Roms", "GBA"))
	if _, err := os.Stat(filepath.Join(only, "mnt", "SDCARD", "Roms", "PSX")); err == nil {
		t.Error("psx folder exists with Platforms gba")
	}
	if _, err := os.Stat(filepath.Join(only, "mnt", "SDCARD", "Saves", "PSX")); err == nil {
		t.Error("psx save folder exists with Platforms gba")
	}
}

func TestBuildRefusesNonEmptyDir(t *testing.T) {
	dir := t.TempDir()
	keep := filepath.Join(dir, "keep.txt")
	if err := os.WriteFile(keep, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(dir, Options{Firmware: "testfw"}); err == nil {
		t.Fatal("expected error")
	}
	data, err := os.ReadFile(keep)
	if err != nil || string(data) != "x" {
		t.Errorf("file touched: %q %v", data, err)
	}
	if _, err := Build(dir, Options{Firmware: "testfw", Force: true}); err != nil {
		t.Fatal(err)
	}
	exists(t, keep)
}

func TestBuildUnknownDevice(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "root")
	_, err := Build(dir, Options{Firmware: "testfw", Device: "nope"})
	if err == nil {
		t.Fatal("expected error")
	}
	if _, statErr := os.Stat(dir); statErr == nil {
		t.Error("target dir was created")
	}
	for _, want := range []string{"envdev", "filedev", "generic"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
}

func TestForceRebuildSwitchesDevice(t *testing.T) {
	d, err := firmware.Load("muos")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "root")
	if _, err := Build(dir, Options{Firmware: "muos", Device: "trimui-smart-pro"}); err != nil {
		t.Fatal(err)
	}
	env, err := Build(dir, Options{Firmware: "muos", Force: true})
	if err != nil {
		t.Fatal(err)
	}
	f, err := firmware.Resolve(d, host.OS(envLookup(env)), firmware.Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if f.Device != "anbernic" {
		t.Fatalf("device = %q, want anbernic", f.Device)
	}
}

func TestCopyTreeWithoutMockDir(t *testing.T) {
	m, err := firmware.MockFS("nomock")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := copyTree(m, dir); err != nil {
		t.Fatal(err)
	}
}
