package spruce

import (
	"runtime"
	"slices"
	"testing"

	"github.com/rommapp/grout/core/firmware/host"
)

func TestArchFolders(t *testing.T) {
	for _, tc := range []struct {
		romFolder string
		arch      string
		want      []string
	}{
		{"PS", "32", []string{"PCSX-ReARMed-32"}},
		{"PS", "64", []string{"PCSX-ReARMed-64"}},
		{"FAKE08", "32", []string{"fake-08-32"}},
		{"FAKE08", "64", []string{"fake-08-64"}},
		{"SFC", "64", nil},
		{"MSU1", "64", nil},
		{"PICO8", "64", nil},
		{"NGP", "64", nil},
		{"NGPC", "32", nil},
		{"GBA", "32", nil},
	} {
		if got := archFolders(tc.romFolder, tc.arch); !slices.Equal(got, tc.want) {
			t.Errorf("archFolders(%q, %q) = %q, want %q", tc.romFolder, tc.arch, got, tc.want)
		}
	}
}

func TestArchSaves(t *testing.T) {
	want := "PCSX-ReARMed-64"
	if runtime.GOARCH == "arm" {
		want = "PCSX-ReARMed-32"
	}
	folders, preferred := ArchSaves(&host.Fake{}, "PS")
	if !preferred || !slices.Equal(folders, []string{want}) {
		t.Errorf("ArchSaves(PS) = %q %v, want [%s] true", folders, preferred, want)
	}
	if folders, preferred := ArchSaves(&host.Fake{}, "GBA"); folders != nil || preferred {
		t.Errorf("ArchSaves(GBA) = %q %v, want nil false", folders, preferred)
	}
}
