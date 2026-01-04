package utils

import (
	"fmt"
	"grout/cache"
	"grout/constants"
	"grout/internal/imageutil"
	"grout/romm"
	"image/png"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

func GetMissingArtwork(roms []romm.Rom) []romm.Rom {
	var missing []romm.Rom
	for _, rom := range roms {
		if !HasArtworkURL(rom) {
			continue
		}
		if !cache.ArtworkExists(rom.PlatformSlug, rom.ID) {
			missing = append(missing, rom)
		}
	}
	return missing
}

func CheckRemoteLastModified(url string, authHeader string) (time.Time, error) {
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return time.Time{}, err
	}
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	client := &http.Client{Timeout: 10 * constants.DefaultHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return time.Time{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return time.Time{}, fmt.Errorf("bad status: %s", resp.Status)
	}

	lastModified := resp.Header.Get("Last-Modified")
	if lastModified == "" {
		return time.Time{}, nil
	}

	return http.ParseTime(lastModified)
}

func ArtworkNeedsUpdate(rom romm.Rom, host romm.Host) bool {
	cachePath := cache.GetArtworkCachePath(rom.PlatformSlug, rom.ID)

	localInfo, err := os.Stat(cachePath)
	if err != nil {
		return true
	}

	coverPath := GetArtworkCoverPath(rom)
	if coverPath == "" {
		return false
	}

	artURL := host.URL() + coverPath
	artURL = strings.ReplaceAll(artURL, " ", "%20")

	remoteModTime, err := CheckRemoteLastModified(artURL, host.BasicAuthHeader())
	if err != nil || remoteModTime.IsZero() {
		return false // On error or no Last-Modified header, skip re-download
	}

	return remoteModTime.After(localInfo.ModTime())
}

func GetOutdatedArtwork(roms []romm.Rom, host romm.Host) []romm.Rom {
	var outdated []romm.Rom
	for _, rom := range roms {
		if !HasArtworkURL(rom) {
			continue
		}
		if cache.ArtworkExists(rom.PlatformSlug, rom.ID) && ArtworkNeedsUpdate(rom, host) {
			outdated = append(outdated, rom)
		}
	}
	return outdated
}

func HasArtworkURL(rom romm.Rom) bool {
	return rom.PathCoverSmall != "" || rom.PathCoverLarge != "" || rom.URLCover != ""
}

func GetArtworkCoverPath(rom romm.Rom) string {
	if rom.PathCoverSmall != "" {
		return rom.PathCoverSmall
	}
	if rom.PathCoverLarge != "" {
		return rom.PathCoverLarge
	}
	return rom.URLCover
}

func DownloadAndCacheArtwork(rom romm.Rom, host romm.Host) error {
	logger := gaba.GetLogger()

	coverPath := GetArtworkCoverPath(rom)
	if coverPath == "" {
		return nil // No artwork available
	}

	if err := cache.EnsureArtworkCacheDir(rom.PlatformSlug); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	cachePath := cache.GetArtworkCachePath(rom.PlatformSlug, rom.ID)

	artURL := host.URL() + coverPath
	artURL = strings.ReplaceAll(artURL, " ", "%20")

	req, err := http.NewRequest("GET", artURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", host.BasicAuthHeader())

	client := &http.Client{Timeout: constants.DefaultClientTimeout}
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

func SyncArtworkInBackground(host romm.Host, games []romm.Rom) {
	logger := gaba.GetLogger()

	missing := GetMissingArtwork(games)
	if len(missing) == 0 {
		return
	}

	for _, rom := range missing {
		if err := DownloadAndCacheArtwork(rom, host); err != nil {
			logger.Debug("Failed to download artwork", "rom", rom.Name, "error", err)
		}
	}
}
