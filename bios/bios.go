package bios

import (
	"embed"
	"fmt"
	"grout/cfw"
	"grout/internal/jsonutil"
	"os"
	"path/filepath"
	"strings"
)

//go:embed data
var embeddedFiles embed.FS

func mustLoadJSONMap[K comparable, V any](path string) map[K]V {
	return jsonutil.MustLoadJSONMap[K, V](embeddedFiles, path)
}

var LibretroCoreToBIOS = mustLoadJSONMap[string, CoreBIOS]("data/core_requirements.json")
var PlatformToLibretroCores = mustLoadJSONMap[string, []string]("data/platform_cores.json")

// File represents a single BIOS/firmware file requirement
type File struct {
	FileName     string // e.g., "gba_bios.bin"
	RelativePath string // e.g., "gba_bios.bin" or "psx/scph5500.bin"
	Optional     bool   // true if BIOS file is optional for the emulator to function
}

// CoreBIOS represents all BIOS requirements for a Libretro core
type CoreBIOS struct {
	CoreName    string // e.g., "mgba_libretro"
	DisplayName string // e.g., "Nintendo - Game Boy Advance (mGBA)"
	Files       []File // List of BIOS files for this core
}

func SaveFile(biosFile File, platformFSSlug string, data []byte) error {
	filePaths := cfw.GetBIOSFilePaths(biosFile.RelativePath, platformFSSlug)

	for _, filePath := range filePaths {
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}

		if err := os.WriteFile(filePath, data, 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", filePath, err)
		}
	}

	return nil
}

// FileExists checks if a BIOS file exists on the filesystem for the given platform.
func FileExists(biosFile File, platformFSSlug string) bool {
	filePaths := cfw.GetBIOSFilePaths(biosFile.RelativePath, platformFSSlug)
	for _, filePath := range filePaths {
		if _, err := os.Stat(filePath); err == nil {
			return true
		}
	}
	return false
}

func GetFilesForPlatform(platformFSSlug string) []File {
	var biosFiles []File

	coreNames, ok := PlatformToLibretroCores[platformFSSlug]
	if !ok {
		return biosFiles
	}

	seen := make(map[string]bool)
	for _, coreName := range coreNames {
		normalizedCoreName := strings.TrimSuffix(coreName, "_libretro")
		coreInfo, ok := LibretroCoreToBIOS[normalizedCoreName]
		if !ok {
			continue
		}

		for _, file := range coreInfo.Files {
			if !seen[file.FileName] {
				biosFiles = append(biosFiles, file)
				seen[file.FileName] = true
			}
		}
	}

	return biosFiles
}
