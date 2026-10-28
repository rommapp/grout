package muos

import (
	"embed"
	"grout/settings"
	"grout/tables"
	"os"
	"path/filepath"
	"strings"
)

//go:embed data/*.json
var embeddedFiles embed.FS

const (
	StoragePath     = "/run/muos/storage"
	RomsFolderUnion = "/mnt/union/ROMS"
)

var (
	Platforms       = tables.MustLoad[string, []string](embeddedFiles, "data/platforms.json")
	SaveDirectories = tables.MustLoad[string, []string](embeddedFiles, "data/save_directories.json")
	ArtDirectories  = tables.MustLoad[string, string](embeddedFiles, "data/art_directories.json")
)

func GetBasePath() string {
	if basePath := os.Getenv("BASE_PATH"); basePath != "" {
		return filepath.Join(basePath, "MUOS")
	}
	return StoragePath
}

func GetRomDirectory() string {
	if basePath := os.Getenv("BASE_PATH"); basePath != "" {
		return filepath.Join(basePath, "ROMS")
	}
	return romDirectory("/", settings.RomStorage(os.Getenv(settings.RomStorageEnvVar)))
}

// UsesUnion reports an older muOS that merges every card into one library,
// where there is no card to choose.
func UsesUnion() bool {
	return os.Getenv("BASE_PATH") == "" && isDir(RomsFolderUnion)
}

// storageConfigDir is where muOS keeps each storage's mount point.
const storageConfigDir = "/opt/muos/device/config/storage"

// unionOrder is the order the union mount took writes in, and so where a
// library is looked for now that there is no union.
var unionOrder = []struct {
	storage  settings.RomStorage
	fallback string
}{
	{settings.RomStorageUSB, "/mnt/usb"},
	{settings.RomStorageSD2, "/mnt/sdcard"},
	{settings.RomStorageSD1, "/mnt/mmc"},
}

// romDirectory finds the library under root. Older muOS merges every card at
// /mnt/union; newer muOS dropped that in March 2026 and mounts each card on
// its own, so the chosen card is used if it has a ROMS folder and the union
// order otherwise.
func romDirectory(root string, preferred settings.RomStorage) string {
	if isDir(filepath.Join(root, RomsFolderUnion)) {
		return filepath.Join(root, RomsFolderUnion)
	}

	candidates := make([]string, 0, len(unionOrder)+1)
	for _, entry := range unionOrder {
		roms := filepath.Join(root, mountPoint(root, entry.storage, entry.fallback), "ROMS")
		if entry.storage == preferred {
			candidates = append([]string{roms}, candidates...)
		} else {
			candidates = append(candidates, roms)
		}
	}

	for _, roms := range candidates {
		if isDir(roms) {
			return roms
		}
	}
	return filepath.Join(root, mountPoint(root, settings.RomStorageSD1, "/mnt/mmc"), "ROMS")
}

// mountPoint reads where muOS mounts a storage, falling back to its default.
func mountPoint(root string, storage settings.RomStorage, fallback string) string {
	data, err := os.ReadFile(filepath.Join(root, storageConfigDir, string(storage), "mount"))
	if err != nil {
		return fallback
	}
	if mount := strings.TrimSpace(string(data)); mount != "" {
		return mount
	}
	return fallback
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func GetBIOSDirectory() string {
	return filepath.Join(GetBasePath(), "bios")
}

func GetInfoDirectory() string {
	return filepath.Join(GetBasePath(), "info")
}

func GetBaseSavePath() string {
	return filepath.Join(GetBasePath(), "save")
}

func GetArtDirectory(platformFSSlug, platformName string) string {
	systemName, exists := ArtDirectories[platformFSSlug]
	if !exists {
		systemName = platformName
	}
	return filepath.Join(GetInfoDirectory(), "catalogue", systemName, "box")
}

func GetTextDirectory(platformFSSlug, platformName string) string {
	systemName, exists := ArtDirectories[platformFSSlug]
	if !exists {
		systemName = platformName
	}
	return filepath.Join(GetInfoDirectory(), "catalogue", systemName, "text")
}

func GetPreviewDirectory(platformFSSlug, platformName string) string {
	systemName, exists := ArtDirectories[platformFSSlug]
	if !exists {
		systemName = platformName
	}
	return filepath.Join(GetInfoDirectory(), "catalogue", systemName, "preview")
}

func GetSplashDirectory(platformFSSlug, platformName string) string {
	systemName, exists := ArtDirectories[platformFSSlug]
	if !exists {
		systemName = platformName
	}
	return filepath.Join(GetInfoDirectory(), "catalogue", systemName, "splash")
}

// EmulatorLabel is what to call a save folder on screen.
//
// muOS stores saves under paths like "file/PPSSPP/backup", where the parts
// around the emulator's name are storage layout rather than anything the user
// chose between.
func EmulatorLabel(dir string) string {
	// Pickles keeps saves per core under the rom folder's name, so the core is
	// the part worth showing.
	if core, ok := strings.CutPrefix(dir, picklesSaveRoot+"/"); ok {
		core, _, _ = strings.Cut(core, "/")
		return "Pickles (" + core + ")"
	}
	trimmed := strings.ReplaceAll(dir, "file/", "")
	trimmed = strings.ReplaceAll(trimmed, "/backup", "")
	return filepath.Base(trimmed)
}
