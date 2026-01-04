package cfw

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

type CFW string

const (
	NextUI CFW = "NEXTUI"
	MuOS   CFW = "MUOS"
	Knulli CFW = "KNULLI"
	Spruce CFW = "SPRUCE"
)

var (
	NextUIPlatforms       = mustLoadJSONMap[string, []string]("nextui/platforms.json")
	NextUISaveDirectories = mustLoadJSONMap[string, []string]("nextui/save_directories.json")

	MuOSPlatforms       = mustLoadJSONMap[string, []string]("muos/platforms.json")
	MuOSSaveDirectories = mustLoadJSONMap[string, []string]("muos/save_directories.json")
	MuOSArtDirectory    = mustLoadJSONMap[string, string]("muos/art_directories.json")

	SprucePlatforms       = mustLoadJSONMap[string, []string]("spruce/platforms.json")
	SpruceSaveDirectories = mustLoadJSONMap[string, []string]("spruce/save_directories.json")

	KnulliPlatforms = mustLoadJSONMap[string, []string]("knulli/platforms.json")
)

const MuOSSD1 = "/mnt/mmc"
const MuOSSD2 = "/mnt/sdcard"
const MuOSRomsFolderUnion = "/mnt/union/ROMS"

func GetCFW() CFW {
	cfwEnv := strings.ToUpper(os.Getenv("CFW"))
	cfw := CFW(cfwEnv)

	switch cfw {
	case MuOS, NextUI, Knulli, Spruce:
		return cfw
	default:
		log.SetOutput(os.Stderr)
		log.Fatalf("Unsupported CFW: '%s'. Valid options: NextUI, muOS, Knulli, Spruce", cfwEnv)
		return ""
	}
}

func GetRomDirectory() string {
	cfw := GetCFW()

	switch cfw {
	case MuOS:
		// For MuOS, use union path in production, or derive from base path in dev
		if os.Getenv("BASE_PATH") != "" {
			return filepath.Join(getBasePath(MuOS), "ROMS")
		}
		return MuOSRomsFolderUnion
	case NextUI:
		return filepath.Join(getBasePath(NextUI), "Roms")
	case Knulli:
		return filepath.Join(getBasePath(Knulli), "roms")
	case Spruce:
		return filepath.Join(getBasePath(Spruce), "Roms")
	}

	return ""
}

func GetBIOSDirectory() string {
	cfw := GetCFW()

	switch cfw {
	case NextUI:
		return filepath.Join(getBasePath(NextUI), "Bios")
	case MuOS:
		return filepath.Join(getBasePath(MuOS), "MUOS", "bios")
	case Knulli:
		return filepath.Join(getBasePath(Knulli), "bios")
	case Spruce:
		return filepath.Join(getBasePath(Spruce), "BIOS")
	}

	return ""
}

func GetBIOSFilePaths(relativePath string, platformSlug string) []string {
	biosDir := GetBIOSDirectory()
	c := GetCFW()

	if c == NextUI {
		tags, ok := NextUISaveDirectories[platformSlug]
		if ok && len(tags) > 0 {
			paths := make([]string, 0, len(tags))
			filename := filepath.Base(relativePath)
			for _, platformTag := range tags {
				paths = append(paths, filepath.Join(biosDir, platformTag, filename))
			}
			return paths
		}
	}

	return []string{filepath.Join(biosDir, relativePath)}
}

func GetPlatformMap(c CFW) map[string][]string {
	switch c {
	case MuOS:
		return MuOSPlatforms
	case NextUI:
		return NextUIPlatforms
	case Knulli:
		return KnulliPlatforms
	case Spruce:
		return SprucePlatforms
	default:
		return nil
	}
}

func EmulatorFolderMap(c CFW) map[string][]string {
	switch c {
	case MuOS:
		return MuOSSaveDirectories
	case NextUI:
		return NextUISaveDirectories
	case Knulli:
		return KnulliPlatforms
	case Spruce:
		return SpruceSaveDirectories
	default:
		return nil
	}
}

func EmulatorFoldersForSlug(slug string) []string {
	saveDirectoriesMap := EmulatorFolderMap(GetCFW())
	if saveDirectoriesMap == nil {
		return nil
	}
	return saveDirectoriesMap[slug]
}

func RomMSlugToCFW(slug string) string {
	cfwPlatformMap := GetPlatformMap(GetCFW())
	if cfwPlatformMap == nil {
		return strings.ToLower(slug)
	}

	if value, ok := cfwPlatformMap[slug]; ok {
		if len(value) > 0 {
			return value[0]
		}

		return ""
	}

	return strings.ToLower(slug)
}

func getBasePath(cfw CFW) string {
	switch cfw {
	case MuOS:
		sd1 := MuOSSD1
		sd2 := MuOSSD2

		if basePath := os.Getenv("BASE_PATH"); basePath != "" {
			sd1 = filepath.Join(basePath, "mmc")
			sd2 = filepath.Join(basePath, "sdcard")
		}

		// TODO verify if this actually works
		// Hack to see if there is actually content
		sd2InfoDir := filepath.Join(sd2, "MUOS", "info")
		if _, err := os.Stat(sd2InfoDir); err == nil {
			gaba.GetLogger().Debug("Using MUOS Base Path", "path", sd2)
			return sd2
		}

		gaba.GetLogger().Debug("Using MUOS Base Path", "path", sd1)
		return sd1

	case NextUI:
		return "/mnt/SDCARD"

	case Knulli:
		return "/userdata"

	case Spruce:
		return "/mnt/SDCARD"
	default:
		return ""
	}
}

func GetMuOSInfoDirectory() string {
	return filepath.Join(getBasePath(MuOS), "MUOS", "info")
}

func BaseSavePath() string {
	cfw := GetCFW()
	switch cfw {
	case MuOS:
		return filepath.Join(getBasePath(cfw), "MUOS", "save", "file")
	case NextUI:
		return filepath.Join(getBasePath(cfw), "Saves")
	case Knulli:
		return filepath.Join(getBasePath(cfw), "saves")
	case Spruce:
		return filepath.Join(getBasePath(cfw), "Saves", "saves")
	}

	return ""
}

// GetPlatformRomDirectory returns the ROM directory for a platform.
// relativePath is the configured relative path from directory mappings.
// platformSlug is used as fallback if relativePath is empty.
func GetPlatformRomDirectory(relativePath, platformSlug string) string {
	rp := relativePath
	if rp == "" {
		rp = RomMSlugToCFW(platformSlug)
	}
	return filepath.Join(GetRomDirectory(), rp)
}

// GetArtDirectory returns the artwork directory for a platform.
func GetArtDirectory(romDir string, platformSlug, platformName string) string {
	switch GetCFW() {
	case NextUI:
		return filepath.Join(romDir, ".media")
	case Knulli:
		return filepath.Join(romDir, "images")
	case Spruce:
		return filepath.Join(romDir, "Imgs")
	case MuOS:
		systemName, exists := MuOSArtDirectory[platformSlug]
		if !exists {
			systemName = platformName
		}
		return filepath.Join(GetMuOSInfoDirectory(), "catalogue", systemName, "box")
	default:
		return ""
	}
}

// RomFolderBase returns the base folder name for ROM matching.
// tagParser is a function that extracts tags from paths (for NextUI).
func RomFolderBase(path string, tagParser func(string) string) string {
	switch GetCFW() {
	case NextUI:
		if tagParser != nil {
			return tagParser(path)
		}
		return path
	default:
		return path
	}
}
