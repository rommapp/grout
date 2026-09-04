package catalog

import (
	"fmt"
	"slices"
	"strings"

	"grout/cache"
	"grout/romm"
	"grout/settings"
	"grout/textmatch"
)

// GameEntry is one row of the games list.
type GameEntry struct {
	// Game is the rom itself, left as it came, so callers can key off its
	// identity rather than off anything shown on screen.
	Game romm.Rom
	// Name is what to show: the rom's readable name, prefixed with its platform
	// when a collection mixes several. Download and multi-file markers are
	// glyphs the screen owns and are not in here.
	Name string
	// Downloaded is only filled in when the config asks for the markers, since
	// answering costs a look at the card for every game in the list.
	Downloaded DownloadState
	// MultipleFiles marks a game holding several selectable versions.
	MultipleFiles bool
}

// GameList is everything a games screen needs to draw itself.
type GameList struct {
	Entries []GameEntry
	// Title names what is being browsed, without the search and filter
	// prefixes the screen adds.
	Title string
	// AllMappedOut means the collection had games but none of them are on a
	// platform the user has mapped a folder for. That needs its own message,
	// since "no games" would be misleading.
	AllMappedOut bool
}

// BrowseRequest asks for the list of games to show.
type BrowseRequest struct {
	Config     settings.Config
	Games      []romm.Rom
	Platform   romm.Platform
	Collection romm.Collection
	Filter     cache.GameFilter
	Search     string
}

// IsCollection reports whether a collection is set, real or virtual.
func IsCollection(c romm.Collection) bool {
	return c.ID != 0 || c.VirtualID != ""
}

// Browse decides which games a screen shows and what each one is called.
func Browse(request BrowseRequest) GameList {
	games := sortedByName(request.Games)

	if request.Filter.HasActiveFilters() {
		games = applyMetadataFilter(games, request)
	}

	if request.Config.DownloadedGames == settings.DownloadedGamesModeFilter {
		games = slices.DeleteFunc(games, func(game romm.Rom) bool {
			return IsDownloaded(request.Config, game)
		})
	}

	list := GameList{Title: request.Platform.Name}

	if IsCollection(request.Collection) {
		before := len(games)
		games = slices.DeleteFunc(games, func(game romm.Rom) bool {
			_, mapped := request.Config.DirectoryMappings[game.PlatformFSSlug]
			return !mapped
		})
		list.AllMappedOut = before > 0 && len(games) == 0
		list.Title = collectionTitle(request.Collection, request.Platform)
	}

	if request.Search != "" {
		games = FilterByName(games, request.Search)
	}

	list.Entries = entriesFor(request, games)
	return list
}

// sortedByName copies the games and orders them the way they are read, leaving
// the caller's slice alone: it is the unfiltered list the screen hands back.
func sortedByName(games []romm.Rom) []romm.Rom {
	sorted := slices.Clone(games)
	slices.SortFunc(sorted, func(a, b romm.Rom) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return sorted
}

// applyMetadataFilter narrows the list by genre, region and the rest, which
// only the cache can answer.
//
// A collection is narrowed by intersection, since its membership is its own
// and the cache query cannot express it. A cache that cannot answer leaves the
// list as it was, which shows too much rather than too little.
func applyMetadataFilter(games []romm.Rom, request BrowseRequest) []romm.Rom {
	manager := cache.GetCacheManager()
	if manager == nil {
		return games
	}

	filter := request.Filter
	filter.PlatformID = request.Platform.ID

	matched, err := manager.GetFilteredGames(filter)
	if err != nil {
		return games
	}

	if !IsCollection(request.Collection) {
		return sortedByName(matched)
	}

	allowed := make(map[int]struct{}, len(matched))
	for _, game := range matched {
		allowed[game.ID] = struct{}{}
	}
	return slices.DeleteFunc(games, func(game romm.Rom) bool {
		_, ok := allowed[game.ID]
		return !ok
	})
}

// collectionTitle names a collection, and the platform too once one has been
// picked out of it.
func collectionTitle(collection romm.Collection, platform romm.Platform) string {
	if platform.ID == 0 {
		return collection.Name
	}
	return fmt.Sprintf("%s - %s", collection.Name, platform.Name)
}

func entriesFor(request BrowseRequest, games []romm.Rom) []GameEntry {
	// A collection with no platform picked mixes them, so each row says which
	// one it came from.
	unified := IsCollection(request.Collection) && request.Platform.ID == 0
	marked := request.Config.DownloadedGames == settings.DownloadedGamesModeMark

	entries := make([]GameEntry, 0, len(games))
	for _, game := range games {
		name := textmatch.PrepareRomName(game.Name, game.Regions)
		if unified {
			name = fmt.Sprintf("[%s] %s", game.PlatformFSSlug, name)
		}

		entry := GameEntry{
			Game:          game,
			Name:          name,
			MultipleFiles: game.HasNestedSingleFile,
		}
		if marked {
			entry.Downloaded = DownloadStateOf(request.Config, game)
		}
		entries = append(entries, entry)
	}

	return entries
}
