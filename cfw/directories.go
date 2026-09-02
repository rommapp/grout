package cfw

import (
	"grout/cfw/allium"
	"grout/cfw/arkos"
	"grout/cfw/batocera"
	"grout/cfw/knulli"
	"grout/cfw/koriki"
	"grout/cfw/minui"
	"grout/cfw/muos"
	"grout/cfw/nextui"
	"grout/cfw/onion"
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
	case Koriki:
		return koriki.GetRomDirectory()
	case ArkOS:
		return arkos.GetRomDirectory()
	case Batocera:
		return batocera.GetRomDirectory()
	case MinUI:
		return minui.GetRomDirectory()
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
	case Koriki:
		return koriki.GetBIOSDirectory()
	case ArkOS:
		return arkos.GetBIOSDirectory()
	case Batocera:
		return batocera.GetBIOSDirectory()
	case MinUI:
		return minui.GetBIOSDirectory()
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

// coverDirectory returns where a firmware keeps cover art for a platform.
//
// Most firmwares put artwork beside the roms, so they need only the rom
// directory. muOS and TrimUI keep a separate catalogue keyed by platform
// instead, which is why this takes both and each firmware uses what it needs.
func coverDirectory(c CFW, romDir, platformFSSlug, platformName string) string {
	switch c {
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
	case Koriki:
		return koriki.GetArtDirectory(romDir)
	case ArkOS:
		return arkos.GetArtDirectory(romDir)
	case Batocera:
		return batocera.GetArtDirectory(romDir)
	case MinUI:
		return minui.GetArtDirectory(romDir)
	default:
		return ""
	}
}

// esSidecarDirectories returns the video, manual and bezel directories for the
// EmulationStation family, which are the only firmwares that keep them.
func esSidecarDirectories(c CFW, romDir string) (video, manual, bezel string) {
	switch c {
	case ROCKNIX:
		return rocknix.GetVideoDirectory(romDir), rocknix.GetManualDirectory(romDir), rocknix.GetBezelDirectory(romDir)
	case ArkOS:
		return arkos.GetVideoDirectory(romDir), arkos.GetManualDirectory(romDir), arkos.GetBezelDirectory(romDir)
	case Knulli:
		return knulli.GetVideoDirectory(romDir), knulli.GetManualDirectory(romDir), knulli.GetBezelDirectory(romDir)
	case Batocera:
		return batocera.GetVideoDirectory(romDir), batocera.GetManualDirectory(romDir), batocera.GetBezelDirectory(romDir)
	default:
		return "", "", ""
	}
}

// ArtDirectory returns where firmware c keeps a given kind of artwork for a
// platform, or "" when it keeps none.
//
// This replaced nine near-identical functions that each switched over the same
// firmwares. Four of them were textually the same, one routed Batocera to
// Knulli's directory, and one had no callers at all -- which is what happens
// when adding an art kind means copying a switch statement.
//
// An empty result means the firmware has nowhere to put this kind. Callers
// must treat that as "skip", not as an error.
func ArtDirectory(c CFW, slot ArtSlot, romDir, platformFSSlug, platformName string) string {
	switch slot {
	case ArtCover:
		return coverDirectory(c, romDir, platformFSSlug, platformName)

	case ArtMarquee, ArtBoxback, ArtFanart:
		// The EmulationStation family keeps these beside the cover and tells
		// them apart by a filename suffix; see ArtFileName. No other firmware
		// has anywhere to put them.
		if !c.IsBasedOnEmulationStation() {
			return ""
		}
		return coverDirectory(c, romDir, platformFSSlug, platformName)

	case ArtVideo:
		video, _, _ := esSidecarDirectories(c, romDir)
		return video
	case ArtManual:
		_, manual, _ := esSidecarDirectories(c, romDir)
		return manual
	case ArtBezel:
		_, _, bezel := esSidecarDirectories(c, romDir)
		return bezel

	case ArtScreenshotPreview:
		if c == MuOS {
			return muos.GetPreviewDirectory(platformFSSlug, platformName)
		}
		return ""

	case ArtThumbnail:
		// muOS calls this the splash directory.
		if c == MuOS {
			return muos.GetSplashDirectory(platformFSSlug, platformName)
		}
		return ""

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
	case Koriki:
		return koriki.GetBaseSavePath()
	case ArkOS:
		return arkos.GetBaseSavePath()
	case Batocera:
		return batocera.GetBaseSavePath()
	case MinUI:
		return minui.GetBaseSavePath()
	}
	return ""
}
