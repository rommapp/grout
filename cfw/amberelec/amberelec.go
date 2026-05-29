package amberelec

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"grout/cfw/rocknix"
)

//go:embed input_mappings/*.json
var embeddedInputMappings embed.FS

type Device string

const (
	DeviceGeneric Device = "generic"
	DeviceRG351V  Device = "rg351v"
)

var Platforms = buildPlatforms()

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
		data, err = embeddedInputMappings.ReadFile(filename)
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

func buildPlatforms() map[string][]string {
	platforms := clonePlatformMap(rocknix.Platforms)

	setFolders(platforms, "gamegear", "gamegear", "gamegearh")
	setFolders(platforms, "gb", "gb", "gbh")
	setFolders(platforms, "gba", "gba", "gbah")
	setFolders(platforms, "gbc", "gbc", "gbch")
	setFolders(platforms, "genesis", "genesis", "megadrive", "genh", "megadrive-japan")
	setFolders(platforms, "nes", "nes", "nesh")
	setFolders(platforms, "sfam", "sfc", "snes")
	setFolders(platforms, "snes", "snes", "sfc", "snesh", "snesmsu1")

	setFolders(platforms, "gamegearh", "gamegearh")
	setFolders(platforms, "gbh", "gbh")
	setFolders(platforms, "gbah", "gbah")
	setFolders(platforms, "gbch", "gbch")
	setFolders(platforms, "genh", "genh")
	setFolders(platforms, "megadrive-japan", "megadrive-japan")
	setFolders(platforms, "nesh", "nesh")
	setFolders(platforms, "sfc", "sfc")
	setFolders(platforms, "snesh", "snesh")
	setFolders(platforms, "snesmsu1", "snesmsu1")
	setFolders(platforms, "vic-20", "vic20")

	// AmberELEC wiki systems that are not currently exposed as RomM fs_slugs.
	// Uncomment or adjust these when RomM adds the corresponding platform keys.
	// setFolders(platforms, "atomiswave", "atomiswave")
	// setFolders(platforms, "laserdisc", "laserdisc")
	// setFolders(platforms, "naomi", "naomi")
	// setFolders(platforms, "advision", "advision")
	// setFolders(platforms, "gamepocketcomputer", "gamepocketcomputer")
	// setFolders(platforms, "gamate", "gamate")
	// setFolders(platforms, "gamemaster", "gamemaster")
	// setFolders(platforms, "gamecom", "gamecom")
	// setFolders(platforms, "gameking", "gameking")
	// setFolders(platforms, "gameking3", "gameking3")
	// setFolders(platforms, "pspminis", "pspminis")
	// setFolders(platforms, "pv1000", "pv1000")
	// setFolders(platforms, "satellaview", "satellaview")
	// setFolders(platforms, "scv", "scv")
	// setFolders(platforms, "sufami", "sufami")
	// setFolders(platforms, "vsmile", "vsmile")
	// setFolders(platforms, "chip-8", "chip-8")
	// setFolders(platforms, "lowresnx", "lowresnx")
	// setFolders(platforms, "piece", "piece")
	// setFolders(platforms, "vircon32", "vircon32")
	// setFolders(platforms, "wasm4", "wasm4")
	// setFolders(platforms, "build", "build")
	// setFolders(platforms, "doom", "doom")
	// setFolders(platforms, "easyrpg", "easyrpg")
	// setFolders(platforms, "ecwolf", "ecwolf")
	// setFolders(platforms, "scummvm", "scummvm")
	// setFolders(platforms, "solarus", "solarus")
	// setFolders(platforms, "zmachine", "zmachine")
	// setFolders(platforms, "ep64-128", "ep64-128")
	// setFolders(platforms, "sc-3000", "sc-3000")
	// setFolders(platforms, "thomson", "thomson")
	// setFolders(platforms, "tvc", "tvc")

	return platforms
}

func clonePlatformMap(platforms map[string][]string) map[string][]string {
	cloned := make(map[string][]string, len(platforms))
	for slug, folders := range platforms {
		cloned[slug] = append([]string(nil), folders...)
	}
	return cloned
}

func setFolders(platforms map[string][]string, slug string, folders ...string) {
	platforms[slug] = append([]string(nil), folders...)
}