package internal

import (
	"fmt"
	"grout/cache"
	"grout/romm"
	"time"
)

// SortPlatformsByOrder sorts platforms based on the saved order.
// If no order is saved, platforms are sorted alphabetically.
func SortPlatformsByOrder(platforms []romm.Platform, order []string) []romm.Platform {
	if len(order) == 0 {
		return SortPlatformsAlphabetically(platforms)
	}

	platformMap := make(map[string]romm.Platform)
	for _, p := range platforms {
		platformMap[p.FSSlug] = p
	}

	var result []romm.Platform
	usedSlugs := make(map[string]bool)

	for _, fsSlug := range order {
		if platform, exists := platformMap[fsSlug]; exists {
			result = append(result, platform)
			usedSlugs[fsSlug] = true
		}
	}

	var newPlatforms []romm.Platform
	for _, p := range platforms {
		if !usedSlugs[p.FSSlug] {
			newPlatforms = append(newPlatforms, p)
		}
	}
	newPlatforms = SortPlatformsAlphabetically(newPlatforms)
	result = append(result, newPlatforms...)

	return result
}

// SortPlatformsAlphabetically sorts platforms by name
func SortPlatformsAlphabetically(platforms []romm.Platform) []romm.Platform {
	sorted := make([]romm.Platform, len(platforms))
	copy(sorted, platforms)

	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i].Name > sorted[j].Name {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted
}

func PrunePlatformOrder(order []string, mappings map[string]DirectoryMapping) []string {
	if len(order) == 0 {
		return order
	}

	pruned := make([]string, 0, len(order))
	for _, fsSlug := range order {
		if _, exists := mappings[fsSlug]; exists {
			pruned = append(pruned, fsSlug)
		}
	}

	return pruned
}

func GetMappedPlatforms(host romm.Host, mappings map[string]DirectoryMapping, timeout ...time.Duration) ([]romm.Platform, error) {
	var rommPlatforms []romm.Platform
	var err error

	if cm := cache.GetCacheManager(); cm != nil {
		rommPlatforms, err = cm.GetPlatforms()
	}
	if len(rommPlatforms) == 0 {
		c := romm.NewClientFromHost(host, timeout...)
		rommPlatforms, err = c.GetPlatforms()
		if err != nil {
			return nil, fmt.Errorf("failed to get platforms from RomM: %w", err)
		}
	}

	romm.DisambiguatePlatformNames(rommPlatforms)

	var platforms []romm.Platform

	for _, platform := range rommPlatforms {
		_, exists := mappings[platform.FSSlug]
		if exists {
			platforms = append(platforms, platform)
		}
	}

	return platforms, nil
}
