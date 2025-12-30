package utils

import (
	"fmt"
	"grout/romm"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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

	// Check if cache directory exists
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		return nil // Nothing to clear
	}

	// Remove the entire cache directory
	return os.RemoveAll(cacheDir)
}

// HasArtworkCache returns true if the artwork cache directory exists and has content
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

func GetCachedArtworkForRom(rom romm.Rom) string {
	return GetArtworkCachePath(rom.PlatformSlug, rom.ID)
}

func GetMissingArtwork(roms []romm.Rom) []romm.Rom {
	var missing []romm.Rom
	for _, rom := range roms {
		if !HasArtworkURL(rom) {
			continue
		}
		if !ArtworkExists(rom.PlatformSlug, rom.ID) {
			missing = append(missing, rom)
		}
	}
	return missing
}

// CheckRemoteLastModified performs a HEAD request to get the Last-Modified time for a remote artwork URL
func CheckRemoteLastModified(url string, authHeader string) (time.Time, error) {
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return time.Time{}, err
	}
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	client := &http.Client{Timeout: 10 * time.Second}
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

// ArtworkNeedsUpdate checks if artwork needs to be downloaded based on Last-Modified time
func ArtworkNeedsUpdate(rom romm.Rom, host romm.Host) bool {
	cachePath := GetArtworkCachePath(rom.PlatformSlug, rom.ID)

	// If artwork doesn't exist locally, it needs to be downloaded
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

	// Re-download if remote is newer than local
	return remoteModTime.After(localInfo.ModTime())
}

// GetOutdatedArtwork returns ROMs whose artwork exists but has changed on server
func GetOutdatedArtwork(roms []romm.Rom, host romm.Host) []romm.Rom {
	var outdated []romm.Rom
	for _, rom := range roms {
		if !HasArtworkURL(rom) {
			continue
		}
		if ArtworkExists(rom.PlatformSlug, rom.ID) && ArtworkNeedsUpdate(rom, host) {
			outdated = append(outdated, rom)
		}
	}
	return outdated
}

func HasArtworkURL(rom romm.Rom) bool {
	return rom.PathCoverSmall != "" || rom.PathCoverLarge != "" || rom.URLCover != ""
}

func EnsureArtworkCacheDir(platformSlug string) error {
	dir := filepath.Join(GetArtworkCacheDir(), platformSlug)
	return os.MkdirAll(dir, 0755)
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

	if err := EnsureArtworkCacheDir(rom.PlatformSlug); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	cachePath := GetArtworkCachePath(rom.PlatformSlug, rom.ID)

	artURL := host.URL() + coverPath
	artURL = strings.ReplaceAll(artURL, " ", "%20")

	req, err := http.NewRequest("GET", artURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", host.BasicAuthHeader())

	client := &http.Client{Timeout: 30 * time.Second}
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

	if err := ProcessArtImage(cachePath); err != nil {
		logger.Warn("Failed to process artwork image", "path", cachePath, "error", err)
		os.Remove(cachePath)
		return fmt.Errorf("failed to process artwork: %w", err)
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
