package cfw

import (
	"grout/cfw/allium"
	"grout/cfw/knulli"
	"grout/cfw/muos"
	"grout/cfw/nextui"
	"grout/cfw/rocknix"
	"grout/cfw/spruce"
	"grout/cfw/trimui"
	"path/filepath"
)

// GetRomDirectory returns the ROM directory for the current CFW.
func GetRomDirectory() string {
	switch GetCFW() {
	case MuOS:
		return muos.GetRomDirectory()
	case NextUI:
		return nextui.GetRomDirectory()
	case Knulli:
		return knulli.GetRomDirectory()
	case Spruce:
		return spruce.GetRomDirectory()
	case ROCKNIX:
		return rocknix.GetRomDirectory()
	case Trimui:
		return trimui.GetRomDirectory()
	case Allium:
		return allium.GetRomDirectory()
	}
	return ""
}

// RomFolderBase returns the base folder name for ROM matching.
// tagParser is a function that extracts tags from paths (for NextUI).
func RomFolderBase(path string, tagParser func(string) string) string {
	if GetCFW() == NextUI {
		return nextui.RomFolderBase(path, tagParser)
	}
	return path
}

// GetBIOSDirectory returns the BIOS directory for the current CFW.
func GetBIOSDirectory() string {
	switch GetCFW() {
	case MuOS:
		return muos.GetBIOSDirectory()
	case NextUI:
		return nextui.GetBIOSDirectory()
	case Knulli:
		return knulli.GetBIOSDirectory()
	case Spruce:
		return spruce.GetBIOSDirectory()
	case ROCKNIX:
		return rocknix.GetBIOSDirectory()
	case Trimui:
		return trimui.GetBIOSDirectory()
	case Allium:
		return allium.GetBIOSDirectory()
	}
	return ""
}

// GetBIOSFilePaths returns the BIOS file paths for a given relative path and platform.
func GetBIOSFilePaths(relativePath string, platformFSSlug string) []string {
	if GetCFW() == NextUI {
		return nextui.GetBIOSFilePaths(relativePath, platformFSSlug)
	}
	return []string{filepath.Join(GetBIOSDirectory(), relativePath)}
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

// GetArtDirectory returns the artwork directory for a platform.
func GetArtDirectory(romDir string, platformFSSlug, platformName string) string {
	switch GetCFW() {
	case NextUI:
		return nextui.GetArtDirectory(romDir)
	case Knulli:
		return knulli.GetArtDirectory(romDir)
	case Spruce:
		return spruce.GetArtDirectory(romDir)
	case MuOS:
		return muos.GetArtDirectory(platformFSSlug, platformName)
	case ROCKNIX:
		return rocknix.GetArtDirectory(romDir)
	case Trimui:
		return trimui.GetArtDirectory(platformFSSlug, platformName)
	case Allium:
		return allium.GetArtDirectory(romDir)
	default:
		return ""
	}
}

// BaseSavePath returns the base save path for the current CFW.
func BaseSavePath() string {
	switch GetCFW() {
	case MuOS:
		return muos.GetBaseSavePath()
	case NextUI:
		return nextui.GetBaseSavePath()
	case Knulli:
		return knulli.GetBaseSavePath()
	case Spruce:
		return spruce.GetBaseSavePath()
	case ROCKNIX:
		return rocknix.GetBaseSavePath()
	case Trimui:
		return trimui.GetBaseSavePath()
	case Allium:
		return allium.GetBaseSavePath()
	}
	return ""
}
