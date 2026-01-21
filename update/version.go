package update

import (
	"fmt"
	"strconv"
	"strings"
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
		return 0
	}

	latestVer, err := ParseVersion(latest)
	if err != nil {
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
		// Both are prereleases - compare prerelease strings lexicographically
		// For simplicity, we'll just do a string comparison
		// In practice, this handles cases like "beta.1" vs "beta.2"
		if currentVer.Prerelease < latestVer.Prerelease {
			return -1
		}
		if currentVer.Prerelease > latestVer.Prerelease {
			return 1
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
