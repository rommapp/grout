package amberelec

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"grout/internal/jsonutil"
)

//go:embed data/*.json input_mappings/*.json
var embeddedFiles embed.FS

type Device string

const (
	DeviceGeneric Device = "generic"
	DeviceRG351V  Device = "rg351v"
)

// Platforms is sourced from dedicated AmberELEC data instead of inheriting ROCKNIX folders at runtime.
// AmberELEC wiki systems that are still absent from RomM fs_slugs remain intentionally out of
// platforms.json for now: atomiswave, laserdisc, naomi, advision, gamepocketcomputer, gamate,
// gamemaster, gamecom, gameking, gameking3, pspminis, pv1000, satellaview, scv, sufami,
// tvboy, uzebox, vsmile, chip-8, lowresnx, piece, vircon32, wasm4, build, doom, easyrpg,
// ecwolf, scummvm, solarus, zmachine, ep64-128, sc-3000, thomson, tvc.
var Platforms = jsonutil.MustLoadJSONMap[string, []string](embeddedFiles, "data/platforms.json")

func DetectDevice() Device {
	arch, err := os.ReadFile("/storage/.config/.OS_ARCH")
	if err != nil {
		return DeviceGeneric
	}

	switch strings.ToUpper(strings.TrimSpace(string(arch))) {
	case "RG351V":
		return DeviceRG351V
	default:
		return DeviceGeneric
	}
}

func GetInputMappingBytes() ([]byte, error) {
	var filename string
	switch DetectDevice() {
	case DeviceRG351V:
		filename = "input_mappings/rg351v.json"
	default:
		return nil, nil
	}

	overridePath := filepath.Join("overrides", "cfw", "amberelec", filename)
	data, err := os.ReadFile(overridePath)
	if err != nil {
		data, err = embeddedFiles.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("failed to read embedded input mapping %s: %w", filename, err)
		}
	}

	return data, nil
}

func GetBasePath() string {
	if basePath := os.Getenv("BASE_PATH"); basePath != "" {
		return basePath
	}
	return "/storage"
}

func GetRomDirectory() string {
	return filepath.Join(GetBasePath(), "roms")
}

func GetBIOSDirectory() string {
	return filepath.Join(GetRomDirectory(), "bios")
}

func GetBaseSavePath() string {
	return GetRomDirectory()
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

func GetManualDirectory(romDir string) string {
	return filepath.Join(romDir, "manuals")
}

func GetBezelDirectory(romDir string) string {
	return filepath.Join(romDir, "bezels")
}
