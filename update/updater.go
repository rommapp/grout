package update

import (
	"fmt"
	"grout/constants"
	"grout/version"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/atomic"
)

type Info struct {
	CurrentVersion  string
	LatestVersion   string
	ReleaseNotes    string
	DownloadURL     string
	AssetSize       int64
	UpdateAvailable bool
}

func GetAssetName(cfw constants.CFW) string {
	switch cfw {
	case constants.MuOS, constants.Knulli:
		return "grout"
	default:
		return ""
	}
}

func CheckForUpdate(cfw constants.CFW) (*Info, error) {
	currentVersion := version.Get().Version

	if currentVersion == "dev" {
		return &Info{
			CurrentVersion:  currentVersion,
			UpdateAvailable: false,
		}, nil
	}

	release, err := FetchLatestRelease()
	if err != nil {
		return nil, fmt.Errorf("failed to check for updates: %w", err)
	}

	info := &Info{
		CurrentVersion: currentVersion,
		LatestVersion:  release.TagName,
		ReleaseNotes:   release.Body,
	}

	if !IsNewerVersion(currentVersion, release.TagName) {
		info.UpdateAvailable = false
		return info, nil
	}

	assetName := GetAssetName(cfw)
	if assetName == "" {
		return nil, fmt.Errorf("unsupported platform for updates")
	}

	asset := release.FindAsset(assetName)
	if asset == nil {
		return nil, fmt.Errorf("update binary not found for platform: %s", assetName)
	}

	info.UpdateAvailable = true
	info.DownloadURL = asset.BrowserDownloadURL
	info.AssetSize = asset.Size

	return info, nil
}

func PerformUpdate(downloadURL string, progress *atomic.Float64) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}

	tmpPath := execPath + ".new"
	oldPath := execPath + ".old"

	if err := downloadBinary(downloadURL, tmpPath, progress); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to download update: %w", err)
	}

	if err := os.Chmod(tmpPath, 0755); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	os.Remove(oldPath)

	if err := os.Rename(execPath, oldPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to backup current binary: %w", err)
	}

	if err := os.Rename(tmpPath, execPath); err != nil {
		if rollbackErr := os.Rename(oldPath, execPath); rollbackErr != nil {
			return fmt.Errorf("failed to install update and rollback failed: install=%w, rollback=%v", err, rollbackErr)
		}
		return fmt.Errorf("failed to install update (rolled back): %w", err)
	}

	os.Remove(oldPath)

	return nil
}

func downloadBinary(url, destPath string, progress *atomic.Float64) error {
	client := &http.Client{
		Timeout: 5 * time.Minute,
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", "Grout-Updater")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	totalSize := resp.ContentLength
	if totalSize <= 0 {
		totalSize = 1
	}

	var written int64
	buf := make([]byte, 32*1024)

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			nw, writeErr := out.Write(buf[:n])
			if writeErr != nil {
				return writeErr
			}
			written += int64(nw)

			if progress != nil {
				progress.Store(float64(written) / float64(totalSize))
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}

	return nil
}

func FormatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
	)

	switch {
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
