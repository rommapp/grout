package retrodeck

import (
	"embed"
	"grout/internal/jsonutil"
	"os"
	"path/filepath"
	"sync"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

//go:embed data/*.json
var embeddedFiles embed.FS

var (
	Platforms = jsonutil.MustLoadJSONMap[string, []string](embeddedFiles, "data/platforms.json")

	configPathsOnce sync.Once
	configPaths     *Paths
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

func GetConfigPaths() *Paths {
	configPathsOnce.Do(func() {
		paths, err := LoadConfig()
		if err != nil {
			gaba.GetLogger().Error("Failed to load RetroDECK config", "error", err)
			return
		}
		configPaths = paths
	})
	return configPaths
}

func GetRomDirectory() string {
	if paths := GetConfigPaths(); paths != nil {
		return paths.RomsPath
	}
	return filepath.Join(GetBasePath(), "roms")
}

func GetBIOSDirectory() string {
	if paths := GetConfigPaths(); paths != nil {
		return paths.BiosPath
	}
	return filepath.Join(GetBasePath(), "bios")
}

func GetBaseSavePath() string {
	if paths := GetConfigPaths(); paths != nil {
		return paths.SavesPath
	}
	return filepath.Join(GetBasePath(), "saves")
}

func GetArtDirectory(romDir string) string {
	if paths := GetConfigPaths(); paths != nil {
		return paths.DownloadedMediaPath
	}
	return filepath.Join(romDir, "images")
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

func GetGamelistDirectory() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		if home := os.Getenv("HOME"); home != "" {
			base = filepath.Join(home, ".config")
		}
	}
	return filepath.Join(base, "ES-DE", "gamelists")
}

func GetGamelistPath(romDir, filename string) string {
	system := filepath.Base(romDir)
	return filepath.Join(GetGamelistDirectory(), system, filename)
}
