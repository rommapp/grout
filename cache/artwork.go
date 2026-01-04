package cache

import (
	"image/png"
	"os"
	"path/filepath"
	"strconv"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

func GetArtworkCacheDir() string {
	wd, err := os.Getwd()
	if err != nil {
		return filepath.Join(os.TempDir(), ".cache", "artwork")
	}
	return filepath.Join(wd, ".cache", "artwork")
}

func ClearArtworkCache() error {
	cacheDir := GetArtworkCacheDir()

	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		return nil
	}

	return os.RemoveAll(cacheDir)
}

func HasArtworkCache() bool {
	cacheDir := GetArtworkCacheDir()

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return false
	}

	return len(entries) > 0
}

func GetArtworkCachePath(platformSlug string, romID int) string {
	return filepath.Join(GetArtworkCacheDir(), platformSlug, strconv.Itoa(romID)+".png")
}

func ArtworkExists(platformSlug string, romID int) bool {
	path := GetArtworkCachePath(platformSlug, romID)
	_, err := os.Stat(path)
	return err == nil
}

func EnsureArtworkCacheDir(platformSlug string) error {
	dir := filepath.Join(GetArtworkCacheDir(), platformSlug)
	return os.MkdirAll(dir, 0755)
}

func ValidateArtworkCache() {
	go func() {
		logger := gaba.GetLogger()
		cacheDir := GetArtworkCacheDir()

		platformDirs, err := os.ReadDir(cacheDir)
		if err != nil {
			return
		}

		removed := 0
		for _, platformDir := range platformDirs {
			if !platformDir.IsDir() {
				continue
			}

			platformPath := filepath.Join(cacheDir, platformDir.Name())
			files, err := os.ReadDir(platformPath)
			if err != nil {
				continue
			}

			for _, file := range files {
				if file.IsDir() || filepath.Ext(file.Name()) != ".png" {
					continue
				}

				path := filepath.Join(platformPath, file.Name())
				f, err := os.Open(path)
				if err != nil {
					continue
				}

				_, err = png.DecodeConfig(f)
				f.Close()
				if err != nil {
					os.Remove(path)
					removed++
				}
			}
		}

		if removed > 0 {
			logger.Debug("Removed corrupted artwork files", "count", removed)
		}
	}()
}
