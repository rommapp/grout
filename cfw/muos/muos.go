package muos

import (
	"embed"
	"grout/internal/jsonutil"
	"os"
	"path/filepath"
)

//go:embed data/*.json
var embeddedFiles embed.FS

const (
	StoragePath     = "/run/muos/storage"
	RomsFolderUnion = "/mnt/union/ROMS"
)

var (
	Platforms       = jsonutil.MustLoadJSONMap[string, []string](embeddedFiles, "data/platforms.json")
	SaveDirectories = jsonutil.MustLoadJSONMap[string, []string](embeddedFiles, "data/save_directories.json")
	ArtDirectories  = jsonutil.MustLoadJSONMap[string, string](embeddedFiles, "data/art_directories.json")
)

func GetBasePath() string {
	if basePath := os.Getenv("BASE_PATH"); basePath != "" {
		return filepath.Join(basePath, "MUOS")
	}
	return StoragePath
}

func GetRomDirectory() string {
	if basePath := os.Getenv("BASE_PATH"); basePath != "" {
		return filepath.Join(basePath, "ROMS")
	}
	return RomsFolderUnion
}

func GetBIOSDirectory() string {
	return filepath.Join(GetBasePath(), "bios")
}

func GetInfoDirectory() string {
	return filepath.Join(GetBasePath(), "info")
}

func GetBaseSavePath() string {
	return filepath.Join(GetBasePath(), "save")
}

func GetArtDirectory(platformFSSlug, platformName string) string {
	systemName, exists := ArtDirectories[platformFSSlug]
	if !exists {
		systemName = platformName
	}
	return filepath.Join(GetInfoDirectory(), "catalogue", systemName, "box")
}

func GetTextDirectory(platformFSSlug, platformName string) string {
	systemName, exists := ArtDirectories[platformFSSlug]
	if !exists {
		systemName = platformName
	}
	return filepath.Join(GetInfoDirectory(), "catalogue", systemName, "text")
}

func GetPreviewDirectory(platformFSSlug, platformName string) string {
	systemName, exists := ArtDirectories[platformFSSlug]
	if !exists {
		systemName = platformName
	}
	return filepath.Join(GetInfoDirectory(), "catalogue", systemName, "preview")
}

func GetSplashDirectory(platformFSSlug, platformName string) string {
	systemName, exists := ArtDirectories[platformFSSlug]
	if !exists {
		systemName = platformName
	}
	return filepath.Join(GetInfoDirectory(), "catalogue", systemName, "splash")
}
