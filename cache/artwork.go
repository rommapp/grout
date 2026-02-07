package cache

import (
	"fmt"
	"grout/internal/artutil"
	"grout/internal/fileutil"
	"grout/internal/imageutil"
	"grout/romm"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

func GetArtworkCachePath(platformFSSlug string, romID int) string {
	return filepath.Join(GetArtworkCacheDir(), platformFSSlug, strconv.Itoa(romID)+".png")
}

func ArtworkExists(platformFSSlug string, romID int) bool {
	return fileutil.FileExists(GetArtworkCachePath(platformFSSlug, romID))
}

func EnsureArtworkCacheDir(platformFSSlug string) error {
	dir := filepath.Join(GetArtworkCacheDir(), platformFSSlug)
	return os.MkdirAll(dir, 0755)
}

func (cm *Manager) ValidateArtworkCache() (int, error) {
	logger := gaba.GetLogger()
	cacheDir := GetArtworkCacheDir()
	removed := 0

	platformDirs, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

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

			filePath := filepath.Join(platformPath, file.Name())
			if !isValidPNG(filePath) {
				os.Remove(filePath)
				removed++
			}
		}
	}

	if removed > 0 {
		logger.Debug("Removed invalid artwork files", "count", removed)
	}

	return removed, nil
}

func RunArtworkValidation() {
	if cm := GetCacheManager(); cm != nil {
		go func() {
			removed, err := cm.ValidateArtworkCache()
			if err != nil {
				gaba.GetLogger().Debug("Failed to validate artwork cache", "error", err)
				return
			}
			if removed > 0 {
				gaba.GetLogger().Debug("Removed invalid artwork files", "count", removed)
			}
		}()
	}
}

func isValidPNG(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	_, err = png.DecodeConfig(f)
	return err == nil
}

func GetMissingArtwork(roms []romm.Rom) []romm.Rom {
	var missing []romm.Rom
	for _, rom := range roms {
		if !HasArtworkURL(rom) {
			continue
		}
		if !ArtworkExists(rom.PlatformFSSlug, rom.ID) {
			missing = append(missing, rom)
		}
	}
	return missing
}

func HasArtworkURL(rom romm.Rom) bool {
	return rom.PathCoverSmall != "" || rom.PathCoverLarge != "" || rom.URLCover != ""
}

func GetArtworkCoverPath(rom romm.Rom, artkind artutil.ArtKind, host romm.Host) string {
	return rom.GetArtworkURL(artkind, host)
}

func DownloadAndCacheArtwork(rom romm.Rom, kind artutil.ArtKind, host romm.Host) error {
	logger := gaba.GetLogger()

	artURL := GetArtworkCoverPath(rom, kind, host)
	if artURL == "" {
		return nil // No artwork available
	}

	if err := EnsureArtworkCacheDir(rom.PlatformFSSlug); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	cachePath := GetArtworkCachePath(rom.PlatformFSSlug, rom.ID)

	req, err := http.NewRequest("GET", artURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", host.BasicAuthHeader())

	client := &http.Client{Timeout: romm.DefaultClientTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download artwork: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	outFile, err := os.Create(cachePath)
	if err != nil {
		return fmt.Errorf("failed to create cache file: %w", err)
	}
	defer outFile.Close()

	if _, err = io.Copy(outFile, resp.Body); err != nil {
		os.Remove(cachePath)
		return fmt.Errorf("failed to write cache file: %w", err)
	}
	outFile.Close()

	if err := imageutil.ProcessArtImage(cachePath); err != nil {
		logger.Warn("Failed to process artwork image", "path", cachePath, "error", err)
		os.Remove(cachePath)
		return fmt.Errorf("failed to process artwork: %w", err)
	}

	file, err := os.Open(cachePath)
	if err != nil {
		return fmt.Errorf("failed to open processed artwork: %w", err)
	}
	_, err = png.DecodeConfig(file)
	file.Close()
	if err != nil {
		os.Remove(cachePath)
		return fmt.Errorf("processed artwork is not a valid PNG: %w", err)
	}

	return nil
}

func SyncArtworkInBackground(artkind artutil.ArtKind, host romm.Host, games []romm.Rom) {
	logger := gaba.GetLogger()

	missing := GetMissingArtwork(games)
	if len(missing) == 0 {
		return
	}

	for _, rom := range missing {
		if err := DownloadAndCacheArtwork(rom, artkind, host); err != nil {
			logger.Debug("Failed to download artwork", "rom", rom.Name, "error", err)
		}
	}
}
