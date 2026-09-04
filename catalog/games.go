package catalog

import (
	"fmt"
	"slices"
	"strings"

	"go.uber.org/atomic"

	"grout/cache"
	"grout/romm"
)

// GameSource names where a list of games is being read from.
type GameSource struct {
	Platform   romm.Platform
	Collection romm.Collection
}

// IsCollection reports whether this source is a collection rather than a
// platform.
func (s GameSource) IsCollection() bool { return s.Collection.ID != 0 }

// Name is what to call this source on screen.
func (s GameSource) Name() string {
	if s.IsCollection() {
		return s.Collection.Name
	}
	return s.Platform.Name
}

// HasBIOS reports whether the server has firmware for this platform.
// Collections span platforms, so the question does not apply to them.
func (s GameSource) HasBIOS() bool {
	return !s.IsCollection() && s.Platform.ID != 0 && s.Platform.FirmwareCount > 0
}

// CachedGames returns the games already held locally, and whether any were
// found. A miss means the caller should refresh, which is slow enough to want a
// progress indicator.
func CachedGames(source GameSource) ([]romm.Rom, bool) {
	manager := cache.GetCacheManager()
	if manager == nil {
		return nil, false
	}

	var games []romm.Rom
	var err error
	if source.IsCollection() {
		games, err = manager.GetCollectionGames(source.Collection)
	} else {
		games, err = manager.GetPlatformGames(source.Platform.ID)
	}

	if err != nil || len(games) == 0 {
		return nil, false
	}
	return games, true
}

// RefreshGames refetches a source from the server and returns what it now
// holds.
//
// progress, when non-nil, is advanced from 0 to 1 for a platform. Collections
// are already populated by the initial cache build and cannot be refreshed
// individually.
func RefreshGames(source GameSource, progress *atomic.Float64) ([]romm.Rom, error) {
	manager := cache.GetCacheManager()
	if manager == nil {
		return nil, fmt.Errorf("cache unavailable")
	}

	if source.IsCollection() {
		games, err := manager.GetCollectionGames(source.Collection)
		if err != nil {
			return nil, fmt.Errorf("reading collection %q: %w", source.Collection.Name, err)
		}
		return games, nil
	}

	if err := manager.RefreshPlatformGamesWithProgress(source.Platform, progress); err != nil {
		return nil, fmt.Errorf("refreshing %q: %w", source.Platform.Name, err)
	}

	games, err := manager.GetPlatformGames(source.Platform.ID)
	if err != nil {
		return nil, fmt.Errorf("reading %q after refresh: %w", source.Platform.Name, err)
	}
	return games, nil
}

// FilterByName keeps the games whose name contains filter, ignoring case, and
// orders the result by name.
func FilterByName(games []romm.Rom, filter string) []romm.Rom {
	needle := strings.ToLower(filter)

	var matched []romm.Rom
	for _, game := range games {
		if strings.Contains(strings.ToLower(game.Name), needle) {
			matched = append(matched, game)
		}
	}

	slices.SortFunc(matched, func(a, b romm.Rom) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return matched
}

// HasFilterableMetadata reports whether any game carries metadata a filter can
// narrow by.
//
// It mirrors the categories the filter screen offers, so the Filters button
// appears exactly when that screen would have something to show.
func HasFilterableMetadata(games []romm.Rom) bool {
	for i := range games {
		g := &games[i]
		if len(g.Metadatum.Genres) > 0 || len(g.Metadatum.Franchises) > 0 ||
			len(g.Metadatum.Companies) > 0 || len(g.Metadatum.GameModes) > 0 ||
			len(g.Metadatum.AgeRatings) > 0 || len(g.Regions) > 0 ||
			len(g.Languages) > 0 || len(g.Tags) > 0 {
			return true
		}
	}
	return false
}
