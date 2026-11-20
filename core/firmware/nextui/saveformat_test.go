package nextui

import (
	"os"
	"testing"
	"testing/fstest"

	"github.com/rommapp/grout/core/firmware/host"
)

const settingsKey = "mnt/SDCARD/.userdata/shared/minuisettings.txt"

func fakeWith(settings string, present bool) *host.Fake {
	files := fstest.MapFS{}
	if present {
		files[settingsKey] = &fstest.MapFile{Data: []byte(settings)}
	}
	return &host.Fake{Files: files}
}

func TestSaveFormat(t *testing.T) {
	vars := map[string]string{"card": "/mnt/SDCARD"}
	for _, tc := range []struct {
		name     string
		settings string
		present  bool
		keep     bool
		ext      string
	}{
		{"no settings file is NextUI's default", "", false, true, "sav"},
		{"MinUI", "fontSize=2\nsaveFormat=0\n", true, true, "sav"},
		{"RetroArch", "saveFormat=1\n", true, false, "srm"},
		{"Generic", "saveFormat=2\n", true, false, "sav"},
		{"RetroArch uncompressed", "saveFormat=3\n", true, false, "srm"},
		{"setting missing from the file", "fontSize=2\n", true, true, "sav"},
		{"unknown value falls back like minarch", "saveFormat=9\n", true, true, "sav"},
		{"last line wins like CFG_init", "saveFormat=1\nsaveFormat=2\n", true, false, "sav"},
		{"windows line endings", "saveFormat=1\r\n", true, false, "srm"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			keep, ext, ok := SaveFormat(fakeWith(tc.settings, tc.present), vars)
			if !ok || keep != tc.keep || ext != tc.ext {
				t.Errorf("keep=%v ext=%q ok=%v, want keep=%v ext=%q ok=true", keep, ext, ok, tc.keep, tc.ext)
			}
		})
	}
}

func TestSaveFormatReadsUnderCard(t *testing.T) {
	h := &host.Fake{Files: fstest.MapFS{"media/card/.userdata/shared/minuisettings.txt": &fstest.MapFile{Data: []byte("saveFormat=1\n")}}}
	keep, ext, ok := SaveFormat(h, map[string]string{"card": "/media/card"})
	if !ok || keep || ext != "srm" {
		t.Errorf("keep=%v ext=%q ok=%v, want keep=false ext=\"srm\" ok=true", keep, ext, ok)
	}
}

func TestSaveFormatMockFixture(t *testing.T) {
	data, err := os.ReadFile("mock/" + settingsKey)
	if err != nil {
		t.Fatal(err)
	}
	keep, ext, ok := SaveFormat(fakeWith(string(data), true), map[string]string{"card": "/mnt/SDCARD"})
	if !ok || !keep || ext != "sav" {
		t.Errorf("keep=%v ext=%q ok=%v, want keep=true ext=\"sav\" ok=true", keep, ext, ok)
	}
}
