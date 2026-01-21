package update

import (
	"encoding/json"
	"fmt"
	"grout/internal"
	"net/http"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

const (
	repoOwner      = "rommapp"
	repoName       = "grout"
	githubAPIURL   = "https://api.github.com"
	defaultTimeout = 30 * time.Second
)

type GitHubRelease struct {
	TagName    string        `json:"tag_name"`
	Name       string        `json:"name"`
	Body       string        `json:"body"`
	Prerelease bool          `json:"prerelease"`
	Draft      bool          `json:"draft"`
	HTMLURL    string        `json:"html_url"`
	Assets     []GitHubAsset `json:"assets"`
}

type GitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
	ContentType        string `json:"content_type"`
}

func FetchLatestRelease(releaseChannel internal.ReleaseChannel) (*GitHubRelease, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases", githubAPIURL, repoOwner, repoName)

	client := &http.Client{
		Timeout: defaultTimeout,
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Grout-Updater")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("no releases found")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var releases []GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(releases) == 0 {
		return nil, fmt.Errorf("no releases found")
	}

	if releaseChannel == internal.ReleaseChannelBeta {
		gaba.GetLogger().Debug("latest release", "release", releases[0].TagName)
		return &releases[0], nil
	}

	for _, release := range releases {
		if !release.Prerelease && !release.Draft {
			gaba.GetLogger().Debug("latest stable release", "release", release.TagName)
			return &release, nil
		}
	}

	return &releases[0], nil
}

func (r *GitHubRelease) FindAsset(name string) *GitHubAsset {
	for i := range r.Assets {
		if r.Assets[i].Name == name {
			return &r.Assets[i]
		}
	}
	return nil
}

// FetchReleaseForRomMVersion fetches the latest Grout release that matches
// the first 3 semver components (major.minor.patch) of the given RomM version.
// For example, if rommVersion is "4.6.0-alpha.3", this will find Grout releases
// like "4.6.0", "4.6.0.1", "4.6.0-beta.1", etc.
func FetchReleaseForRomMVersion(rommVersion string) (*GitHubRelease, error) {
	rommVer, err := ParseVersion(rommVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to parse RomM version: %w", err)
	}

	url := fmt.Sprintf("%s/repos/%s/%s/releases", githubAPIURL, repoOwner, repoName)

	client := &http.Client{
		Timeout: defaultTimeout,
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Grout-Updater")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("no releases found")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var releases []GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(releases) == 0 {
		return nil, fmt.Errorf("no releases found")
	}

	// Find the latest release that matches the RomM version's major.minor.patch
	for _, release := range releases {
		if release.Draft {
			continue
		}

		releaseVer, err := ParseVersion(release.TagName)
		if err != nil {
			gaba.GetLogger().Debug("skipping release with unparseable version", "tag", release.TagName, "error", err)
			continue
		}

		// Check if major.minor.patch match
		if releaseVer.Major == rommVer.Major &&
			releaseVer.Minor == rommVer.Minor &&
			releaseVer.Patch == rommVer.Patch {
			gaba.GetLogger().Debug("found matching release for RomM version",
				"rommVersion", rommVersion, "release", release.TagName)
			return &release, nil
		}
	}

	return nil, fmt.Errorf("no Grout release found matching RomM version %d.%d.%d",
		rommVer.Major, rommVer.Minor, rommVer.Patch)
}
