package muos

import (
	"embed"
	"grout/internal/fileutil"
	"grout/internal/jsonutil"
	"os"
	"path/filepath"
)

//go:embed data/*.json
var embeddedFiles embed.FS

const (
	SD1             = "/mnt/mmc"
	SD2             = "/mnt/sdcard"
	RomsFolderUnion = "/mnt/union/ROMS"
)

var (
	Platforms       = jsonutil.MustLoadJSONMap[string, []string](embeddedFiles, "data/platforms.json", "cfw/muos")
	SaveDirectories = jsonutil.MustLoadJSONMap[string, []string](embeddedFiles, "data/save_directories.json", "cfw/muos")
	ArtDirectories  = jsonutil.MustLoadJSONMap[string, string](embeddedFiles, "data/art_directories.json", "cfw/muos")
)

func GetBasePath() string {
	sd1 := SD1
	sd2 := SD2

	if basePath := os.Getenv("BASE_PATH"); basePath != "" {
		sd1 = filepath.Join(basePath, "mmc")
		sd2 = filepath.Join(basePath, "sdcard")
	}

	// Hack to see if there is actually content on SD2
	sd2InfoDir := filepath.Join(sd2, "MUOS", "info")
	if fileutil.FileExists(sd2InfoDir) {
		return sd2
	}

	return sd1
}

func GetRomDirectory() string {
	if os.Getenv("BASE_PATH") != "" {
		return filepath.Join(GetBasePath(), "ROMS")
	}
	return RomsFolderUnion
}

func GetBIOSDirectory() string {
	return filepath.Join(GetBasePath(), "MUOS", "bios")
}

func GetInfoDirectory() string {
	return filepath.Join(GetBasePath(), "MUOS", "info")
}

func GetBaseSavePath() string {
	return filepath.Join(GetBasePath(), "MUOS", "save")
}

func GetArtDirectory(platformFSSlug, platformName string) string {
	systemName, exists := ArtDirectories[platformFSSlug]
	if !exists {
		systemName = platformName
	}
	return filepath.Join(GetInfoDirectory(), "catalogue", systemName, "box")
}
