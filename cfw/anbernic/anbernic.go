package anbernic

import (
	"embed"
	"fmt"
	"grout/internal/jsonutil"
	"os"
	"path/filepath"
)

//go:embed data/*.json
var embeddedFiles embed.FS

//go:embed input_mappings/*.json
var embeddedInputMappings embed.FS

var (
	Platforms = jsonutil.MustLoadJSONMap[string, []string](embeddedFiles, "data/platforms.json")
)

// biosDirectory is RetroArch's system directory on the stock firmware. It lives on the appfs
// partition, which the launcher mounts read-write at boot (/etc/init.d/launcher.sh).
const biosDirectory = "/mnt/vendor/deep/retro/system"

// cardPaths are the mount points the stock firmware uses for the two card slots: TF1 holds the
// OS and is always mounted at /mnt/mmc, TF2 is mounted at /mnt/sdcard when a second card is
// inserted (/mnt/vendor/ctrl/mmc_new.sh).
var cardPaths = []string{"/mnt/mmc", "/mnt/sdcard"}

// GetBasePath returns the root of the card Grout manages. The launch script exports BASE_PATH
// derived from its own location, so a copy installed on TF2 manages TF2's ROMs.
func GetBasePath() string {
	if basePath := os.Getenv("BASE_PATH"); basePath != "" {
		return basePath
	}
	for _, path := range cardPaths {
		if info, err := os.Stat(filepath.Join(path, "Roms")); err == nil && info.IsDir() {
			return path
		}
	}
	return cardPaths[0]
}

func GetRomDirectory() string {
	return filepath.Join(GetBasePath(), "Roms")
}

func GetBIOSDirectory() string {
	return biosDirectory
}

// GetBaseSavePath returns the directory RetroArch writes saves to. The stock firmware ships a
// patched RetroArch that, for content under a card mount point, rewrites the save path to
// <card>/saves_RA/<ROM path relative to the card>, so a ROM in Roms/GBA saves to
// saves_RA/Roms/GBA. Save states follow the same scheme under states_RA.
func GetBaseSavePath() string {
	return filepath.Join(GetBasePath(), "saves_RA", "Roms")
}

func GetArtDirectory(romDir string) string {
	return filepath.Join(romDir, "Imgs")
}

// GetInputMappingBytes returns the embedded input mapping JSON for the Anbernic stock OS.
//
// The pad presents as a raw SDL joystick named ANBERNIC-keys, with the same button order as
// RetroArch's ANBERNIC-keys autoconfig on the device: 0=A, 1=B, 2=Y, 3=X, 4=L1, 5=R1, 6=Select,
// 7=Start, 8=Menu, 9=L2, 10=R2, d-pad on hat 0. This is the same hardware muOS runs on, but muOS
// enumerates the same buttons starting at index 3, so its mapping cannot be reused as-is.
func GetInputMappingBytes() ([]byte, error) {
	filename := "input_mappings/anbernic.json"

	overridePath := filepath.Join("overrides", "cfw", "anbernic", filename)
	data, err := os.ReadFile(overridePath)
	if err != nil {
		data, err = embeddedInputMappings.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("failed to read embedded input mapping %s: %w", filename, err)
		}
	}

	return data, nil
}
