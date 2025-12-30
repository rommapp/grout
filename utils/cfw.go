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
	cfwEnv := strings.ToUpper(os.Getenv("CFW"))
	cfw := constants.CFW(cfwEnv)

	switch cfw {
	case constants.MuOS, constants.NextUI, constants.Knulli:
		return cfw
	default:
		LogStandardFatal(
			fmt.Sprintf("Unsupported CFW: '%s'. Valid options: NextUI, muOS, Knulli", cfwEnv),
			nil,
		)
		return ""
	}
}

func GetRomDirectory() string {
	cfw := GetCFW()

	switch cfw {
	case constants.MuOS:
		// For MuOS, use union path in production, or derive from base path in dev
		if os.Getenv("BASE_PATH") != "" || os.Getenv("MUOS_BASE_PATH") != "" {
			return filepath.Join(getBasePath(constants.MuOS), "ROMS")
		}
		return constants.MuOSRomsFolderUnion
	case constants.NextUI:
		return filepath.Join(getBasePath(constants.NextUI), "Roms")
	case constants.Knulli:
		return filepath.Join(getBasePath(constants.Knulli), "roms")
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
	case constants.Knulli:
		romDir := GetPlatformRomDirectory(config, platform)
		return filepath.Join(romDir, "images")
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

func GetBIOSDirectory() string {
	cfw := GetCFW()

	switch cfw {
	case constants.NextUI:
		return filepath.Join(getBasePath(constants.NextUI), "Bios")
	case constants.MuOS:
		return filepath.Join(getBasePath(constants.MuOS), "MUOS", "bios")
	case constants.Knulli:
		return filepath.Join(getBasePath(constants.Knulli), "bios")
	}

	return ""
}

func GetBIOSFilePaths(biosFile constants.BIOSFile, platformSlug string) []string {
	biosDir := GetBIOSDirectory()
	cfw := GetCFW()

	if cfw == constants.NextUI {
		tags, ok := constants.NextUISaveDirectories[platformSlug]
		if ok && len(tags) > 0 {
			paths := make([]string, 0, len(tags))
			filename := filepath.Base(biosFile.RelativePath)
			for _, platformTag := range tags {
				paths = append(paths, filepath.Join(biosDir, platformTag, filename))
			}
			return paths
		}
	}

	// For other CFWs or if no tag mapping exists, honor subdirectories from firmware*_path
	// e.g., "psx/scph5500.bin" → "/path/to/BIOS/psx/scph5500.bin"
	return []string{filepath.Join(biosDir, biosFile.RelativePath)}
}

func GetPlatformMap(cfw constants.CFW) map[string][]string {
	switch cfw {
	case constants.MuOS:
		return constants.MuOSPlatforms
	case constants.NextUI:
		return constants.NextUIPlatforms
	case constants.Knulli:
		return constants.KnulliPlatforms
	default:
		return nil
	}
}

func EmulatorFolderMap(cfw constants.CFW) map[string][]string {
	switch cfw {
	case constants.MuOS:
		return constants.MuOSSaveDirectories
	case constants.NextUI:
		return constants.NextUISaveDirectories
	case constants.Knulli:
		return constants.KnulliSaveDirectories
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

func getBasePath(cfw constants.CFW) string {
	if basePath := os.Getenv("BASE_PATH"); basePath != "" {
		return basePath
	}

	switch cfw {
	case constants.MuOS:
		if os.Getenv("MUOS_BASE_PATH") != "" {
			return os.Getenv("MUOS_BASE_PATH")
		}
		// Hack to see if there is actually content
		sd2InfoDir := filepath.Join(constants.MuOSSD2, "MuOS", "info")
		if _, err := os.Stat(sd2InfoDir); err == nil {
			return constants.MuOSSD2
		}
		return constants.MuOSSD1

	case constants.NextUI:
		if os.Getenv("NEXTUI_BASE_PATH") != "" {
			return os.Getenv("NEXTUI_BASE_PATH")
		}
		return "/mnt/SDCARD"

	case constants.Knulli:
		if os.Getenv("KNULLI_BASE_PATH") != "" {
			return os.Getenv("KNULLI_BASE_PATH")
		}
		return "/userdata"

	default:
		return ""
	}
}

func getMuOSInfoDirectory() string {
	return filepath.Join(getBasePath(constants.MuOS), "MUOS", "info")
}

func BaseSavePath() string {
	cfw := GetCFW()
	switch cfw {
	case constants.MuOS:
		return filepath.Join(getBasePath(cfw), "MUOS", "save", "file")
	case constants.NextUI:
		return filepath.Join(getBasePath(cfw), "Saves")
	case constants.Knulli:
		return filepath.Join(getBasePath(cfw), "saves")
	}

	return ""
}
