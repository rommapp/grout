package update

import (
	"fmt"
	"grout/cfw"
	"grout/internal"
	"grout/internal/constants"
	"grout/romm"
	"grout/version"
	"io"
	"net/http"
	"os"
	"path/filepath"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
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

func GetAssetName(c cfw.CFW) string {
	switch c {
	case cfw.MuOS, cfw.Knulli, cfw.Spruce, cfw.NextUI:
		return "grout"
	default:
		return ""
	}
}

// CheckForUpdate checks for available updates based on the release channel.
// For ReleaseChannelMatchRomM, the host parameter is required to fetch the RomM version.
// For other channels, the host parameter is optional and ignored.
func CheckForUpdate(c cfw.CFW, releaseChannel internal.ReleaseChannel, host *romm.Host) (*Info, error) {
	currentVersion := version.Get().Version

	if currentVersion == "dev" {
		return &Info{
			CurrentVersion:  currentVersion,
			UpdateAvailable: false,
		}, nil
	}

	var release *GitHubRelease
	var err error

	if releaseChannel == internal.ReleaseChannelMatchRomM {
		if host == nil {
			return nil, fmt.Errorf("host is required for Match RomM release channel")
		}

		// Fetch RomM version from heartbeat
		client := romm.NewClientFromHost(*host)
		heartbeat, err := client.GetHeartbeat()
		if err != nil {
			return nil, fmt.Errorf("failed to get RomM version: %w", err)
		}

		gaba.GetLogger().Debug("fetched RomM version for update check", "version", heartbeat.System.Version)

		// Find a Grout release matching the RomM version
		release, err = FetchReleaseForRomMVersion(heartbeat.System.Version)
		if err != nil {
			return nil, fmt.Errorf("failed to find matching release: %w", err)
		}
	} else {
		release, err = FetchLatestRelease(releaseChannel)
		if err != nil {
			return nil, fmt.Errorf("failed to check for updates: %w", err)
		}
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

	assetName := GetAssetName(c)
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
		Timeout: constants.UpdaterTimeout,
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
