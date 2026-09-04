package cfw

import (
	"strings"
)

// GetPlatformMap returns the rom folder names the given CFW accepts, keyed by
// RomM filesystem slug.
func GetPlatformMap(c CFW) map[string][]string {
	return Lookup(c).Platforms()
}

// FSSlugToFolder converts a RomM filesystem slug to firmware c's folder name.
//
// A platform the firmware maps to nothing returns "", which means it has no
// folder there rather than that the slug is unknown.
func FSSlugToFolder(c CFW, fsSlug string) string {
	platformMap := GetPlatformMap(c)
	if platformMap == nil {
		return strings.ToLower(fsSlug)
	}

	if folders, ok := platformMap[fsSlug]; ok {
		if len(folders) > 0 {
			return folders[0]
		}
		return ""
	}

	return strings.ToLower(fsSlug)
}

// RomMFSSlugToCFW converts a slug for the firmware grout is running on.
func RomMFSSlugToCFW(fsSlug string) string {
	return FSSlugToFolder(GetCFW(), fsSlug)
}
