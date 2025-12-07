package utils

import (
	"fmt"
	"grout/models"
	"os"
	"path/filepath"
	"strings"

	"github.com/brandonkowalski/go-romm"
)

func GetCFW() models.CFW {
	cfw := strings.ToLower(os.Getenv("CFW"))
	switch cfw {
	case "muos":
		return models.MUOS
	case "nextui":
		return models.NEXTUI
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
	case models.MUOS:
		return muOSRomsFolderUnion
	case models.NEXTUI:
		return nextUIRomsFolder
	}

	return ""
}

func GetPlatformRomDirectory(config models.Config, platform romm.Platform) string {
	rp := config.DirectoryMappings[platform.Slug].RelativePath

	if rp == "" {
		rp = RomMSlugToCFW(platform.Slug)
	}

	return filepath.Join(GetRomDirectory(), rp)
}

func RomMSlugToCFW(slug string) string {
	var cfwPlatformMap map[string][]string

	switch GetCFW() {
	case models.MUOS:
		cfwPlatformMap = muOSPlatforms
	case models.NEXTUI:
		cfwPlatformMap = nextUIPlatforms
	}

	if value, ok := cfwPlatformMap[slug]; ok {
		if len(value) > 0 {
			return value[0]
		}

		return ""
	} else {
		return strings.ToLower(slug)
	}
}

func RomFolderBase(path string) string {
	switch GetCFW() {
	case models.MUOS:
		return path
	case models.NEXTUI:
		_, tag := ItemNameCleaner(path, true)
		return tag
	default:
		return path
	}
}
