package minui

import (
	"embed"
	"grout/internal/jsonutil"
	"os"
	"path/filepath"
)

//go:embed data/*.json
var embeddedFiles embed.FS

var (
	Platforms       = jsonutil.MustLoadJSONMap[string, []string](embeddedFiles, "data/platforms.json")
	SaveDirectories = jsonutil.MustLoadJSONMap[string, []string](embeddedFiles, "data/save_directories.json")
)

var cachedBasePath string

func GetBasePath() string {
	if basePath := os.Getenv("BASE_PATH"); basePath != "" {
		return basePath
	}

	if cachedBasePath != "" {
		return cachedBasePath
	}

	for _, candidate := range []string{"/mnt/SDCARD", "/mnt/sdcard"} {
		if _, err := os.Stat(candidate); err == nil {
			cachedBasePath = candidate
			return cachedBasePath
		}
	}

	cachedBasePath = "/mnt/mmc"
	return cachedBasePath
}

func GetRomDirectory() string {
	return filepath.Join(GetBasePath(), "Roms")
}

func GetBIOSDirectory() string {
	return filepath.Join(GetBasePath(), "Bios")
}

func GetBaseSavePath() string {
	return filepath.Join(GetBasePath(), "Saves")
}

func GetArtDirectory(romDir string) string {
	return filepath.Join(romDir, ".res")
}

func GetBIOSFilePaths(relativePath, platformFSSlug string) []string {
	biosDir := GetBIOSDirectory()

	tags, ok := SaveDirectories[platformFSSlug]
	if ok && len(tags) > 0 {
		paths := make([]string, 0, len(tags))
		filename := filepath.Base(relativePath)
		for _, platformTag := range tags {
			paths = append(paths, filepath.Join(biosDir, platformTag, filename))
		}
		return paths
	}

	return []string{filepath.Join(biosDir, relativePath)}
}

// RomFolderBase returns the base folder name for ROM matching using the tag parser.
func RomFolderBase(path string, tagParser func(string) string) string {
	if tagParser != nil {
		return tagParser(path)
	}
	return path
}
