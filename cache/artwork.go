package cache

import (
	"fmt"
	"grout/files"
	"grout/imaging"
	"grout/library"
	"grout/romm"
	"grout/settings"
	"image/png"
	"os"
	"path/filepath"
	"strconv"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

func GetArtworkCachePath(platformFSSlug string, romID int) string {
	return filepath.Join(GetArtworkCacheDir(), platformFSSlug, strconv.Itoa(romID)+".png")
}

func ArtworkExists(platformFSSlug string, romID int) bool {
	return files.FileExists(GetArtworkCachePath(platformFSSlug, romID))
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

func GetArtworkCoverPath(rom romm.Rom, artkind library.ArtKind, host settings.Host) string {
	return rom.GetArtworkURL(artkind, host)
}

func DownloadAndCacheArtwork(rom romm.Rom, kind library.ArtKind, host settings.Host) error {
	artURL := GetArtworkCoverPath(rom, kind, host)
	if artURL == "" {
		return nil // No artwork available
	}

	if err := EnsureArtworkCacheDir(rom.PlatformFSSlug); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	fetcher := romm.NewArtFetcher(host, romm.DefaultClientTimeout)
	fetcher.Process = imaging.ProcessArtImage
	return fetcher.Save(artURL, GetArtworkCachePath(rom.PlatformFSSlug, rom.ID))
}

func SyncArtworkInBackground(artkind library.ArtKind, host settings.Host, games []romm.Rom) {
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
