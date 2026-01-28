package rocknix

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
	return "/storage/games-external"
}

func GetRomDirectory() string {
	return filepath.Join(GetBasePath(), "roms")
}

func GetBIOSDirectory() string {
	return filepath.Join(GetRomDirectory(), "bios")
}

func GetBaseSavePath() string {
	// ROCKNIX stores saves alongside ROMs in the platform directory
	return GetRomDirectory()
}

func GetArtDirectory(romDir string) string {
	return filepath.Join(romDir, "images")
}
