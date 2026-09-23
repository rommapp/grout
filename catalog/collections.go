package catalog

import (
	"fmt"
	"slices"
	"strings"

	"grout/cache"
	"grout/romm"
	"grout/settings"
)

// Mapped reports whether the user has given a platform a folder on the device.
//
// This is what makes a platform's games worth offering: without one there is
// nowhere to download them to.
func Mapped(config settings.Config, fsSlug string) bool {
	_, ok := config.DirectoryMappings[fsSlug]
	return ok
}

// GamesOnMappedPlatforms keeps the games that have somewhere to go.
func GamesOnMappedPlatforms(config settings.Config, games []romm.Rom) []romm.Rom {
	kept := make([]romm.Rom, 0, len(games))
	for _, game := range games {
		if Mapped(config, game.PlatformFSSlug) {
			kept = append(kept, game)
		}
	}
	return kept
}

// CollectionGames reads a collection's games out of the cache.
//
// The join table is asked first. A cache built before that table existed still
// holds the games themselves, so the collection's own rom ids are the fallback
// rather than a trip to the server.
func CollectionGames(collection romm.Collection) ([]romm.Rom, error) {
	manager := cache.GetCacheManager()
	if manager == nil {
		return nil, fmt.Errorf("cache unavailable")
	}

	if games, err := manager.GetCollectionGames(collection); err == nil && len(games) > 0 {
		return games, nil
	}

	if len(collection.ROMIDs) == 0 {
		return nil, fmt.Errorf("collection %q is not cached", collection.Name)
	}

	games, err := manager.GetGamesByIDs(collection.ROMIDs)
	if err != nil || len(games) == 0 {
		return nil, fmt.Errorf("collection %q is not cached", collection.Name)
	}
	return games, nil
}

// PlatformsIn lists the platforms a set of games covers, each with its own
// games, keeping only those the user has mapped.
func PlatformsIn(config settings.Config, games []romm.Rom) []PlatformGroup {
	return groupByPlatform(games, func(fsSlug string) bool { return Mapped(config, fsSlug) })
}

// VisibleCollections lists the collections worth showing: the kinds the user
// asked for, that hold at least one game on a platform they have mapped.
//
// A collection whose games are all on unmapped platforms would open empty, so
// it is left out rather than offered.
func VisibleCollections(config settings.Config) []romm.Collection {
	manager := cache.GetCacheManager()
	if manager == nil || !manager.HasCollections() {
		return nil
	}

	wanted := map[string]bool{
		"regular": config.ShowRegularCollections,
		"smart":   config.ShowSmartCollections,
		"virtual": config.ShowVirtualCollections,
	}

	var collections []romm.Collection
	for kind, show := range wanted {
		if !show {
			continue
		}
		if found, err := manager.GetCollectionsByType(kind); err == nil {
			collections = append(collections, found...)
		}
	}

	collections = withMappedGames(manager, config, collections)

	slices.SortFunc(collections, func(a, b romm.Collection) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return collections
}

// withMappedGames drops the collections holding nothing the device can take.
//
// A cache that knows of no games on any mapped platform has nothing to judge
// by, so the collections are left alone rather than all hidden.
func withMappedGames(manager *cache.Manager, config settings.Config, collections []romm.Collection) []romm.Collection {
	slugs := make([]string, 0, len(config.DirectoryMappings))
	for slug := range config.DirectoryMappings {
		slugs = append(slugs, slug)
	}

	reachable := manager.GetCachedGameIDsForPlatforms(slugs)
	if len(reachable) == 0 {
		return collections
	}

	kept := make([]romm.Collection, 0, len(collections))
	for _, collection := range collections {
		for _, romID := range collection.ROMIDs {
			if reachable[romID] {
				kept = append(kept, collection)
				break
			}
		}
	}
	return kept
}

// FilterCollectionsByName keeps the collections whose name contains filter,
// ignoring case.
func FilterCollectionsByName(collections []romm.Collection, filter string) []romm.Collection {
	needle := strings.ToLower(filter)

	matched := make([]romm.Collection, 0, len(collections))
	for _, collection := range collections {
		if strings.Contains(strings.ToLower(collection.Name), needle) {
			matched = append(matched, collection)
		}
	}
	return matched
}
