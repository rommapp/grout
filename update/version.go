package update

import (
	"fmt"
	"strconv"
	"strings"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

type Version struct {
	Major      int
	Minor      int
	Patch      int
	Build      int    // Fourth component for Grout-specific releases
	Prerelease string // e.g., "beta.1" from "4.6.0.0-beta.1"
}

func ParseVersion(v string) (Version, error) {
	v = strings.TrimPrefix(v, "v")

	parts := strings.SplitN(v, "-", 2)
	versionStr := parts[0]
	var prerelease string
	if len(parts) > 1 {
		prerelease = parts[1]
	}

	segments := strings.Split(versionStr, ".")
	if len(segments) < 1 || len(segments) > 4 {
		return Version{}, fmt.Errorf("invalid version format: %s", v)
	}

	var version Version
	version.Prerelease = prerelease

	if len(segments) >= 1 {
		major, err := strconv.Atoi(segments[0])
		if err != nil {
			return Version{}, fmt.Errorf("invalid major version: %s", segments[0])
		}
		version.Major = major
	}

	if len(segments) >= 2 {
		minor, err := strconv.Atoi(segments[1])
		if err != nil {
			return Version{}, fmt.Errorf("invalid minor version: %s", segments[1])
		}
		version.Minor = minor
	}

	if len(segments) >= 3 {
		patch, err := strconv.Atoi(segments[2])
		if err != nil {
			return Version{}, fmt.Errorf("invalid patch version: %s", segments[2])
		}
		version.Patch = patch
	}

	if len(segments) >= 4 {
		build, err := strconv.Atoi(segments[3])
		if err != nil {
			return Version{}, fmt.Errorf("invalid build version: %s", segments[3])
		}
		version.Build = build
	}

	return version, nil
}

func CompareVersions(current, latest string) int {
	currentVer, err := ParseVersion(current)
	if err != nil {
		gaba.GetLogger().Warn("Failed to parse current version", "version", current, "error", err)
		return 0
	}

	latestVer, err := ParseVersion(latest)
	if err != nil {
		gaba.GetLogger().Warn("Failed to parse latest version", "version", latest, "error", err)
		return 0
	}

	if currentVer.Major < latestVer.Major {
		return -1
	}
	if currentVer.Major > latestVer.Major {
		return 1
	}

	if currentVer.Minor < latestVer.Minor {
		return -1
	}
	if currentVer.Minor > latestVer.Minor {
		return 1
	}

	if currentVer.Patch < latestVer.Patch {
		return -1
	}
	if currentVer.Patch > latestVer.Patch {
		return 1
	}

	if currentVer.Build < latestVer.Build {
		return -1
	}
	if currentVer.Build > latestVer.Build {
		return 1
	}

	// If numeric versions are equal, compare prerelease status
	// According to semver: a version without a prerelease is newer than one with a prerelease
	currentHasPrerelease := currentVer.Prerelease != ""
	latestHasPrerelease := latestVer.Prerelease != ""

	if !currentHasPrerelease && latestHasPrerelease {
		// Current is a full release, latest is prerelease - current is newer
		return 1
	}
	if currentHasPrerelease && !latestHasPrerelease {
		// Current is prerelease, latest is full release - latest is newer
		return -1
	}
	if currentHasPrerelease {
		// Both are prereleases - compare identifiers per semver:
		// split on ".", compare each segment numerically if both are numbers,
		// otherwise lexicographically.
		currentParts := strings.Split(currentVer.Prerelease, ".")
		latestParts := strings.Split(latestVer.Prerelease, ".")
		maxLen := len(currentParts)
		if len(latestParts) > maxLen {
			maxLen = len(latestParts)
		}
		for i := 0; i < maxLen; i++ {
			if i >= len(currentParts) {
				return -1 // current has fewer identifiers, so it's less
			}
			if i >= len(latestParts) {
				return 1 // latest has fewer identifiers, so current is greater
			}
			cNum, cErr := strconv.Atoi(currentParts[i])
			lNum, lErr := strconv.Atoi(latestParts[i])
			if cErr == nil && lErr == nil {
				// Both numeric: compare as integers
				if cNum < lNum {
					return -1
				}
				if cNum > lNum {
					return 1
				}
			} else {
				// At least one non-numeric: compare as strings
				if currentParts[i] < latestParts[i] {
					return -1
				}
				if currentParts[i] > latestParts[i] {
					return 1
				}
			}
		}
	}

	return 0
}

func IsNewerVersion(current, latest string) bool {
	return CompareVersions(current, latest) < 0
}

func (v Version) String() string {
	base := fmt.Sprintf("%d.%d.%d.%d", v.Major, v.Minor, v.Patch, v.Build)
	if v.Prerelease != "" {
		return base + "-" + v.Prerelease
	}
	return base
}
