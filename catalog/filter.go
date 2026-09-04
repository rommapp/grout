package catalog

import (
	"slices"

	"grout/romm"
	"grout/settings"
)

// MappingStatus narrows platforms by whether a rom folder has been chosen for
// them yet.
type MappingStatus string

const (
	StatusAll      MappingStatus = "all"
	StatusMapped   MappingStatus = "mapped"
	StatusUnmapped MappingStatus = "unmapped"
)

// PlatformFilter narrows the platform list on the mapping screen.
//
// The zero value accepts everything, so an unset filter shows the whole
// library.
type PlatformFilter struct {
	WithGamesOnly bool
	Status        MappingStatus
	// Category and Family are IGDB metadata RomM often leaves blank. An empty
	// value or StatusAll's "all" accepts any.
	Category   string
	Family     string
	Generation int
}

// Match reports whether a platform survives the filter. mapped says whether the
// platform already has a rom folder chosen.
func (f PlatformFilter) Match(p romm.Platform, mapped bool) bool {
	if f.WithGamesOnly && p.ROMCount == 0 {
		return false
	}
	if f.Generation != 0 && p.Generation != f.Generation {
		return false
	}
	if !matchesValue(f.Category, p.Category) {
		return false
	}
	if !matchesValue(f.Family, p.Family) {
		return false
	}
	switch f.Status {
	case StatusMapped:
		return mapped
	case StatusUnmapped:
		return !mapped
	}
	return true
}

func matchesValue(want, have string) bool {
	return want == "" || want == string(StatusAll) || want == have
}

// FilterPlatforms keeps the platforms the filter accepts, in their original
// order.
func FilterPlatforms(platforms []romm.Platform, filter PlatformFilter, mappings map[string]settings.DirectoryMapping) []romm.Platform {
	var kept []romm.Platform
	for _, p := range platforms {
		mapping, ok := mappings[p.FSSlug]
		if filter.Match(p, ok && mapping.RelativePath != "") {
			kept = append(kept, p)
		}
	}
	return kept
}

// Categories lists the distinct categories present, sorted.
//
// An empty result means the filter has nothing to offer and should be hidden
// rather than shown as an "All"-only picker.
func Categories(platforms []romm.Platform) []string {
	return distinct(platforms, func(p romm.Platform) string { return p.Category })
}

// Families lists the distinct hardware families present, sorted.
func Families(platforms []romm.Platform) []string {
	return distinct(platforms, func(p romm.Platform) string { return p.Family })
}

// Generations lists the distinct console generations present, sorted. Zero
// means RomM did not say, and is left out.
func Generations(platforms []romm.Platform) []int {
	seen := make(map[int]bool)
	for _, p := range platforms {
		if p.Generation > 0 {
			seen[p.Generation] = true
		}
	}
	values := make([]int, 0, len(seen))
	for v := range seen {
		values = append(values, v)
	}
	slices.Sort(values)
	return values
}

func distinct(platforms []romm.Platform, get func(romm.Platform) string) []string {
	seen := make(map[string]bool)
	for _, p := range platforms {
		if v := get(p); v != "" {
			seen[v] = true
		}
	}
	values := make([]string, 0, len(seen))
	for v := range seen {
		values = append(values, v)
	}
	slices.Sort(values)
	return values
}
