package spruce

import (
	"embed"
	"grout/tables"
	"os"
	"path/filepath"
)

//go:embed data/*.json
var embeddedFiles embed.FS

var (
	Platforms       = tables.MustLoad[string, []string](embeddedFiles, "data/platforms.json")
	SaveDirectories = tables.MustLoad[string, []string](embeddedFiles, "data/save_directories.json")
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
