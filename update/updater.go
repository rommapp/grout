package update

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"grout/cfw"
	"grout/romm"
	"grout/settings"
	"grout/version"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"go.uber.org/atomic"
)

type Info struct {
	CurrentVersion  string
	LatestVersion   string
	ReleaseNotes    string
	DownloadURL     string
	AssetSize       int64
	AssetSHA256     string
	UpdateAvailable bool
}

// GetDistributionAssetName returns the release zip for a CFW on this
// architecture, or "" when there is no build for it.
func GetDistributionAssetName(c cfw.CFW) string {
	return cfw.Lookup(c).Packaging().AssetName(runtime.GOARCH)
}

// getInstallRoot returns the top-level install directory where the
// distribution zip contents should be extracted.
func getInstallRoot(c cfw.CFW) (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve executable path: %w", err)
	}

	levels := cfw.Lookup(c).Packaging().InstallDepth
	if levels < 1 {
		return "", fmt.Errorf("no packaging layout known for %s", c)
	}

	root := execPath
	for i := 0; i < levels; i++ {
		root = filepath.Dir(root)
	}
	return root, nil
}

// CheckForUpdate checks for available updates based on the release channel.
// For ReleaseChannelMatchRomM, the host parameter is required to fetch the RomM version.
// For other channels, the host parameter is optional and ignored.
func CheckForUpdate(c cfw.CFW, releaseChannel settings.ReleaseChannel, host *settings.Host) (*Info, error) {
	currentVersion := version.Get().Version

	if currentVersion == "dev" {
		return &Info{
			CurrentVersion:  currentVersion,
			UpdateAvailable: false,
		}, nil
	}

	logger := gaba.GetLogger()

	versions, err := FetchVersionsFile()
	if err != nil {
		return nil, fmt.Errorf("failed to check for updates: %w", err)
	}

	var release *ChannelRelease

	switch releaseChannel {
	case settings.ReleaseChannelMatchRomM:
		if host == nil {
			return nil, fmt.Errorf("host is required for Match RomM release channel")
		}

		client := romm.NewClientFromHost(*host)
		heartbeat, err := client.GetHeartbeat()
		if err != nil {
			return nil, fmt.Errorf("failed to get RomM version: %w", err)
		}

		logger.Debug("fetched RomM version for update check", "version", heartbeat.System.Version)

		rommVer, err := ParseVersion(heartbeat.System.Version)
		if err != nil {
			return nil, fmt.Errorf("failed to parse RomM version: %w", err)
		}

		key := fmt.Sprintf("%d.%d.%d", rommVer.Major, rommVer.Minor, rommVer.Patch)
		release = versions.RomM[key]
		if release == nil {
			return nil, fmt.Errorf("no Grout release found matching RomM version %s", key)
		}

	case settings.ReleaseChannelBeta:
		release = versions.Beta
		if release == nil {
			return nil, fmt.Errorf("no beta release available")
		}

	default:
		release = versions.Stable
		if release == nil {
			return nil, fmt.Errorf("no stable release available")
		}
	}

	return checkRelease(c, release, currentVersion)
}

// checkRelease works out whether a release is worth offering and where to get
// it, given the version already installed.
func checkRelease(c cfw.CFW, release *ChannelRelease, currentVersion string) (*Info, error) {
	info := &Info{
		CurrentVersion: currentVersion,
		LatestVersion:  release.Version,
		ReleaseNotes:   release.Notes,
	}

	if !IsNewerVersion(currentVersion, release.Version) {
		info.UpdateAvailable = false
		return info, nil
	}

	assetName := GetDistributionAssetName(c)
	if assetName == "" {
		return nil, fmt.Errorf("unsupported platform for updates")
	}

	asset, ok := release.Assets[assetName]
	if !ok {
		return nil, fmt.Errorf("update not found for platform: %s", assetName)
	}

	// The checksum is what pins the download, and it arrives in the same
	// document that says where to download from. Treating a missing one as
	// nothing to check would let that document waive its own verification, so
	// an asset without one is not offered at all.
	if asset.SHA256 == "" {
		return nil, fmt.Errorf("no checksum published for %s", assetName)
	}

	info.UpdateAvailable = true
	info.DownloadURL = asset.URL
	info.AssetSize = asset.Size
	info.AssetSHA256 = asset.SHA256

	return info, nil
}

