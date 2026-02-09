package allium

import (
	"embed"
	"fmt"
	"grout/internal/jsonutil"
	"os"
	"path/filepath"
)

// TODO these are stolen from Spruce... replace with Allium-specific data
//
//go:embed data/*.json
var embeddedFiles embed.FS

//go:embed input_mappings/*.json
var embeddedInputMappings embed.FS

var (
	Platforms       = jsonutil.MustLoadJSONMap[string, []string](embeddedFiles, "data/platforms.json")
	SaveDirectories = jsonutil.MustLoadJSONMap[string, []string](embeddedFiles, "data/save_directories.json")
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

// GetInputMappingBytes returns the embedded input mapping JSON for Allium (Miyoo Mini Plus)
func GetInputMappingBytes() ([]byte, error) {
	filename := "input_mappings/miyoo.json"

	overridePath := filepath.Join("overrides", "cfw", "allium", filename)
	data, err := os.ReadFile(overridePath)
	if err != nil {
		data, err = embeddedInputMappings.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("failed to read embedded input mapping %s: %w", filename, err)
		}
	}

	return data, nil
}
