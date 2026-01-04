package utils

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"grout/cfw"
	"grout/constants"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func SaveBIOSFile(biosFile constants.BIOSFile, platformSlug string, data []byte) error {
	filePaths := cfw.GetBIOSFilePaths(biosFile.RelativePath, platformSlug)

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

func VerifyBIOSFileMD5(data []byte, expectedMD5 string) (bool, string) {
	if expectedMD5 == "" {
		// No MD5 hash to verify against
		return true, ""
	}

	hash := md5.Sum(data)
	actualMD5 := hex.EncodeToString(hash[:])

	return actualMD5 == expectedMD5, actualMD5
}

func GetBIOSFileInfo(biosFile constants.BIOSFile, platformSlug string) (exists bool, size int64, md5Hash string, err error) {
	filePaths := cfw.GetBIOSFilePaths(biosFile.RelativePath, platformSlug)

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

func GetBIOSFilesForPlatform(platformSlug string) []constants.BIOSFile {
	var biosFiles []constants.BIOSFile

	coreNames, ok := constants.PlatformToLibretroCores[platformSlug]
	if !ok {
		return biosFiles
	}

	seen := make(map[string]bool)
	for _, coreName := range coreNames {
		normalizedCoreName := strings.TrimSuffix(coreName, "_libretro")
		coreInfo, ok := constants.LibretroCoreToBIOS[normalizedCoreName]
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

type BIOSStatus string

const (
	BIOSStatusMissing        BIOSStatus = "missing"
	BIOSStatusValid          BIOSStatus = "valid"
	BIOSStatusInvalidHash    BIOSStatus = "invalid_hash"
	BIOSStatusNoHashToVerify BIOSStatus = "no_hash"
)

type BIOSFileStatus struct {
	File        constants.BIOSFile
	Status      BIOSStatus
	Exists      bool
	Size        int64
	ActualMD5   string
	ExpectedMD5 string
}

func CheckBIOSFileStatus(biosFile constants.BIOSFile, platformSlug string) BIOSFileStatus {
	status := BIOSFileStatus{
		File:        biosFile,
		ExpectedMD5: biosFile.MD5Hash,
	}

	exists, size, actualMD5, err := GetBIOSFileInfo(biosFile, platformSlug)
	if err != nil {
		status.Status = BIOSStatusMissing
		return status
	}

	status.Exists = exists
	status.Size = size
	status.ActualMD5 = actualMD5

	if !exists {
		status.Status = BIOSStatusMissing
		return status
	}

	if biosFile.MD5Hash == "" {
		status.Status = BIOSStatusNoHashToVerify
		return status
	}

	if actualMD5 == biosFile.MD5Hash {
		status.Status = BIOSStatusValid
	} else {
		status.Status = BIOSStatusInvalidHash
	}

	return status
}
