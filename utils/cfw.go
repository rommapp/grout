package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"grout/constants"
	"grout/romm"
)

func GetCFW() constants.CFW {
	cfw := strings.ToLower(os.Getenv("CFW"))
	switch cfw {
	case "muos":
		return constants.MuOS
	case "nextui":
		return constants.NextUI
	default:
		LogStandardFatal(fmt.Sprintf("Unsupported CFW: %s", cfw), nil)
	}
	return ""
}

func GetRomDirectory() string {
	if os.Getenv("ROM_DIRECTORY") != "" {
		return os.Getenv("ROM_DIRECTORY")
	}

	cfw := GetCFW()

	switch cfw {
	case constants.MuOS:
		return constants.MuOSRomsFolderUnion
	case constants.NextUI:
		return filepath.Join(getNextUIBasePath(), "Roms")
	case constants.Knulli:
		return filepath.Join(getKnulliBasePath(), "roms")

	}

	return ""
}

func GetPlatformRomDirectory(config Config, platform romm.Platform) string {
	rp := config.DirectoryMappings[platform.Slug].RelativePath

	if rp == "" {
		rp = RomMSlugToCFW(platform.Slug)
	}

	return filepath.Join(GetRomDirectory(), rp)
}

func GetArtDirectory(config Config, platform romm.Platform) string {
	switch GetCFW() {
	case constants.NextUI:
		romDir := GetPlatformRomDirectory(config, platform)
		return filepath.Join(romDir, ".media")
	case constants.MuOS:
		systemName, exists := constants.MuOSArtDirectory[platform.Slug]
		if !exists {
			systemName = platform.Name
		}
		muosInfoDir := getMuOSInfoDirectory()
		return filepath.Join(muosInfoDir, "catalogue", systemName, "box")
	default:
		return ""
	}
}

func RomMSlugToCFW(slug string) string {
	var cfwPlatformMap map[string][]string

	switch GetCFW() {
	case constants.MuOS:
		cfwPlatformMap = constants.MuOSPlatforms
	case constants.NextUI:
		cfwPlatformMap = constants.NextUIPlatforms
	case constants.Knulli:
		cfwPlatformMap = constants.KnulliPlatforms
	}

	if value, ok := cfwPlatformMap[slug]; ok {
		if len(value) > 0 {
			return value[0]
		}

		return ""
	}

	return strings.ToLower(slug)
}

func RomFolderBase(path string) string {
	switch GetCFW() {
	case constants.MuOS, constants.Knulli:
		return path
	case constants.NextUI:
		return ParseTag(path)
	default:
		return path
	}
}

func getMuOSBasePath() string {
	if os.Getenv("MUOS_BASE_PATH") != "" {
		return os.Getenv("MUOS_BASE_PATH")
	}

	// Hack to see if there is actually content
	sd2InfoDir := filepath.Join(constants.MuOSSD2, "MuOS", "info")
	if _, err := os.Stat(sd2InfoDir); err == nil {
		return constants.MuOSSD2
	}

	return constants.MuOSSD1
}

func getMuOSInfoDirectory() string {
	return filepath.Join(getMuOSBasePath(), "MUOS", "info")
}

func getNextUIBasePath() string {
	if os.Getenv("NEXTUI_BASE_PATH") != "" {
		return os.Getenv("NEXTUI_BASE_PATH")
	}

	return "/mnt/SDCARD"
}

func getKnulliBasePath() string {
	if os.Getenv("KNULLI_BASE_PATH") != "" {
		return os.Getenv("KNULLI_BASE_PATH")
	}

	return "/userdata"
}

func getSaveDirectory() string {
	switch GetCFW() {
	case constants.MuOS:
		return filepath.Join(getMuOSBasePath(), "MUOS", "save", "file")
	case constants.NextUI:
		return filepath.Join(getNextUIBasePath(), "Saves")
	case constants.Knulli:
		return filepath.Join(getKnulliBasePath(), "saves")
	}

	return ""
}
