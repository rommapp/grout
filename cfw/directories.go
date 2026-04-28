package cfw

import (
	"grout/cfw/allium"
	"grout/cfw/batocera"
	"grout/cfw/knulli"
	"grout/cfw/minui"
	"grout/cfw/muos"
	"grout/cfw/nextui"
	"grout/cfw/onion"
	"grout/cfw/retrodeck"
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
	case Onion:
		return onion.GetRomDirectory()
	case Batocera:
		return batocera.GetRomDirectory()
	case MinUI:
		return minui.GetRomDirectory()
	case RetroDECK:
		return retrodeck.GetRomDirectory()
	}
	return ""
}

// RomFolderBase returns the base folder name for ROM matching.
// tagParser is a function that extracts tags from paths (for NextUI).
func RomFolderBase(path string, tagParser func(string) string) string {
	switch GetCFW() {
	case NextUI:
		return nextui.RomFolderBase(path, tagParser)
	case MinUI:
		return minui.RomFolderBase(path, tagParser)
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
	case Onion:
		return onion.GetBIOSDirectory()
	case Batocera:
		return batocera.GetBIOSDirectory()
	case MinUI:
		return minui.GetBIOSDirectory()
	case RetroDECK:
		return retrodeck.GetBIOSDirectory()
	}
	return ""
}

// GetBIOSFilePaths returns the BIOS file paths for a given relative path and platform.
func GetBIOSFilePaths(relativePath string, platformFSSlug string) []string {
	switch GetCFW() {
	case NextUI:
		return nextui.GetBIOSFilePaths(relativePath, platformFSSlug)
	case MinUI:
		return minui.GetBIOSFilePaths(relativePath, platformFSSlug)
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
	case Onion:
		return onion.GetArtDirectory(romDir)
	case Batocera:
		return batocera.GetArtDirectory(romDir)
	case MinUI:
		return minui.GetArtDirectory(romDir)
	case RetroDECK:
		return retrodeck.GetArtDirectory(romDir)
	default:
		return ""
	}
}

func GetArtPreviewDirectory(romDir string, platformFSSlug, platformName string) string {
	switch GetCFW() {
	case MuOS:
		return muos.GetPreviewDirectory(platformFSSlug, platformName)
	default:
		return ""
	}
}

func GetArtSplashDirectory(romDir string, platformFSSlug, platformName string) string {
	switch GetCFW() {
	case MuOS:
		return muos.GetSplashDirectory(platformFSSlug, platformName)
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
	case Onion:
		return onion.GetBaseSavePath()
	case Batocera:
		return batocera.GetBaseSavePath()
	case MinUI:
		return minui.GetBaseSavePath()
	case RetroDECK:
		return retrodeck.GetBaseSavePath()
	}
	return ""
}

func GetArtMarqueeDirectory(romDir string, platformFSSlug, platformName string) string {
	switch GetCFW() {
	case ROCKNIX:
		return rocknix.GetArtDirectory(romDir)
	case Knulli:
		return knulli.GetArtDirectory(romDir)
	case Batocera:
		return batocera.GetArtDirectory(romDir)
	case RetroDECK:
		return retrodeck.GetArtDirectory(romDir)
	default:
		return ""
	}
}

func GetArtVideoDirectory(romDir string, platformFSSlug, platformName string) string {
	switch GetCFW() {
	case ROCKNIX:
		return rocknix.GetVideoDirectory(romDir)
	case Knulli:
		return knulli.GetVideoDirectory(romDir)
	case Batocera:
		return batocera.GetVideoDirectory(romDir)
	case RetroDECK:
		return retrodeck.GetVideoDirectory(romDir)
	default:
		return ""

	}
}

func GetArtThumbnailDirectory(romDir string, platformFSSlug, platformName string) string {
	switch GetCFW() {
	case ROCKNIX:
		return rocknix.GetArtDirectory(romDir)
	case Knulli:
		return knulli.GetArtDirectory(romDir)
	case Batocera:
		return knulli.GetArtDirectory(romDir)
	case RetroDECK:
		return retrodeck.GetArtDirectory(romDir)
	default:
		return ""
	}
}

func GetArtBezelDirectory(romDir string, platformFSSlug, platformName string) string {
	switch GetCFW() {
	case ROCKNIX:
		return rocknix.GetBezelDirectory(romDir)
	case Knulli:
		return knulli.GetBezelDirectory(romDir)
	case Batocera:
		return batocera.GetBezelDirectory(romDir)
	case RetroDECK:
		return retrodeck.GetBezelDirectory(romDir)
	default:
		return ""
	}
}

func GetManualDirectory(romDir string, platformFSSlug, platformName string) string {
	switch GetCFW() {
	case ROCKNIX:
		return rocknix.GetManualDirectory(romDir)
	case Knulli:
		return knulli.GetManualDirectory(romDir)
	case Batocera:
		return batocera.GetManualDirectory(romDir)
	case RetroDECK:
		return retrodeck.GetManualDirectory(romDir)
	default:
		return ""
	}
}

func GetBoxbackDirectory(romDir string, platformFSSlug, platformName string) string {
	switch GetCFW() {
	case ROCKNIX:
		return rocknix.GetArtDirectory(romDir)
	case Knulli:
		return knulli.GetArtDirectory(romDir)
	case Batocera:
		return batocera.GetArtDirectory(romDir)
	case RetroDECK:
		return retrodeck.GetArtDirectory(romDir)
	default:
		return ""
	}
}

func GetFanartDirectory(romDir string, platformFSSlug, platformName string) string {
	switch GetCFW() {
	case ROCKNIX:
		return rocknix.GetArtDirectory(romDir)
	case Knulli:
		return knulli.GetArtDirectory(romDir)
	case Batocera:
		return batocera.GetArtDirectory(romDir)
	case RetroDECK:
		return retrodeck.GetArtDirectory(romDir)
	default:
		return ""
	}
}
