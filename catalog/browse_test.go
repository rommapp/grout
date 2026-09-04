package catalog

import (
	"testing"

	"grout/cache"
	"grout/romm"
	"grout/settings"
)

func rom(id int, name, slug string) romm.Rom {
	return romm.Rom{ID: id, Name: name, PlatformFSSlug: slug}
}

func entryNames(entries []GameEntry) []string {
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry.Name)
	}
	return out
}

func testMappings(slugs ...string) map[string]settings.DirectoryMapping {
	m := make(map[string]settings.DirectoryMapping, len(slugs))
	for _, slug := range slugs {
		m[slug] = settings.DirectoryMapping{RomMSlug: slug, RelativePath: slug}
	}
	return m
}

func TestBrowse_SortsByName(t *testing.T) {
	list := Browse(BrowseRequest{
		Games: []romm.Rom{rom(1, "Zelda", "snes"), rom(2, "astro boy", "snes"), rom(3, "Mario", "snes")},
	})

	want := []string{"astro boy", "Mario", "Zelda"}
	for i, name := range want {
		if list.Entries[i].Name != name {
			t.Fatalf("order = %v, want %v (case insensitive)", entryNames(list.Entries), want)
		}
	}
}

// The screen hands the same slice back out as its unfiltered list, so browsing
// must not reorder or shorten what the caller holds.
func TestBrowse_LeavesTheCallersSliceAlone(t *testing.T) {
	games := []romm.Rom{rom(1, "Zelda", "snes"), rom(2, "Astro", "gba"), rom(3, "Mario", "snes")}

	Browse(BrowseRequest{
		Games:      games,
		Collection: romm.Collection{ID: 7},
		Config:     settings.Config{DirectoryMappings: testMappings("snes")},
	})

	if len(games) != 3 {
		t.Fatalf("caller's slice has %d games, want all 3 left in place", len(games))
	}
	for i, want := range []string{"Zelda", "Astro", "Mario"} {
		if games[i].Name != want {
			t.Errorf("game %d is %q, want %q: the caller's order must survive", i, games[i].Name, want)
		}
	}
}

// A collection spans platforms, and one the user has mapped no folder for has
// nowhere to download to.
func TestBrowse_CollectionDropsUnmappedPlatforms(t *testing.T) {
	list := Browse(BrowseRequest{
		Games:      []romm.Rom{rom(1, "Mario", "snes"), rom(2, "Sonic", "genesis")},
		Collection: romm.Collection{ID: 7, Name: "Favourites"},
		Config:     settings.Config{DirectoryMappings: testMappings("snes")},
	})

	if len(list.Entries) != 1 || list.Entries[0].Game.ID != 1 {
		t.Fatalf("entries = %v, want only the mapped platform's game", entryNames(list.Entries))
	}
	if list.AllMappedOut {
		t.Error("AllMappedOut must be false while something is still showing")
	}
}

// "No games found" would be misleading here: the collection has games, they
// just have nowhere to go. That needs its own message.
func TestBrowse_AllMappedOut(t *testing.T) {
	list := Browse(BrowseRequest{
		Games:      []romm.Rom{rom(1, "Sonic", "genesis")},
		Collection: romm.Collection{ID: 7, Name: "Favourites"},
		Config:     settings.Config{DirectoryMappings: testMappings("snes")},
	})

	if len(list.Entries) != 0 {
		t.Fatalf("entries = %v, want none", entryNames(list.Entries))
	}
	if !list.AllMappedOut {
		t.Error("AllMappedOut must be true when mappings emptied a collection that had games")
	}
}

// An empty collection is empty for the ordinary reason, so it gets the
// ordinary message.
func TestBrowse_EmptyCollectionIsNotMappedOut(t *testing.T) {
	list := Browse(BrowseRequest{
		Collection: romm.Collection{ID: 7, Name: "Favourites"},
		Config:     settings.Config{DirectoryMappings: testMappings("snes")},
	})

	if list.AllMappedOut {
		t.Error("a collection with no games was not mapped out of existence")
	}
}

// A collection with no platform chosen mixes them, so each row has to say
// which one it came from.
func TestBrowse_UnifiedCollectionNamesThePlatform(t *testing.T) {
	config := settings.Config{DirectoryMappings: testMappings("snes")}
	games := []romm.Rom{rom(1, "Mario", "snes")}

	unified := Browse(BrowseRequest{
		Games: games, Config: config,
		Collection: romm.Collection{ID: 7, Name: "Favourites"},
	})
	if got := unified.Entries[0].Name; got != "[snes] Mario" {
		t.Errorf("name = %q, want the platform named", got)
	}

	// Once a platform is picked, every row is that platform and saying so is
	// just noise.
	picked := Browse(BrowseRequest{
		Games: games, Config: config,
		Collection: romm.Collection{ID: 7, Name: "Favourites"},
		Platform:   romm.Platform{ID: 3, Name: "SNES"},
	})
	if got := picked.Entries[0].Name; got != "Mario" {
		t.Errorf("name = %q, want no platform prefix", got)
	}
}

func TestBrowse_Title(t *testing.T) {
	tests := []struct {
		name       string
		platform   romm.Platform
		collection romm.Collection
		want       string
	}{
		{"platform", romm.Platform{ID: 3, Name: "SNES"}, romm.Collection{}, "SNES"},
		{"collection", romm.Platform{}, romm.Collection{ID: 7, Name: "Favourites"}, "Favourites"},
		{
			"collection narrowed to a platform",
			romm.Platform{ID: 3, Name: "SNES"},
			romm.Collection{ID: 7, Name: "Favourites"},
			"Favourites - SNES",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			list := Browse(BrowseRequest{Platform: tt.platform, Collection: tt.collection})
			if list.Title != tt.want {
				t.Errorf("Title = %q, want %q", list.Title, tt.want)
			}
		})
	}
}

