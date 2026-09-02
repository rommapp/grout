package cfw

import (
	"strings"
)

// GetPlatformMap returns the rom folder names the given CFW accepts, keyed by
// RomM filesystem slug.
func GetPlatformMap(c CFW) map[string][]string {
	return Lookup(c).Platforms()
}

// RomMFSSlugToCFW converts a RomM filesystem slug to the CFW-specific folder name.
func RomMFSSlugToCFW(fsSlug string) string {
	cfwPlatformMap := GetPlatformMap(GetCFW())
	if cfwPlatformMap == nil {
		return strings.ToLower(fsSlug)
	}

	if value, ok := cfwPlatformMap[fsSlug]; ok {
		if len(value) > 0 {
			return value[0]
		}
		return ""
	}

	return strings.ToLower(fsSlug)
}
