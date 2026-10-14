package leaf

import (
	"embed"
	"fmt"
	"grout/tables"
	"os"
	"path/filepath"
	"strings"
)

//go:embed data/*.json
var embeddedFiles embed.FS

//go:embed input_mappings/*.json
var embeddedInputMappings embed.FS

var (
	Platforms       = tables.MustLoad[string, []string](embeddedFiles, "data/platforms.json")
	SaveDirectories = tables.MustLoad[string, []string](embeddedFiles, "data/save_directories.json")
)

// defaultCardPath is the primary card. Leaf runs on top of stock LoongOS, which
// mounts the second slot at /media/sdcard1.
const defaultCardPath = "/mnt/sdcard"

// installDepth is how many directory levels separate the grout binary from the
// card root: <card>/Apps/mlp1/Grout.pak/grout. The release archive itself
// unpacks into Apps/mlp1, which is what the updater counts from.
const installDepth = 4

// GetBasePath returns the root of the card Leaf runs from. launch.sh exports
// BASE_PATH from the SDCARD_PATH Leaf hands every pak; without it (a binary
// started by hand) the same answer is derived from the executable's own path.
func GetBasePath() string {
	if basePath := os.Getenv("BASE_PATH"); basePath != "" {
		return basePath
	}
	if sdcard := os.Getenv("SDCARD_PATH"); sdcard != "" {
		return sdcard
	}
	if root := cardRootFromExecutable(); root != "" {
		return root
	}
	return defaultCardPath
}

// cardRootFromExecutable walks up from the running binary to the card root,
// returning "" when the result does not look like a Leaf card.
func cardRootFromExecutable() string {
	execPath, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(execPath); err == nil {
		execPath = resolved
	}

	root := execPath
	for i := 0; i < installDepth; i++ {
		root = filepath.Dir(root)
	}

	if info, err := os.Stat(filepath.Join(root, "Roms")); err != nil || !info.IsDir() {
		return ""
	}
	return root
}

func GetRomDirectory() string {
	return filepath.Join(GetBasePath(), "Roms")
}

// GetBIOSDirectory is RetroArch's system directory, which Leaf points at the
// card's BIOS folder.
func GetBIOSDirectory() string {
	return filepath.Join(GetBasePath(), "BIOS")
}

// GetBaseSavePath returns the directory RetroArch writes saves to. Leaf forces
// sort_savefiles_enable on, so each core keeps its saves in a folder named
// after it (Saves/mGBA, Saves/gpSP), and the save table lists those folders.
func GetBaseSavePath() string {
	return filepath.Join(GetBasePath(), "Saves")
}

// GetArtDirectory returns where box art goes: a folder under Images named like
// the rom folder. Leaf looks there after the system's canonical Images folder,
// so roms in an alias folder (Roms/FC) still show their art.
func GetArtDirectory(romDir string) string {
	return filepath.Join(GetBasePath(), "Images", filepath.Base(romDir))
}

// GetBIOSFilePaths returns where a BIOS file goes. Most cores read RetroArch's
// system directory as-is; Leaf's Amiga and Saturn packaging, and its Neo Geo
// guide, put theirs elsewhere.
func GetBIOSFilePaths(relativePath, platformFSSlug string) []string {
	biosDir := GetBIOSDirectory()
	name := filepath.Base(relativePath)

	switch platformFSSlug {
	case "amiga":
		// The packaged PUAE cores read kickstarts from BIOS/puae only.
		return []string{filepath.Join(biosDir, "puae", name)}
	case "saturn":
		// YabaSanshiro, in RetroArch and standalone, offers what is in BIOS/SATURN.
		return []string{filepath.Join(biosDir, "SATURN", name), filepath.Join(biosDir, relativePath)}
	}

	// FinalBurn Neo finds neogeo.zip at the root, which is where Leaf tells
	// users to put it.
	if strings.HasPrefix(relativePath, "fbneo/") {
		return []string{filepath.Join(biosDir, name), filepath.Join(biosDir, relativePath)}
	}

	return []string{filepath.Join(biosDir, relativePath)}
}

// GetInputMappingBytes returns the embedded input mapping JSON for the
// Miniloong Pocket 1.
//
// The Loong gamepad presents as a raw SDL joystick numbered in Linux BTN_*
// order, as Leaf's own toolkit reads it: 0=B (south), 1=A (east), 2=X (north),
// 3=Y (west), 4=L1, 5=R1, 6=L2, 7=R2, 8=Select, 9=Start, 10=Menu, d-pad on hat 0.
func GetInputMappingBytes() ([]byte, error) {
	filename := "input_mappings/mlp1.json"

	overridePath := filepath.Join("overrides", "cfw", "leaf", filename)
	data, err := os.ReadFile(overridePath)
	if err != nil {
		data, err = embeddedInputMappings.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("failed to read embedded input mapping %s: %w", filename, err)
		}
	}

	return data, nil
}
