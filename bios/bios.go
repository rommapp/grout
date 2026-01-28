package bios

import (
	"crypto/md5"
	"embed"
	"encoding/hex"
	"fmt"
	"grout/cfw"
	"grout/internal/jsonutil"
	"io"
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
	MD5Hash      string // e.g., "a860e8c0b6d573d191e4ec7db1b1e4f6" (optional, empty string if unknown)
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

func VerifyFileMD5(data []byte, expectedMD5 string) (bool, string) {
	if expectedMD5 == "" {
		// No MD5 hash to verify against
		return true, ""
	}

	hash := md5.Sum(data)
	actualMD5 := hex.EncodeToString(hash[:])

	return actualMD5 == expectedMD5, actualMD5
}

func GetFileInfo(biosFile File, platformFSSlug string) (exists bool, size int64, md5Hash string, err error) {
	filePaths := cfw.GetBIOSFilePaths(biosFile.RelativePath, platformFSSlug)

	for _, filePath := range filePaths {
		info, err := os.Stat(filePath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return false, 0, "", err
		}

		file, err := os.Open(filePath)
		if err != nil {
			return true, info.Size(), "", err
		}
		defer file.Close()

		hash := md5.New()
		if _, err := io.Copy(hash, file); err != nil {
			return true, info.Size(), "", err
		}

		md5Hash = hex.EncodeToString(hash.Sum(nil))

		return true, info.Size(), md5Hash, nil
	}

	return false, 0, "", nil
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

type Status string

const (
	StatusMissing        Status = "missing"
	StatusValid          Status = "valid"
	StatusInvalidHash    Status = "invalid_hash"
	StatusNoHashToVerify Status = "no_hash"
)

type FileStatus struct {
	File        File
	Status      Status
	Exists      bool
	Size        int64
	ActualMD5   string
	ExpectedMD5 string
}

func CheckFileStatus(biosFile File, platformFSSlug string) FileStatus {
	status := FileStatus{
		File:        biosFile,
		ExpectedMD5: biosFile.MD5Hash,
	}

	exists, size, actualMD5, err := GetFileInfo(biosFile, platformFSSlug)
	if err != nil {
		status.Status = StatusMissing
		return status
	}

	status.Exists = exists
	status.Size = size
	status.ActualMD5 = actualMD5

	if !exists {
		status.Status = StatusMissing
		return status
	}

	if biosFile.MD5Hash == "" {
		status.Status = StatusNoHashToVerify
		return status
	}

	if actualMD5 == biosFile.MD5Hash {
		status.Status = StatusValid
	} else {
		status.Status = StatusInvalidHash
	}

	return status
}
