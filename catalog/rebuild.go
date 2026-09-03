package catalog

import (
	"fmt"

	"go.uber.org/atomic"

	"grout/cache"
	"grout/romm"
	"grout/settings"
)

// CacheScope names which caches a rebuild clears.
type CacheScope int

const (
	// ScopeMetadata clears the game and platform tables, which are refetched.
	ScopeMetadata CacheScope = iota
	// ScopeArtwork clears downloaded cover art, which is refetched lazily as
	// screens ask for it.
	ScopeArtwork
	// ScopeAll clears both.
	ScopeAll
)

// ClearsMetadata reports whether this scope needs a metadata rebuild
// afterwards. Artwork alone does not: covers are refetched on demand.
func (s CacheScope) ClearsMetadata() bool {
	return s == ScopeMetadata || s == ScopeAll
}

func (s CacheScope) clearsArtwork() bool {
	return s == ScopeArtwork || s == ScopeAll
}

// ClearCache empties the caches named by scope.
//
// The cache manager is initialised if it is not already, since this runs after
// the user has chosen to rebuild and a missing manager should not abandon that.
func ClearCache(host settings.Host, config settings.Config, scope CacheScope) error {
	manager, err := ensureCacheManager(host, config)
	if err != nil {
		return err
	}

	if scope.ClearsMetadata() {
		if err := manager.ClearMetadata(); err != nil {
			return fmt.Errorf("clearing metadata cache: %w", err)
		}
	}
	if scope.clearsArtwork() {
		manager.ClearArtwork()
	}
	return nil
}

// RebuildMetadata refetches every mapped platform and repopulates the cache,
// returning the platforms in the user's configured order.
//
// progress, when non-nil, is advanced from 0 to 1 as platforms are populated.
func RebuildMetadata(host settings.Host, config settings.Config, progress *atomic.Float64) ([]romm.Platform, error) {
	manager, err := ensureCacheManager(host, config)
	if err != nil {
		return nil, err
	}

	platforms, err := MappedPlatforms(host, config.DirectoryMappings, config.ApiTimeout.Duration())
	if err != nil {
		return nil, fmt.Errorf("fetching platforms: %w", err)
	}
	platforms = SortByOrder(platforms, config.PlatformOrder)

	if _, err := manager.PopulateFullCacheWithProgress(platforms, progress); err != nil {
		return nil, fmt.Errorf("populating cache: %w", err)
	}
	return platforms, nil
}

func ensureCacheManager(host settings.Host, config settings.Config) (*cache.Manager, error) {
	if manager := cache.GetCacheManager(); manager != nil {
		return manager, nil
	}
	if err := cache.InitCacheManager(host, config); err != nil {
		return nil, fmt.Errorf("opening cache: %w", err)
	}
	manager := cache.GetCacheManager()
	if manager == nil {
		return nil, fmt.Errorf("opening cache: manager unavailable after initialisation")
	}
	return manager, nil
}
