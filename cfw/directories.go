package cfw

import "path/filepath"

// GetRomDirectory returns the ROM directory for the current CFW.
func GetRomDirectory() string {
	return ActiveFirmware().RomDirectory()
}

// RomFolderBase returns the base folder name for ROM matching. tagParser
// extracts tags from paths, for firmwares that allow them.
func RomFolderBase(path string, tagParser func(string) string) string {
	return ActiveFirmware().RomFolderBase(path, tagParser)
}

// GetBIOSDirectory returns the BIOS directory for the current CFW.
func GetBIOSDirectory() string {
	return ActiveFirmware().BIOSDirectory()
}

// GetBIOSFilePaths returns the BIOS file paths for a given relative path and platform.
func GetBIOSFilePaths(relativePath string, platformFSSlug string) []string {
	return ActiveFirmware().BIOSFilePaths(relativePath, platformFSSlug)
}

// GetPlatformRomDirectory returns the ROM directory for a platform.
// relativePath is the configured relative path from directory mappings.
// platformFSSlug is used as fallback if relativePath is empty.
func GetPlatformRomDirectory(relativePath, platformFSSlug string) string {
	rp := relativePath
	if rp == "" {
		rp = RomMFSSlugToCFW(platformFSSlug)
	}
	return filepath.Join(GetRomDirectory(), rp)
}

// ArtDirectory returns where firmware c keeps a kind of artwork, or "" if it
// keeps none. Callers treat "" as "skip", not as an error.
func ArtDirectory(c CFW, slot ArtSlot, romDir, platformFSSlug, platformName string) string {
	return Lookup(c).ArtDirectory(slot, romDir, platformFSSlug, platformName)
}

// BaseSavePath returns the base save path for the current CFW.
func BaseSavePath() string {
	return ActiveFirmware().BaseSavePath()
}

// joinPath returns "" for an empty base, so a missing directory does not
// become a relative path.
func joinPath(base, rel string) string {
	if base == "" {
		return ""
	}
	return filepath.Join(base, rel)
}