func TestBrowse_Search(t *testing.T) {
	list := Browse(BrowseRequest{
		Games:  []romm.Rom{rom(1, "Super Mario World", "snes"), rom(2, "Zelda", "snes")},
		Search: "mario",
	})

	if len(list.Entries) != 1 || list.Entries[0].Game.ID != 1 {
		t.Errorf("entries = %v, want just the match", entryNames(list.Entries))
	}
}

// Every download marker costs a look at the card. With markers switched off
// nothing is going to read the answer, so it must not be asked for.
func TestBrowse_DownloadStateOnlyWhenMarked(t *testing.T) {
	games := []romm.Rom{rom(1, "Mario", "snes")}

	for _, mode := range []settings.DownloadedGamesMode{
		settings.DownloadedGamesModeDoNothing,
		settings.DownloadedGamesModeFilter,
	} {
		list := Browse(BrowseRequest{Games: games, Config: settings.Config{DownloadedGames: mode}})
		if len(list.Entries) > 0 && list.Entries[0].Downloaded != NotDownloaded {
			t.Errorf("mode %q reported a download state nothing asked for", mode)
		}
	}
}

func TestBrowse_MultipleFiles(t *testing.T) {
	game := rom(1, "Final Fantasy VII", "psx")
	game.HasNestedSingleFile = true

	list := Browse(BrowseRequest{Games: []romm.Rom{game}})

	if !list.Entries[0].MultipleFiles {
		t.Error("a game holding several files must be flagged so the list can mark it")
	}
}

// The cache is what answers a metadata filter. Without one, showing everything
// beats showing nothing.
func TestBrowse_MetadataFilterWithoutCacheShowsEverything(t *testing.T) {
	list := Browse(BrowseRequest{
		Games:  []romm.Rom{rom(1, "Mario", "snes"), rom(2, "Zelda", "snes")},
		Filter: cache.GameFilter{Genres: []string{"Platform"}},
	})

	if len(list.Entries) != 2 {
		t.Errorf("entries = %v, want both games rather than an empty list", entryNames(list.Entries))
	}
}

// A game with no platform has no rom directory to look in, so asking is
// pointless and the answer must not be a guess.
func TestDownloadStateOf_NoPlatform(t *testing.T) {
	if got := DownloadStateOf(settings.Config{}, romm.Rom{Name: "Orphan"}); got != NotDownloaded {
		t.Errorf("DownloadStateOf = %v, want NotDownloaded", got)
	}
}

func TestIsCollection(t *testing.T) {
	tests := []struct {
		name       string
		collection romm.Collection
		want       bool
	}{
		{"none", romm.Collection{}, false},
		{"real", romm.Collection{ID: 7}, true},
		{"virtual", romm.Collection{VirtualID: "recently-added"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsCollection(tt.collection); got != tt.want {
				t.Errorf("IsCollection = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGroupByPlatform(t *testing.T) {
	games := []romm.Rom{
		{ID: 1, Name: "Zelda", PlatformFSSlug: "snes", PlatformDisplayName: "Super Nintendo"},
		{ID: 2, Name: "Sonic", PlatformFSSlug: "genesis", PlatformDisplayName: "Mega Drive"},
		{ID: 3, Name: "Mario", PlatformFSSlug: "snes", PlatformDisplayName: "Super Nintendo"},
	}
	platforms := []romm.Platform{{FSSlug: "snes"}, {FSSlug: "genesis"}}

	groups := GroupByPlatform(games, platforms)

	if len(groups) != 2 {
		t.Fatalf("got %d groups, want one per platform", len(groups))
	}
	// Platforms read in name order, not the order games happened to arrive.
	if groups[0].Name != "Mega Drive" || groups[1].Name != "Super Nintendo" {
		t.Errorf("groups = %v, %v, want them alphabetical", groups[0].Name, groups[1].Name)
	}
	if len(groups[1].Games) != 2 || groups[1].Games[0].Name != "Mario" {
		t.Errorf("SNES games = %v, want them sorted with Mario first", groups[1].Games)
	}
}

// A platform the user has unmapped has nowhere on the device for its games, so
// listing it would offer something that cannot be acted on.
func TestGroupByPlatform_DropsUnmappedPlatforms(t *testing.T) {
	games := []romm.Rom{
		{ID: 1, Name: "Mario", PlatformFSSlug: "snes"},
		{ID: 2, Name: "Sonic", PlatformFSSlug: "genesis"},
	}

	groups := GroupByPlatform(games, []romm.Platform{{FSSlug: "snes"}})

	if len(groups) != 1 || groups[0].FSSlug != "snes" {
		t.Errorf("groups = %v, want only the mapped platform", groups)
	}
}

// RomM does not always send a display name, and a blank heading tells the user
// nothing about which games are under it.
func TestGroupByPlatform_FallsBackToTheSlug(t *testing.T) {
	games := []romm.Rom{{ID: 1, Name: "Mario", PlatformFSSlug: "snes"}}

	groups := GroupByPlatform(games, []romm.Platform{{FSSlug: "snes"}})

	if len(groups) != 1 || groups[0].Name != "snes" {
		t.Errorf("group name = %q, want the slug when there is no display name", groups[0].Name)
	}
}

func TestGroupByPlatform_NoPlatformsMapped(t *testing.T) {
	games := []romm.Rom{{ID: 1, Name: "Mario", PlatformFSSlug: "snes"}}

	if groups := GroupByPlatform(games, nil); len(groups) != 0 {
		t.Errorf("groups = %v, want none when nothing is mapped", groups)
	}
}
