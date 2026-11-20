package muos

import (
	"testing"

	"github.com/rommapp/grout/core/firmware/host"
)

func romRoot(t *testing.T, h *host.Fake, storage string) string {
	t.Helper()
	got, err := RomRoot(h, map[string]string{"card": "/run/muos/storage", "muos_rom_storage": storage})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestRomRootFollowsTheUnionOrder(t *testing.T) {
	for _, tc := range []struct {
		name     string
		withRoms []string
		want     string
	}{
		{"only SD1", []string{"/mnt/mmc"}, "/mnt/mmc/ROMS"},
		{"SD2 before SD1", []string{"/mnt/mmc", "/mnt/sdcard"}, "/mnt/sdcard/ROMS"},
		{"USB before both", []string{"/mnt/mmc", "/mnt/sdcard", "/mnt/usb"}, "/mnt/usb/ROMS"},
		{"nothing anywhere falls back to SD1", nil, "/mnt/mmc/ROMS"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := mockHost(t)
			for _, m := range tc.withRoms {
				mkdir(h, m+"/ROMS")
			}
			if got := romRoot(t, h, ""); got != tc.want {
				t.Errorf("rom root = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestRomRootUsesTheUnionWhenItExists(t *testing.T) {
	h := mockHost(t)
	mkdir(h, "/mnt/mmc/ROMS")
	mkdir(h, "/mnt/sdcard/ROMS")
	mkdir(h, "/mnt/union/ROMS")
	if got := romRoot(t, h, "sdcard"); got != "/mnt/union/ROMS" {
		t.Errorf("rom root = %s, want the union", got)
	}
}

func TestRomRootHonoursTheChosenStorage(t *testing.T) {
	h := mockHost(t)
	mkdir(h, "/mnt/mmc/ROMS")
	mkdir(h, "/mnt/sdcard/ROMS")
	if got := romRoot(t, h, "rom"); got != "/mnt/mmc/ROMS" {
		t.Errorf("SD1 chosen: %s", got)
	}
	if got := romRoot(t, h, "sdcard"); got != "/mnt/sdcard/ROMS" {
		t.Errorf("SD2 chosen: %s", got)
	}
	if got := romRoot(t, h, "usb"); got != "/mnt/sdcard/ROMS" {
		t.Errorf("USB chosen but has no roms: %s, want the automatic SD2", got)
	}
}

func TestRomRootSD2WithoutUnion(t *testing.T) {
	h := mockHost(t)
	mkdir(h, "/mnt/sdcard/ROMS")
	if got := romRoot(t, h, ""); got != "/mnt/sdcard/ROMS" {
		t.Errorf("rom root = %s, want the SD2 mount", got)
	}
	if got := romRoot(t, h, "rom"); got != "/mnt/sdcard/ROMS" {
		t.Errorf("SD1 chosen without roms: %s, want SD2", got)
	}
}

func TestRomRootReadsMountPointsFromMuOS(t *testing.T) {
	h := mockHost(t)
	put(h, "/opt/muos/device/config/storage/sdcard/mount", "/media/card2\n")
	mkdir(h, "/media/card2/ROMS")
	if got := romRoot(t, h, ""); got != "/media/card2/ROMS" {
		t.Errorf("rom root = %s, want the configured SD2 mount", got)
	}
}

func TestRomRootDefaultMountsWithoutConfig(t *testing.T) {
	h := emptyHost()
	mkdir(h, "/mnt/usb/ROMS")
	if got := romRoot(t, h, ""); got != "/mnt/usb/ROMS" {
		t.Errorf("rom root = %s, want the default USB mount", got)
	}
	remove(h, "/mnt/usb/ROMS")
	put(h, "/opt/muos/device/config/storage/rom/mount", "  \n")
	if got := romRoot(t, h, ""); got != "/mnt/mmc/ROMS" {
		t.Errorf("rom root = %s, want the default SD1 mount for a blank file", got)
	}
}

func TestRomRootMockFixture(t *testing.T) {
	h := mockHost(t)
	for storage, want := range map[string]string{"rom": "/mnt/mmc", "sdcard": "/mnt/sdcard", "usb": "/mnt/usb"} {
		if got := mountPoint(h, storage); got != want {
			t.Errorf("mount of %s = %s, want %s", storage, got, want)
		}
	}
}
