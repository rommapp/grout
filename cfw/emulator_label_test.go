package cfw

import "testing"

// muOS keeps saves under paths like "file/PPSSPP/backup". The parts around the
// emulator's name are storage layout, not something the user picks between, so
// showing them would make two folders look different when only the emulator
// matters.
func TestEmulatorLabel_MuOS(t *testing.T) {
	tests := map[string]string{
		"file/PPSSPP/backup": "PPSSPP",
		"file/FinalBurn Neo": "FinalBurn Neo",
		"PPSSPP":             "PPSSPP",
		"file/mgba/backup":   "mgba",
	}

	for dir, want := range tests {
		if got := EmulatorLabel(MuOS, dir); got != want {
			t.Errorf("EmulatorLabel(%q) = %q, want %q", dir, got, want)
		}
	}
}

// Every other firmware keeps the emulator's name as the last part of the path,
// so the label is just that.
func TestEmulatorLabel_OtherFirmwares(t *testing.T) {
	for _, c := range []CFW{Knulli, MinUI, ArkOS, ROCKNIX} {
		if got := EmulatorLabel(c, "saves/mgba"); got != "mgba" {
			t.Errorf("%s: EmulatorLabel = %q, want mgba", c, got)
		}
	}
}

// Two folders that differ only in the parts muOS strips would read the same,
// which would be a picker offering the same answer twice. The real tables must
// not do that.
func TestEmulatorLabel_MuOSLabelsStayDistinct(t *testing.T) {
	for slug, dirs := range EmulatorFolderMap(MuOS) {
		if len(dirs) < 2 {
			continue
		}

		seen := make(map[string]string, len(dirs))
		for _, dir := range dirs {
			label := EmulatorLabel(MuOS, dir)
			if other, clash := seen[label]; clash {
				t.Errorf("%s: %q and %q both read as %q", slug, other, dir, label)
			}
			seen[label] = dir
		}
	}
}
