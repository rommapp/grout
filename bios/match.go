package bios

import (
	"path/filepath"
	"strings"

	"grout/romm"
)

// Requirement pairs a firmware file the server holds with what grout knows
// about where it belongs on the device.
type Requirement struct {
	// Firmware is the entry as RomM holds it.
	Firmware romm.Firmware
	// File says where the file goes and whether an emulator can run without
	// it.
	File File
	// Known is false when nothing in the tables matched. The file is still
	// downloaded, under the name the server gave it, but Optional is then a
	// default rather than a claim.
	Known bool
}

// Installed reports whether this file is already on the device.
func (r Requirement) Installed(platformFSSlug string) bool {
	return FileExists(r.File, platformFSSlug)
}

// Save writes a downloaded file everywhere the firmware expects to find it.
func (r Requirement) Save(platformFSSlug string, data []byte) error {
	return SaveFile(r.File, platformFSSlug, data)
}

// Match pairs each firmware entry the server holds with grout's own tables.
//
// RomM names an uploaded file however whoever uploaded it did, so the match
// ignores case and tries the path it was stored under as well as the bare
// name. Anything unmatched still comes back: downloading it under the server's
// own name is the best guess available, and is what the emulator will most
// often be looking for anyway.
func Match(platformFSSlug string, firmware []romm.Firmware) []Requirement {
	return matchAgainst(GetFilesForPlatform(platformFSSlug), firmware)
}

func matchAgainst(knownFiles []File, firmware []romm.Firmware) []Requirement {
	byName := make(map[string]File)
	byPath := make(map[string]File)
	for _, known := range knownFiles {
		byName[strings.ToLower(known.FileName)] = known
		byPath[strings.ToLower(known.RelativePath)] = known
		byName[strings.ToLower(filepath.Base(known.RelativePath))] = known
	}

	requirements := make([]Requirement, 0, len(firmware))
	for _, entry := range firmware {
		requirement := Requirement{
			Firmware: entry,
			// Falling back to the server's name means every later step has one
			// path to work with instead of two branches.
			File: File{FileName: entry.FileName, RelativePath: entry.FileName},
		}

		for _, candidate := range []string{
			strings.ToLower(entry.FileName),
			strings.ToLower(entry.FilePath),
			strings.ToLower(filepath.Base(entry.FilePath)),
		} {
			if known, ok := byName[candidate]; ok {
				requirement.File, requirement.Known = known, true
				break
			}
			if known, ok := byPath[candidate]; ok {
				requirement.File, requirement.Known = known, true
				break
			}
		}

		requirements = append(requirements, requirement)
	}

	return requirements
}
