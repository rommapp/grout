package esde

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

func GetBasePath() string {
	if basePath := os.Getenv("BASE_PATH"); basePath != "" {
		return basePath
	}
	return filepath.Join(os.Getenv("HOME"), "Emulation")
}

func GetRomDirectory() string {
	return filepath.Join(GetBasePath(), "roms")
}

func GetBIOSDirectory() string {
	return filepath.Join(GetBasePath(), "bios")
}

func GetBaseSavePath() string {
	return filepath.Join(GetBasePath(), "saves")
}

func GetArtDirectory(romDir string) string {
	return filepath.Join(romDir, "images")
}

func GetGroutGamelist() string {
	return filepath.Join(GetRomDirectory(), "ports", "gamelist.xml")
}

func GetVideoDirectory(romDir string) string {
	return filepath.Join(romDir, "videos")
}

func GetBezelDirectory(romDir string) string {
	return filepath.Join(romDir, "bezels")
}

func GetManualDirectory(romDir string) string {
	return filepath.Join(romDir, "manuals")
}
