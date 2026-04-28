package retrodeck

import (
	"embed"
	"grout/internal/jsonutil"
	"os"
	"path/filepath"
)

//go:embed data/*.json
var embeddedFiles embed.FS

var (
	Platforms = jsonutil.MustLoadJSONMap[string, []string](embeddedFiles, "data/platforms.json")
)

func GetBasePath() string {
	if basePath := os.Getenv("BASE_PATH"); basePath != "" {
		return basePath
	}

	homeInstall := "/home/deck/retrodeck"
	if _, err := os.Stat(homeInstall); err == nil {
		return homeInstall
	}

	sdcardInstall := "/run/media/mmcblk0p1/retrodeck"
	if _, err := os.Stat(sdcardInstall); err == nil {
		return sdcardInstall
	}

	if homePath := os.Getenv("HOME"); homePath != "" {
		customPath := filepath.Join(homePath, "retrodeck")
		if _, err := os.Stat(customPath); err == nil {
			return customPath
		}
	}

	return "/retrodeck"
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
