package knulli

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
	return "/userdata"
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
	return filepath.Join(GetRomDirectory(), "tools", "gamelist.xml")
}
