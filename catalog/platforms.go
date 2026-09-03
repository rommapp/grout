// Package library answers questions about the game library that need both the
// local cache and the RomM server, and so belong above either.
package catalog

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"grout/cache"
	"grout/romm"
	"grout/settings"
)

// MappedPlatforms returns the platforms that have a directory mapping and at
// least one rom, preferring the cache and falling back to the server.
func MappedPlatforms(host settings.Host, mappings map[string]settings.DirectoryMapping, timeout ...time.Duration) ([]romm.Platform, error) {
	var platforms []romm.Platform
	var err error

	if cm := cache.GetCacheManager(); cm != nil {
		platforms, err = cm.GetPlatforms()
	}
	if len(platforms) == 0 {
		platforms, err = romm.NewClientFromHost(host, timeout...).GetPlatforms()
		if err != nil {
			return nil, fmt.Errorf("failed to get platforms from RomM: %w", err)
		}
	}

	romm.DisambiguatePlatformNames(platforms)

	var mapped []romm.Platform
	for _, platform := range platforms {
		if _, ok := mappings[platform.FSSlug]; ok && platform.ROMCount > 0 {
			mapped = append(mapped, platform)
		}
	}
	return mapped, nil
}

// ShowCollections reports whether the collections menu has anything to show.
//
// It checks the cache before the server so a device that is offline, or one
// whose server has no collections, does not pay for a network round trip on
// every navigation.
func ShowCollections(config settings.Config, host settings.Host) bool {
	if !config.ShowRegularCollections && !config.ShowSmartCollections && !config.ShowVirtualCollections {
		return false
	}

	if cm := cache.GetCacheManager(); cm != nil && cm.HasCollections() {
		return true
	}

	client := romm.NewClientFromHost(host, config.ApiTimeout.Duration())

	if config.ShowRegularCollections {
		if col, err := client.GetCollections(); err == nil && len(col) > 0 {
			return true
		}
	}
	if config.ShowSmartCollections {
		if col, err := client.GetSmartCollections(); err == nil && len(col) > 0 {
			return true
		}
	}
	if config.ShowVirtualCollections {
		if col, err := client.GetVirtualCollections(); err == nil && len(col) > 0 {
			return true
		}
	}
	return false
}

// LoadPlatformsBinding fetches the server's platform binding and stores it on
// config for later CFW lookups.
//
// Older RomM versions have no such endpoint, so an error here is not fatal.
func LoadPlatformsBinding(config *settings.Config, host settings.Host, timeout ...time.Duration) error {
	rommConfig, err := romm.NewClientFromHost(host, timeout...).GetConfig()
	if err != nil {
		return err
	}
	config.PlatformsBinding = rommConfig.PlatformsBinding
	return nil
}

// SortByOrder arranges platforms by a saved order, appending any that the order
// does not mention. An empty order sorts alphabetically.
func SortByOrder(platforms []romm.Platform, order []string) []romm.Platform {
	if len(order) == 0 {
		return SortAlphabetically(platforms)
	}

	bySlug := make(map[string]romm.Platform, len(platforms))
	for _, p := range platforms {
		bySlug[p.FSSlug] = p
	}

	sorted := make([]romm.Platform, 0, len(platforms))
	placed := make(map[string]bool, len(order))
	for _, fsSlug := range order {
		if p, ok := bySlug[fsSlug]; ok {
			sorted = append(sorted, p)
			placed[fsSlug] = true
		}
	}

	var rest []romm.Platform
	for _, p := range platforms {
		if !placed[p.FSSlug] {
			rest = append(rest, p)
		}
	}
	return append(sorted, SortAlphabetically(rest)...)
}

// SortAlphabetically orders platforms by display name.
//
// The comparison is case sensitive, so an uppercase name sorts before a
// lowercase one. RomM titlecases platform names, so this is not visible today.
func SortAlphabetically(platforms []romm.Platform) []romm.Platform {
	sorted := make([]romm.Platform, len(platforms))
	copy(sorted, platforms)
	slices.SortStableFunc(sorted, func(a, b romm.Platform) int {
		return strings.Compare(a.Name, b.Name)
	})
	return sorted
}

// PruneOrder drops entries whose platform no longer has a directory mapping.
func PruneOrder(order []string, mappings map[string]settings.DirectoryMapping) []string {
	if len(order) == 0 {
		return order
	}

	pruned := make([]string, 0, len(order))
	for _, fsSlug := range order {
		if _, ok := mappings[fsSlug]; ok {
			pruned = append(pruned, fsSlug)
		}
	}
	return pruned
}
