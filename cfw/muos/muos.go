package muos

import (
	"embed"
	"grout/tables"
	"os"
	"path/filepath"
	"strings"
)

//go:embed data/*.json
var embeddedFiles embed.FS

const (
	StoragePath     = "/run/muos/storage"
	RomsFolderUnion = "/mnt/union/ROMS"
)

var (
	Platforms       = tables.MustLoad[string, []string](embeddedFiles, "data/platforms.json")
	SaveDirectories = tables.MustLoad[string, []string](embeddedFiles, "data/save_directories.json")
	ArtDirectories  = tables.MustLoad[string, string](embeddedFiles, "data/art_directories.json")
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

// EmulatorLabel is what to call a save folder on screen.
//
// muOS stores saves under paths like "file/PPSSPP/backup", where the parts
// around the emulator's name are storage layout rather than anything the user
// chose between.
func EmulatorLabel(dir string) string {
	trimmed := strings.ReplaceAll(dir, "file/", "")
	trimmed = strings.ReplaceAll(trimmed, "/backup", "")
	return filepath.Base(trimmed)
}
