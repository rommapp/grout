package cfw

import (
	"grout/cfw/knulli"
	"grout/internal/fileutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type CFW string

const (
	NextUI CFW = "NEXTUI"
	MuOS   CFW = "MUOS"
	Knulli CFW = "KNULLI"
	Spruce CFW = "SPRUCE"
)

const (
	MuOSSD1             = "/mnt/mmc"
	MuOSSD2             = "/mnt/sdcard"
	MuOSRomsFolderUnion = "/mnt/union/ROMS"
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

// platformAliasMap is computed from platform mappings - slugs that map to
// overlapping local folders are considered aliases (e.g., sfam/snes both map to "snes")
var platformAliasMap = buildPlatformAliasMap()

func buildPlatformAliasMap() map[string][]string {
	// Combine all platform maps to find aliases across all CFWs
	allMaps := []map[string][]string{
		KnulliPlatforms,
		MuOSPlatforms,
		NextUIPlatforms,
		SprucePlatforms,
	}

	// Build reverse map: primary folder -> list of RomM slugs that use it as primary
	// Only use the FIRST folder in each list (the primary/default folder)
	// This avoids false aliases like arcade/neogeoaes which share "neogeo" as a secondary folder
	primaryFolderToSlugs := make(map[string]map[string]bool)
	for _, platformMap := range allMaps {
		for slug, folders := range platformMap {
			if len(folders) == 0 {
				continue
			}
			// Use only the primary (first) folder
			primary := strings.ToLower(folders[0])
			if primaryFolderToSlugs[primary] == nil {
				primaryFolderToSlugs[primary] = make(map[string]bool)
			}
			primaryFolderToSlugs[primary][slug] = true
		}
	}

	// Find slug groups that share the same primary folder using union-find
	parent := make(map[string]string)
	var find func(s string) string
	find = func(s string) string {
		if parent[s] == "" {
			parent[s] = s
		}
		if parent[s] != s {
			parent[s] = find(parent[s])
		}
		return parent[s]
	}
	union := func(a, b string) {
		pa, pb := find(a), find(b)
		if pa != pb {
			parent[pa] = pb
		}
	}

	// Union slugs that share the same primary folder
	for _, slugs := range primaryFolderToSlugs {
		var slugList []string
		for slug := range slugs {
			slugList = append(slugList, slug)
		}
		for i := 1; i < len(slugList); i++ {
			union(slugList[0], slugList[i])
		}
	}

	// Group slugs by their root parent
	groups := make(map[string][]string)
	for slug := range parent {
		root := find(slug)
		groups[root] = append(groups[root], slug)
	}

	// Build final alias map (only for groups with more than one slug)
	result := make(map[string][]string)
	for _, group := range groups {
		if len(group) > 1 {
			for _, slug := range group {
				result[slug] = group
			}
		}
	}

	return result
}

// GetPlatformAliases returns all equivalent platform slugs for the given slug.
// Aliases are RomM slugs that map to overlapping local folders across CFWs.
// Returns a slice containing at least the input slug itself.
func GetPlatformAliases(fsSlug string) []string {
	if aliases, ok := platformAliasMap[fsSlug]; ok {
		return aliases
	}
	return []string{fsSlug}
}

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
		if os.Getenv("BASE_PATH") != "" {
			return filepath.Join(os.Getenv("BASE_PATH"), "ROMS")
		}
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

func GetBIOSFilePaths(relativePath string, platformFSSlug string) []string {
	biosDir := GetBIOSDirectory()
	c := GetCFW()

	if c == NextUI {
		tags, ok := NextUISaveDirectories[platformFSSlug]
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

func EmulatorFoldersForFSSlug(fsSlug string) []string {
	saveDirectoriesMap := EmulatorFolderMap(GetCFW())
	if saveDirectoriesMap == nil {
		return nil
	}
	return saveDirectoriesMap[fsSlug]
}

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
		if fileutil.FileExists(sd2InfoDir) {
			return sd2
		}

		return sd1

	case NextUI:
		if basePath := os.Getenv("BASE_PATH"); basePath != "" {
			return basePath
		}
		return "/mnt/SDCARD"

	case Knulli:
		if basePath := os.Getenv("BASE_PATH"); basePath != "" {
			return basePath
		}
		return "/userdata"

	case Spruce:
		if basePath := os.Getenv("BASE_PATH"); basePath != "" {
			return basePath
		}
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
		return filepath.Join(romDir, ".media")
	case Knulli:
		return filepath.Join(romDir, "images")
	case Spruce:
		return filepath.Join(romDir, "Imgs")
	case MuOS:
		systemName, exists := MuOSArtDirectory[platformFSSlug]
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

func FirstLaunchSetup() {
	switch GetCFW() {
	case Knulli:
		knulli.FirstRunSetup(GetRomDirectory())
	default:
		return
	}
}
