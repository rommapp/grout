package utils

import (
	"grout/cfw"
	"grout/romm"
)

// GetPlatformRomDirectory returns the ROM directory for a platform using config.
func GetPlatformRomDirectory(config Config, platform romm.Platform) string {
	rp := config.DirectoryMappings[platform.Slug].RelativePath
	return cfw.GetPlatformRomDirectory(rp, platform.Slug)
}

// GetArtDirectory returns the artwork directory for a platform using config.
func GetArtDirectory(config Config, platform romm.Platform) string {
	romDir := GetPlatformRomDirectory(config, platform)
	return cfw.GetArtDirectory(romDir, platform.Slug, platform.Name)
}

// RomFolderBase returns the base folder name for ROM matching.
func RomFolderBase(path string) string {
	return cfw.RomFolderBase(path, ParseTag)
}
