package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rommapp/grout/core/config"
	"github.com/rommapp/grout/core/romm"
	"github.com/rommapp/grout/core/store"
	"github.com/rommapp/grout/test/fakeromm"
)

func newRefreshLibrary(t *testing.T, f *fakeromm.Server, slugs ...string) *Library {
	t.Helper()
	cfg := &config.Config{DirectoryMappings: map[string]config.DirectoryMapping{}}
	for _, s := range slugs {
		cfg.DirectoryMappings[s] = config.DirectoryMapping{RomMSlug: s}
	}
	return New(openTestStore(t), f.Client, cfg, nil)
}

func refresh(t *testing.T, l *Library, opts RefreshOptions) RefreshResult {
	t.Helper()
	res, err := l.Refresh(context.Background(), opts, nil)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func platformCursor(t *testing.T, s *store.Store, platformID int) (string, string) {
	t.Helper()
	var cursor, status string
	err := s.DB.QueryRow(`SELECT cursor, status FROM platform_sync WHERE platform_id = ?`, platformID).Scan(&cursor, &status)
	if err != nil {
		t.Fatalf("platform_sync %d: %v", platformID, err)
	}
	return cursor, status
}

func streamCursor(t *testing.T, s *store.Store, stream string) (string, bool) {
	t.Helper()
	var v string
	err := s.DB.QueryRow(`SELECT value FROM cursors WHERE stream = ?`, stream).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false
	}
	if err != nil {
		t.Fatal(err)
	}
	return v, true
}

