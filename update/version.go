package update

import (
	"fmt"
	"strconv"
	"strings"
)

type Version struct {
	Major int
	Minor int
	Patch int
}

func ParseVersion(v string) (Version, error) {
	v = strings.TrimPrefix(v, "v")

	parts := strings.SplitN(v, "-", 2)
	v = parts[0]

	segments := strings.Split(v, ".")
	if len(segments) < 1 || len(segments) > 3 {
		return Version{}, fmt.Errorf("invalid version format: %s", v)
	}

	var version Version

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

	return 0
}

func IsNewerVersion(current, latest string) bool {
	return CompareVersions(current, latest) < 0
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}
