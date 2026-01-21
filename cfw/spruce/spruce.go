package spruce

import (
	"embed"
	"grout/internal/jsonutil"
	"os"
	"path/filepath"
)

//go:embed data/*.json
var embeddedFiles embed.FS

var (
	Platforms       = jsonutil.MustLoadJSONMap[string, []string](embeddedFiles, "data/platforms.json", "cfw/spruce")
	SaveDirectories = jsonutil.MustLoadJSONMap[string, []string](embeddedFiles, "data/save_directories.json", "cfw/spruce")
)

func GetBasePath() string {
	if basePath := os.Getenv("BASE_PATH"); basePath != "" {
		return basePath
	}
	return "/mnt/SDCARD"
}

func GetRomDirectory() string {
	return filepath.Join(GetBasePath(), "Roms")
}

func GetBIOSDirectory() string {
	return filepath.Join(GetBasePath(), "BIOS")
}

func GetBaseSavePath() string {
	return filepath.Join(GetBasePath(), "Saves", "saves")
}

func GetArtDirectory(romDir string) string {
	return filepath.Join(romDir, "Imgs")
}