func query(t *testing.T, c fakeromm.Call) url.Values {
	t.Helper()
	v, err := url.ParseQuery(c.Query)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func romCallsFor(t *testing.T, f *fakeromm.Server, platformID int) []url.Values {
	t.Helper()
	var out []url.Values
	for _, c := range f.CallsTo("/api/roms") {
		q := query(t, c)
		if q.Get("platform_ids") == fmt.Sprint(platformID) {
			out = append(out, q)
		}
	}
	return out
}

func overlap(t *testing.T, cursor string) string {
	t.Helper()
	c, err := time.Parse(time.RFC3339Nano, cursor)
	if err != nil {
		t.Fatal(err)
	}
	return c.Add(-5 * time.Second).Format(time.RFC3339Nano)
}

func TestRefreshFirstFill(t *testing.T) {
	f := fakeromm.New(t)
	base := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	f.Platforms = []romm.Platform{
		{ID: 1, Slug: "gb", FSSlug: "gb", Name: "Game Boy", RomCount: 2, UpdatedAt: base},
		{ID: 2, Slug: "gba", FSSlug: "gba", Name: "Game Boy Advance", RomCount: 1, UpdatedAt: base},
	}
	f.Roms = []romm.Rom{
		{ID: 1, PlatformID: 1, Name: "a", UpdatedAt: base.Add(500 * time.Millisecond)},
		{ID: 2, PlatformID: 1, Name: "b", UpdatedAt: base},
		{ID: 3, PlatformID: 2, Name: "c", UpdatedAt: base.Add(time.Hour)},
	}
	f.Collections[romm.CollectionRegular] = []fakeromm.Collection{
		{ID: "10", Name: "favs", RomIDs: []int{1}, UpdatedAt: base.Add(250 * time.Millisecond)},
		{ID: "11", Name: "more", RomIDs: []int{2}, UpdatedAt: base.Add(time.Second)},
	}
	l := newRefreshLibrary(t, f, "gb")
	l.Config.ShowRegularCollections = true

	res := refresh(t, l, RefreshOptions{})

	if got := count(t, l.Store, `SELECT COUNT(*) FROM platforms`); got != 2 {
		t.Fatalf("platforms = %d", got)
	}
	if got := count(t, l.Store, `SELECT COUNT(*) FROM games WHERE platform_id = 1`); got != 2 {
		t.Fatalf("gb games = %d", got)
	}
	if got := count(t, l.Store, `SELECT COUNT(*) FROM games WHERE platform_id = 2`); got != 0 {
		t.Fatalf("gba games = %d", got)
	}
	cursor, status := platformCursor(t, l.Store, 1)
	if want := base.Add(500 * time.Millisecond).Format(time.RFC3339Nano); cursor != want || status != "ok" {
		t.Fatalf("cursor = %q status = %q, want %q ok", cursor, status, want)
	}
	if count(t, l.Store, `SELECT COUNT(*) FROM platform_sync WHERE platform_id = 2`) != 0 {
		t.Fatal("disabled platform has a sync row")
	}
	col, ok := streamCursor(t, l.Store, "collections:regular")
	if want := base.Add(time.Second).Format(time.RFC3339Nano); !ok || col != want {
		t.Fatalf("collections cursor = %q, want %q", col, want)
	}
	if res.Platforms != 2 || res.Games != 2 || res.Collections != 2 || res.Deleted != 0 {
		t.Fatalf("result = %+v", res)
	}
	if len(res.FailedPlatforms) != 0 || len(res.CollectionErrors) != 0 {
		t.Fatalf("result = %+v", res)
	}
}

func TestRefreshIncrementalSendsCursors(t *testing.T) {
	f := fakeromm.New(t)
	base := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	f.Platforms = []romm.Platform{{ID: 1, FSSlug: "gb", Name: "Game Boy", RomCount: 2, UpdatedAt: base}}
	f.Roms = fakeromm.Roms(1, 1, 2, base.Add(time.Minute))
	f.Collections[romm.CollectionRegular] = []fakeromm.Collection{{ID: "10", Name: "favs", RomIDs: []int{1}, UpdatedAt: base.Add(2 * time.Minute)}}
	f.Collections[romm.CollectionVirtual] = []fakeromm.Collection{{ID: "v1", Name: "virt", RomIDs: []int{2}, UpdatedAt: base}}
	l := newRefreshLibrary(t, f, "gb")
	l.Config.ShowRegularCollections = true
	l.Config.ShowVirtualCollections = true

	refresh(t, l, RefreshOptions{})
	romCursor, _ := platformCursor(t, l.Store, 1)
	colCursor, _ := streamCursor(t, l.Store, "collections:regular")
	if _, ok := streamCursor(t, l.Store, "collections:virtual"); ok {
		t.Fatal("virtual cursor stored")
	}
	before := len(f.Calls())

	refresh(t, l, RefreshOptions{})
	second := f.Calls()[before:]

	var sawRoms, sawRegular, sawVirtual, sawPlatforms bool
	for _, c := range second {
		q := query(t, c)
		switch c.Path {
		case "/api/roms":
			sawRoms = true
			if got := q.Get("updated_after"); got != overlap(t, romCursor) {
				t.Errorf("roms updated_after = %q, want %q", got, overlap(t, romCursor))
			}
		case "/api/collections":
			sawRegular = true
			if got := q.Get("updated_after"); got != overlap(t, colCursor) {
				t.Errorf("collections updated_after = %q, want %q", got, overlap(t, colCursor))
			}
		case "/api/collections/virtual":
			sawVirtual = true
			if q.Has("updated_after") {
				t.Errorf("virtual sent updated_after: %s", c.Query)
			}
		case "/api/platforms":
			sawPlatforms = true
			if q.Has("updated_after") {
				t.Errorf("platforms sent updated_after: %s", c.Query)
			}
		}
	}
	if !sawRoms || !sawRegular || !sawVirtual || !sawPlatforms {
		t.Fatalf("missing calls: %+v", second)
	}
}

func TestRefreshPlatformCountChangeWithoutUpdatedAt(t *testing.T) {
	f := fakeromm.New(t)
	base := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	f.Platforms = []romm.Platform{{ID: 1, FSSlug: "gb", Name: "Game Boy", RomCount: 0, UpdatedAt: base}}
	l := newRefreshLibrary(t, f)

	refresh(t, l, RefreshOptions{})
	f.Mutate(func() { f.Platforms[0].RomCount = 3 })
	refresh(t, l, RefreshOptions{})

	if got := count(t, l.Store, `SELECT rom_count FROM platforms WHERE id = 1`); got != 3 {
		t.Fatalf("rom_count = %d", got)
	}
}

func TestRefreshFullFetchRerunsOnShortCount(t *testing.T) {
	f := fakeromm.New(t)
	base := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	f.Platforms = []romm.Platform{{ID: 1, FSSlug: "gb", Name: "Game Boy", RomCount: 600, UpdatedAt: base}}
	f.Roms = fakeromm.Roms(1, 1, 600, base)
	dropped := false
	f.SetBeforePage(func(platformID, offset int) {
		if offset != 250 || dropped {
			return
		}
		dropped = true
		f.Mutate(func() {
			for i, r := range f.Roms {
				if r.ID == 10 {
					f.Roms = append(f.Roms[:i], f.Roms[i+1:]...)
					break
				}
			}
			f.Platforms[0].RomCount = 599
		})
	})
	l := newRefreshLibrary(t, f, "gb")

	refresh(t, l, RefreshOptions{})

	if got := count(t, l.Store, `SELECT COUNT(*) FROM games`); got != 599 {
		t.Fatalf("games = %d", got)
	}
	if got := count(t, l.Store, `SELECT COUNT(*) FROM games WHERE id = 251`); got != 1 {
		t.Fatal("game 251 missing")
	}
	if got := count(t, l.Store, `SELECT COUNT(*) FROM games WHERE id = 10`); got != 0 {
		t.Fatal("deleted game 10 kept")
	}
	if pages := f.RomPages(1); pages != 6 {
		t.Fatalf("rom pages = %d, want 6", pages)
	}
}

func TestRefreshFullOptionIgnoresCursors(t *testing.T) {
	f := fakeromm.New(t)
	base := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	f.Platforms = []romm.Platform{{ID: 1, FSSlug: "gb", Name: "Game Boy", RomCount: 2, UpdatedAt: base}}
	f.Roms = fakeromm.Roms(1, 1, 2, base)
	f.Collections[romm.CollectionRegular] = []fakeromm.Collection{{ID: "10", Name: "a", UpdatedAt: base}}
	f.Collections[romm.CollectionSmart] = []fakeromm.Collection{{ID: "20", Name: "s", UpdatedAt: base}}
	l := newRefreshLibrary(t, f, "gb")
	l.Config.ShowRegularCollections = true
	l.Config.ShowSmartCollections = true

	refresh(t, l, RefreshOptions{})
	before := len(f.Calls())
	res := refresh(t, l, RefreshOptions{Full: true})

	for _, c := range f.Calls()[before:] {
		if query(t, c).Has("updated_after") {
			t.Errorf("%s sent updated_after: %s", c.Path, c.Query)
		}
	}
	if res.Games != 2 || res.Collections != 2 {
		t.Fatalf("result = %+v", res)
	}
}

func TestRefreshUnchangedCollectionsKeepMembers(t *testing.T) {
	f := fakeromm.New(t)
	base := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	f.Platforms = []romm.Platform{{ID: 1, FSSlug: "gb", Name: "Game Boy", RomCount: 3, UpdatedAt: base}}
	f.Roms = fakeromm.Roms(1, 1, 3, base)
	f.Collections[romm.CollectionRegular] = []fakeromm.Collection{
		{ID: "10", Name: "a", RomIDs: []int{1, 2}, UpdatedAt: base},
		{ID: "11", Name: "b", RomIDs: []int{3}, UpdatedAt: base.Add(time.Minute)},
	}
	l := newRefreshLibrary(t, f, "gb")
	l.Config.ShowRegularCollections = true

	refresh(t, l, RefreshOptions{})
	f.Mutate(func() {
		f.Collections[romm.CollectionRegular][1].RomIDs = []int{1, 3}
		f.Collections[romm.CollectionRegular][1].UpdatedAt = base.Add(2 * time.Hour)
	})
	res := refresh(t, l, RefreshOptions{})

	if res.Collections != 1 {
		t.Fatalf("collections saved = %d, want 1", res.Collections)
	}
	if got := count(t, l.Store, `SELECT COUNT(*) FROM collection_games WHERE kind = 'regular' AND remote_id = 10`); got != 2 {
		t.Fatalf("collection 10 members = %d", got)
	}
	if got := count(t, l.Store, `SELECT COUNT(*) FROM collection_games WHERE kind = 'regular' AND remote_id = 11`); got != 2 {
		t.Fatalf("collection 11 members = %d", got)
	}
}

func TestRefreshEmptyIdentifiersPurgesAll(t *testing.T) {
	f := fakeromm.New(t)
	base := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	f.Platforms = []romm.Platform{{ID: 1, FSSlug: "gb", Name: "Game Boy", RomCount: 3, UpdatedAt: base}}
	f.Roms = fakeromm.Roms(1, 1, 3, base)
	l := newRefreshLibrary(t, f, "gb")

	refresh(t, l, RefreshOptions{})
	old := count(t, l.Store, `SELECT COUNT(*) FROM games`)
	if old != 3 {
		t.Fatalf("first fill games = %d", old)
	}
	f.Mutate(func() {
		f.Roms = nil
		f.Platforms[0].RomCount = 0
	})
	res := refresh(t, l, RefreshOptions{})

	if got := count(t, l.Store, `SELECT COUNT(*) FROM games`); got != 0 {
		t.Fatalf("games = %d", got)
	}
	if res.Deleted != old {
		t.Fatalf("deleted = %d, want %d", res.Deleted, old)
	}
}

func TestRefreshPlatformFailureKeepsCursor(t *testing.T) {
	f := fakeromm.New(t)
	base := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	f.Platforms = []romm.Platform{
		{ID: 1, FSSlug: "gb", Name: "A Game Boy", RomCount: 300, UpdatedAt: base},
		{ID: 2, FSSlug: "gba", Name: "B Game Boy Advance", RomCount: 300, UpdatedAt: base},
	}
	f.Roms = append(fakeromm.Roms(1, 1, 300, base), fakeromm.Roms(2, 1001, 300, base)...)
	l := newRefreshLibrary(t, f, "gb", "gba")

	refresh(t, l, RefreshOptions{})
	oldA, _ := platformCursor(t, l.Store, 1)
	oldB, _ := platformCursor(t, l.Store, 2)
	firstPages := f.RomPages(1)

	later := base.Add(time.Hour)
	f.Mutate(func() {
		for i := range f.Roms {
			f.Roms[i].UpdatedAt = later
		}
	})
	f.SetFailAfter(1, firstPages+2)
	res := refresh(t, l, RefreshOptions{})

	if !reflect.DeepEqual(res.FailedPlatforms, []string{"gb"}) {
		t.Fatalf("failed = %v", res.FailedPlatforms)
	}
	cursorA, statusA := platformCursor(t, l.Store, 1)
	if cursorA != oldA || statusA != "failed" {
		t.Fatalf("A cursor = %q status = %q, want %q failed", cursorA, statusA, oldA)
	}
	cursorB, statusB := platformCursor(t, l.Store, 2)
	if cursorB == oldB || cursorB != later.Format(time.RFC3339Nano) || statusB != "ok" {
		t.Fatalf("B cursor = %q status = %q", cursorB, statusB)
	}

	f.ClearFailAfter(1)
	before := len(romCallsFor(t, f, 1))
	res = refresh(t, l, RefreshOptions{})
	calls := romCallsFor(t, f, 1)[before:]
	if len(calls) == 0 || calls[0].Get("updated_after") != overlap(t, oldA) {
		t.Fatalf("third refresh A calls = %v", calls)
	}
	if len(res.FailedPlatforms) != 0 {
		t.Fatalf("failed = %v", res.FailedPlatforms)
	}
	cursorA, statusA = platformCursor(t, l.Store, 1)
	if cursorA != later.Format(time.RFC3339Nano) || statusA != "ok" {
		t.Fatalf("A cursor = %q status = %q", cursorA, statusA)
	}
}

func TestRefreshCursorOverlap(t *testing.T) {
	f := fakeromm.New(t)
	at := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	f.Platforms = []romm.Platform{{ID: 1, FSSlug: "gb", Name: "Game Boy", RomCount: 1, UpdatedAt: at}}
	f.Roms = fakeromm.Roms(1, 1, 1, at)
	l := newRefreshLibrary(t, f, "gb")

	refresh(t, l, RefreshOptions{})
	if cursor, _ := platformCursor(t, l.Store, 1); cursor != at.Format(time.RFC3339Nano) {
		t.Fatalf("cursor = %q", cursor)
	}
	f.Mutate(func() {
		f.Roms = append(f.Roms, fakeromm.Roms(1, 2, 1, at)...)
		f.Platforms[0].RomCount = 2
	})
	refresh(t, l, RefreshOptions{})

	if got := count(t, l.Store, `SELECT COUNT(*) FROM games`); got != 2 {
		t.Fatalf("games = %d", got)
	}
}

func TestRefreshNewlyEnabledPlatformFullFetch(t *testing.T) {
	f := fakeromm.New(t)
	base := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	f.Platforms = []romm.Platform{
		{ID: 1, FSSlug: "gb", Name: "A", RomCount: 1, UpdatedAt: base},
		{ID: 2, FSSlug: "gba", Name: "B", RomCount: 1, UpdatedAt: base},
	}
	f.Roms = append(fakeromm.Roms(1, 1, 1, base), fakeromm.Roms(2, 2, 1, base)...)
	l := newRefreshLibrary(t, f, "gb")

	refresh(t, l, RefreshOptions{})
	if calls := romCallsFor(t, f, 2); len(calls) != 0 {
		t.Fatalf("disabled platform fetched: %v", calls)
	}
	l.Config.DirectoryMappings["gba"] = config.DirectoryMapping{RomMSlug: "gba"}
	refresh(t, l, RefreshOptions{})

	calls := romCallsFor(t, f, 2)
	if len(calls) == 0 || calls[0].Has("updated_after") {
		t.Fatalf("B calls = %v", calls)
	}
	aCalls := romCallsFor(t, f, 1)
	if !aCalls[len(aCalls)-1].Has("updated_after") {
		t.Fatal("A second refresh sent no cursor")
	}
	if got := count(t, l.Store, `SELECT COUNT(*) FROM games WHERE platform_id = 2`); got != 1 {
		t.Fatalf("B games = %d", got)
	}
}

func TestRefreshDisabledKindDropsRowsAndCursor(t *testing.T) {
	f := fakeromm.New(t)
	base := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	f.Collections[romm.CollectionSmart] = []fakeromm.Collection{{ID: "20", Name: "s", RomIDs: []int{1}, UpdatedAt: base}}
	f.Collections[romm.CollectionRegular] = []fakeromm.Collection{{ID: "10", Name: "r", RomIDs: []int{1}, UpdatedAt: base}}
	l := newRefreshLibrary(t, f)
	l.Config.ShowSmartCollections = true
	l.Config.ShowRegularCollections = true

	refresh(t, l, RefreshOptions{})
	if _, ok := streamCursor(t, l.Store, "collections:smart"); !ok {
		t.Fatal("smart cursor missing after first refresh")
	}
	l.Config.ShowSmartCollections = false
	res := refresh(t, l, RefreshOptions{})

	if got := count(t, l.Store, `SELECT COUNT(*) FROM collections WHERE kind = 'smart'`); got != 0 {
		t.Fatalf("smart collections = %d", got)
	}
	if got := count(t, l.Store, `SELECT COUNT(*) FROM collection_games WHERE kind = 'smart'`); got != 0 {
		t.Fatalf("smart members = %d", got)
	}
	if _, ok := streamCursor(t, l.Store, "collections:smart"); ok {
		t.Fatal("smart cursor kept")
	}
	if got := count(t, l.Store, `SELECT COUNT(*) FROM collections WHERE kind = 'regular'`); got != 1 {
		t.Fatalf("regular collections = %d", got)
	}
	if res.Deleted != 1 {
		t.Fatalf("deleted = %d", res.Deleted)
	}
}

func TestRefreshCollectionErrorIsReported(t *testing.T) {
	f := fakeromm.New(t)
	base := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	f.Collections[romm.CollectionSmart] = []fakeromm.Collection{{ID: "20", Name: "s", UpdatedAt: base}}
	f.Collections[romm.CollectionRegular] = []fakeromm.Collection{{ID: "10", Name: "r", UpdatedAt: base}}
	f.FailPath("/api/collections/smart")
	l := newRefreshLibrary(t, f)
	l.Config.ShowSmartCollections = true
	l.Config.ShowRegularCollections = true

	res, err := l.Refresh(context.Background(), RefreshOptions{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.CollectionErrors) != 1 || !strings.HasPrefix(res.CollectionErrors[0], "smart:") {
		t.Fatalf("collection errors = %v", res.CollectionErrors)
	}
	if got := count(t, l.Store, `SELECT COUNT(*) FROM collections WHERE kind = 'regular'`); got != 1 {
		t.Fatalf("regular collections = %d", got)
	}
	if _, ok := streamCursor(t, l.Store, "collections:smart"); ok {
		t.Fatal("smart cursor stored after failure")
	}
}

func TestRefreshProgressStages(t *testing.T) {
	f := fakeromm.New(t)
	base := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	f.Platforms = []romm.Platform{{ID: 1, FSSlug: "gb", Name: "Game Boy", RomCount: 300, UpdatedAt: base}}
	f.Roms = fakeromm.Roms(1, 1, 300, base)
	f.Collections[romm.CollectionRegular] = []fakeromm.Collection{{ID: "10", Name: "r", UpdatedAt: base}}
	l := newRefreshLibrary(t, f, "gb")
	l.Config.ShowRegularCollections = true

	f.Roms[299].UpdatedAt = base.Add(time.Hour)
	refresh(t, l, RefreshOptions{})
	f.Mutate(func() { f.Roms[0].UpdatedAt = base.Add(2 * time.Hour) })
	var reports []Progress
	if _, err := l.Refresh(context.Background(), RefreshOptions{}, func(p Progress) { reports = append(reports, p) }); err != nil {
		t.Fatal(err)
	}

	var stages []Stage
	var lastGames Progress
	for _, p := range reports {
		if len(stages) == 0 || stages[len(stages)-1] != p.Stage {
			stages = append(stages, p.Stage)
		}
		if p.Stage == StageGames {
			lastGames = p
		}
	}
	want := []Stage{StagePlatforms, StageGames, StagePurge, StageCollections}
	if !reflect.DeepEqual(stages, want) {
		t.Fatalf("stages = %v, want %v", stages, want)
	}
	if lastGames.Total != 300 || lastGames.Done != lastGames.Total {
		t.Fatalf("last games report = %+v", lastGames)
	}
}

func TestRefreshPlatformsFailureReturnsError(t *testing.T) {
	f := fakeromm.New(t)
	f.FailPath("/api/platforms")
	l := newRefreshLibrary(t, f)

	if _, err := l.Refresh(context.Background(), RefreshOptions{}, nil); err == nil {
		t.Fatal("expected error")
	}
	if got := len(f.CallsTo("/api/roms")); got != 0 {
		t.Fatalf("roms calls = %d", got)
	}
}

func TestRefreshCancelledContextReturnsError(t *testing.T) {
	f := fakeromm.New(t)
	base := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	f.Platforms = []romm.Platform{{ID: 1, FSSlug: "gb", Name: "Game Boy", RomCount: 300, UpdatedAt: base}}
	f.Roms = fakeromm.Roms(1, 1, 300, base)
	l := newRefreshLibrary(t, f, "gb")
	ctx, cancel := context.WithCancel(context.Background())
	f.SetBeforePage(func(platformID, offset int) {
		if offset == 250 {
			cancel()
		}
	})

	_, err := l.Refresh(ctx, RefreshOptions{}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	if got := count(t, l.Store, `SELECT COUNT(*) FROM platform_sync WHERE status = 'failed'`); got != 0 {
		t.Fatal("cancelled platform marked failed")
	}
}

func TestRefreshRomIDsFailureIsReported(t *testing.T) {
	f := fakeromm.New(t)
	base := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	f.Platforms = []romm.Platform{{ID: 1, FSSlug: "gb", Name: "Game Boy", RomCount: 3, UpdatedAt: base}}
	f.Roms = fakeromm.Roms(1, 1, 3, base)
	f.Collections[romm.CollectionRegular] = []fakeromm.Collection{{ID: "10", Name: "r", RomIDs: []int{1}, UpdatedAt: base}}
	l := newRefreshLibrary(t, f, "gb")
	l.Config.ShowRegularCollections = true

	refresh(t, l, RefreshOptions{})
	f.Mutate(func() {
		f.Roms = f.Roms[:1]
		f.Collections[romm.CollectionRegular] = append(f.Collections[romm.CollectionRegular],
			fakeromm.Collection{ID: "11", Name: "s", RomIDs: []int{1}, UpdatedAt: base.Add(time.Hour)})
	})
	f.FailPath("/api/roms/identifiers")

	res, err := l.Refresh(context.Background(), RefreshOptions{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.PurgeError == "" {
		t.Fatalf("result = %+v", res)
	}
	if got := count(t, l.Store, `SELECT COUNT(*) FROM games`); got != 3 {
		t.Fatalf("games = %d", got)
	}
	if got := count(t, l.Store, `SELECT COUNT(*) FROM collections WHERE kind = 'regular'`); got != 2 {
		t.Fatalf("regular collections = %d", got)
	}
}

func TestRefreshFullFetchHiddenRomsNoRerun(t *testing.T) {
	f := fakeromm.New(t)
	base := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	f.Platforms = []romm.Platform{{ID: 1, FSSlug: "gb", Name: "Game Boy", RomCount: 301, UpdatedAt: base}}
	f.Roms = fakeromm.Roms(1, 1, 301, base)
	f.Hidden[301] = true
	l := newRefreshLibrary(t, f, "gb")

	res := refresh(t, l, RefreshOptions{})

	if pages := f.RomPages(1); pages != 2 {
		t.Fatalf("rom pages = %d, want 2", pages)
	}
	if res.Games != 300 {
		t.Fatalf("games = %d", res.Games)
	}
	calls := romCallsFor(t, f, 1)
	if calls[0].Get("with_total") != "true" || calls[1].Get("with_total") != "false" {
		t.Fatalf("with_total = %q %q", calls[0].Get("with_total"), calls[1].Get("with_total"))
	}

	f.Mutate(func() { f.Roms[0].UpdatedAt = base.Add(time.Hour) })
	before := len(calls)
	refresh(t, l, RefreshOptions{})
	for _, q := range romCallsFor(t, f, 1)[before:] {
		if q.Get("with_total") != "false" {
			t.Fatalf("incremental with_total = %q", q.Get("with_total"))
		}
	}
}

func TestRefreshFullFetchWithoutTotalFallsBackToRomCount(t *testing.T) {
	f := fakeromm.New(t)
	base := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	f.Platforms = []romm.Platform{{ID: 1, FSSlug: "gb", Name: "Game Boy", RomCount: 3, UpdatedAt: base}}
	f.Roms = fakeromm.Roms(1, 1, 3, base)
	f.Hidden[3] = true
	f.OmitTotal = true
	l := newRefreshLibrary(t, f, "gb")

	refresh(t, l, RefreshOptions{})

	if pages := f.RomPages(1); pages != 2 {
		t.Fatalf("rom pages = %d, want 2", pages)
	}
}

func TestRefreshCursorCappedAtPassStart(t *testing.T) {
	f := fakeromm.New(t)
	base := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	start := base.Add(time.Hour)
	f.SetClock(start)
	f.Platforms = []romm.Platform{{ID: 1, FSSlug: "gb", Name: "Game Boy", RomCount: 300, UpdatedAt: base}}
	f.Roms = fakeromm.Roms(1, 1, 300, base)
	l := newRefreshLibrary(t, f, "gb")

	refresh(t, l, RefreshOptions{})
	f.Mutate(func() {
		for i := range f.Roms {
			f.Roms[i].UpdatedAt = base.Add(10 * time.Minute)
		}
	})
	mutated := false
	f.SetBeforePage(func(platformID, offset int) {
		if offset != 250 || mutated {
			return
		}
		mutated = true
		f.Mutate(func() {
			f.Roms[4].Name = "low changed"
			f.Roms[4].UpdatedAt = start.Add(time.Hour)
			f.Roms[289].Name = "high changed"
			f.Roms[289].UpdatedAt = start.Add(2 * time.Hour)
			f.Clock = start.Add(2 * time.Hour)
		})
	})
	refresh(t, l, RefreshOptions{})
	if !mutated {
		t.Fatal("incremental pass did not span two pages")
	}
	if cursor, _ := platformCursor(t, l.Store, 1); cursor != start.Format(time.RFC3339Nano) {
		t.Fatalf("cursor = %q, want %q", cursor, start.Format(time.RFC3339Nano))
	}

	refresh(t, l, RefreshOptions{})

	var name string
	if err := l.Store.DB.QueryRow(`SELECT name FROM games WHERE id = 5`).Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != "low changed" {
		t.Fatalf("game 5 name = %q", name)
	}
}