func PerformUpdate(c cfw.CFW, downloadURL string, expectedSize int64, expectedSHA256 string, progress *atomic.Float64) error {
	// Nothing else checks the download: the expected size only drives the
	// progress bar. Without a checksum the zip would be unpacked over the
	// install on trust alone, so this is settled before spending a download on
	// it.
	if expectedSHA256 == "" {
		return fmt.Errorf("refusing to install an update with no checksum")
	}

	installRoot, err := getInstallRoot(c)
	if err != nil {
		return err
	}

	tmpZip := filepath.Join(os.TempDir(), "grout-update.zip")
	defer os.Remove(tmpZip)

	if err := downloadFile(downloadURL, tmpZip, expectedSize, progress); err != nil {
		return fmt.Errorf("failed to download update: %w", err)
	}

	if err := verifySHA256(tmpZip, expectedSHA256); err != nil {
		return err
	}

	// Extract the full zip to a staging directory at the install root.
	// The launch script will copy everything over on next startup,
	// avoiding issues with overwriting running files.
	updateDir := filepath.Join(installRoot, ".update")
	os.RemoveAll(updateDir)

	if err := extractZip(tmpZip, updateDir); err != nil {
		os.RemoveAll(updateDir)
		return fmt.Errorf("failed to extract update: %w", err)
	}

	// Immediately replace the launch script so the new version
	// (with the update preamble) is in place for next startup.
	scriptRel := getLaunchScriptPath(c)
	stagedScript := filepath.Join(updateDir, scriptRel)
	targetScript := filepath.Join(installRoot, scriptRel)

	if data, err := os.ReadFile(stagedScript); err == nil {
		os.Remove(targetScript)
		if err := os.WriteFile(targetScript, data, 0755); err != nil {
			return fmt.Errorf("failed to update launch script: %w", err)
		}
		os.Remove(stagedScript)
	}

	return nil
}

func getLaunchScriptPath(c cfw.CFW) string {
	return cfw.Lookup(c).Packaging().LaunchScript
}

func extractZip(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		targetPath := filepath.Join(destDir, f.Name)

		// Prevent zip slip
		cleanDest := filepath.Clean(destDir) + string(os.PathSeparator)
		if !strings.HasPrefix(targetPath, cleanDest) && targetPath != filepath.Clean(destDir) {
			return fmt.Errorf("illegal file path in zip: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(targetPath, 0755)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}

		outFile, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return fmt.Errorf("failed to create file %s: %w", f.Name, err)
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return fmt.Errorf("failed to read zip entry %s: %w", f.Name, err)
		}

		_, err = io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()
		if err != nil {
			return fmt.Errorf("failed to write file %s: %w", f.Name, err)
		}
	}

	return nil
}

// CleanupUpdateArtifacts removes any leftover files from a previous update.
func CleanupUpdateArtifacts() {
	os.Remove(filepath.Join(os.TempDir(), "grout-update.zip"))
}

func verifySHA256(filePath, expected string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file for verification: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("failed to hash file: %w", err)
	}

	actual := hex.EncodeToString(h.Sum(nil))
	if actual != expected {
		return fmt.Errorf("download did not match expected hash")
	}

	return nil
}

func downloadFile(url, destPath string, expectedSize int64, progress *atomic.Float64) error {
	client := &http.Client{
		Timeout: settings.UpdaterTimeout,
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

	// Use the known asset size for progress tracking; fall back to Content-Length
	totalSize := expectedSize
	if totalSize <= 0 {
		totalSize = resp.ContentLength
	}
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
