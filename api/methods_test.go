package api

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rommapp/grout/core/config"
	"github.com/rommapp/grout/core/library"
	"github.com/rommapp/grout/core/romm"
	"github.com/rommapp/grout/test/fakeromm"
)

var tableMethods = []string{
	"status",
	"settings.get",
	"settings.set_host",
	"settings.set_directory_mapping",
	"library.refresh",
	"library.scan",
	"library.platforms",
	"library.games",
	"library.filter_values",
	"library.dims",
	"library.collections",
	"library.collection_platforms",
	"library.game_state",
}

func fixtureServer(t *testing.T) *fakeromm.Server {
	t.Helper()
	srv := fakeromm.New(t)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	srv.Platforms = []romm.Platform{
		{ID: 1, Slug: "gba", FSSlug: "gba", Name: "Game Boy Advance", RomCount: 2, UpdatedAt: base},
		{ID: 2, Slug: "snes", FSSlug: "snes", Name: "Super Nintendo", RomCount: 1, UpdatedAt: base},
	}
	srv.Roms = append(fakeromm.Roms(1, 1, 2, base), fakeromm.Roms(2, 3, 1, base)...)
	srv.Collections[romm.CollectionRegular] = []fakeromm.Collection{{ID: "7", Name: "Favorites", RomIDs: []int{1, 3}, UpdatedAt: base}}
	return srv
}

func connectedServer(t *testing.T, srv *fakeromm.Server) *Server {
	t.Helper()
	dir := t.TempDir()
	if err := config.Save(dir, &config.Config{ShowRegularCollections: true}); err != nil {
		t.Fatal(err)
	}
	s := openServer(t, dir)
	mustOK(t, s, "settings.set_host", fmt.Sprintf(`{"root_uri":%q,"token":"secret-token"}`, srv.URL()))
	mustOK(t, s, "settings.set_directory_mapping", `{"fs_slug":"gba","slug":"gba","enabled":true}`)
	mustOK(t, s, "settings.set_directory_mapping", `{"fs_slug":"snes","slug":"snes","enabled":true}`)
	return s
}

func keysOf(t *testing.T, raw json.RawMessage) []string {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("decoding object %s: %v", raw, err)
	}
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func field(t *testing.T, raw json.RawMessage, name string) json.RawMessage {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("decoding object %s: %v", raw, err)
	}
	v, ok := m[name]
	if !ok {
		t.Fatalf("%s has no %q", raw, name)
	}
	return v
}

func items(t *testing.T, raw json.RawMessage) []json.RawMessage {
	t.Helper()
	var out []json.RawMessage
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decoding array %s: %v", raw, err)
	}
	return out
}

func wantKeys(t *testing.T, what string, raw json.RawMessage, want ...string) {
	t.Helper()
	want = slices.Clone(want)
	sort.Strings(want)
	if got := keysOf(t, raw); !reflect.DeepEqual(got, want) {
		t.Fatalf("%s keys = %v, want %v", what, got, want)
	}
}

func wantEachKeys(t *testing.T, what string, raw json.RawMessage, n int, want ...string) {
	t.Helper()
	list := items(t, raw)
	if len(list) != n {
		t.Fatalf("%s = %s, want %d items", what, raw, n)
	}
	for _, it := range list {
		wantKeys(t, what, it, want...)
	}
}

var settingsKeys = []string{
	"hosts", "directory_mappings", "show_regular_collections", "show_smart_collections",
	"show_virtual_collections", "downloaded_games", "api_timeout", "download_timeout", "log_level", "platform_order",
}

var hostKeys = []string{"display_name", "root_uri", "port", "username", "has_token", "insecure_skip_verify"}

var platformKeys = []string{"id", "slug", "fs_slug", "name", "rom_count", "enabled", "proposed", "rom_dirs"}

var refreshKeys = []string{"platforms", "games", "collections", "deleted", "failed_platforms", "collection_errors", "purge_error"}

var scanKeys = []string{"files", "matched", "platforms", "skipped_folders"}

func checkSettings(t *testing.T, raw json.RawMessage) {
	t.Helper()
	wantKeys(t, "settings", raw, settingsKeys...)
	wantEachKeys(t, "host", field(t, raw, "hosts"), 1, hostKeys...)
	maps := field(t, raw, "directory_mappings")
	for _, slug := range keysOf(t, maps) {
		wantKeys(t, "mapping", field(t, maps, slug), "slug", "relative_path")
	}
	var timeout int
	if err := json.Unmarshal(field(t, raw, "api_timeout"), &timeout); err != nil || timeout != 30 {
		t.Fatalf("api_timeout = %s", field(t, raw, "api_timeout"))
	}
}

func TestEveryMethodRoundTrip(t *testing.T) {
	if got := Methods(); !reflect.DeepEqual(got, tableMethods) {
		t.Fatalf("methods = %v", got)
	}
	srv := fixtureServer(t)
	s := connectedServer(t, srv)

	cases := map[string]struct {
		args  string
		check func(t *testing.T, raw json.RawMessage)
	}{
		"status": {"", func(t *testing.T, raw json.RawMessage) {
			wantKeys(t, "status", raw, "config_source", "host", "server_version", "snapshots")
		}},
		"settings.get":      {"", checkSettings},
		"settings.set_host": {fmt.Sprintf(`{"display_name":"Home","root_uri":%q,"port":0,"username":"me","token":"secret-token","insecure_skip_verify":false}`, srv.URL()), checkSettings},
		"settings.set_directory_mapping": {`{"fs_slug":"gba","slug":"gba","relative_path":"","enabled":true}`, func(t *testing.T, raw json.RawMessage) {
			checkSettings(t, raw)
			wantKeys(t, "directory_mappings", field(t, raw, "directory_mappings"), "gba", "snes")
		}},
		"library.refresh": {`{"full":true,"scan":true}`, func(t *testing.T, raw json.RawMessage) {
			wantKeys(t, "refresh result", raw, "refresh", "scan")
			wantKeys(t, "refresh", field(t, raw, "refresh"), refreshKeys...)
			wantKeys(t, "scan", field(t, raw, "scan"), scanKeys...)
		}},
		"library.scan": {"", func(t *testing.T, raw json.RawMessage) {
			wantKeys(t, "scan", raw, scanKeys...)
		}},
		"library.platforms": {`{"all":true}`, func(t *testing.T, raw json.RawMessage) {
			wantEachKeys(t, "platform", raw, 2, platformKeys...)
		}},
		"library.games": {`{"platform_id":0,"collection":null,"search":"","filters":{},"hide_downloaded":false,"offset":0,"limit":0}`, func(t *testing.T, raw json.RawMessage) {
			wantKeys(t, "games result", raw, "total", "games")
			wantEachKeys(t, "game", field(t, raw, "games"), 3, "id", "platform_id", "name", "kind", "state")
			for _, g := range items(t, field(t, raw, "games")) {
				if st := string(field(t, g, "state")); st != `"not_downloaded"` {
					t.Fatalf("state = %s", st)
				}
				if k := string(field(t, g, "kind")); k != `"single"` {
					t.Fatalf("kind = %s", k)
				}
			}
		}},
		"library.filter_values": {`{"query":{"platform_id":1},"dim":"genre"}`, func(t *testing.T, raw json.RawMessage) {
			items(t, raw)
		}},
		"library.dims": {"", func(t *testing.T, raw json.RawMessage) {
			var dims []string
			if err := json.Unmarshal(raw, &dims); err != nil || len(dims) != len(library.Dims()) || dims[0] != "genre" {
				t.Fatalf("dims = %s", raw)
			}
		}},
		"library.collections": {"", func(t *testing.T, raw json.RawMessage) {
			wantEachKeys(t, "collection", raw, 1, "kind", "remote_id", "name", "games")
			c := items(t, raw)[0]
			if string(field(t, c, "kind")) != `"regular"` || string(field(t, c, "remote_id")) != `"7"` || string(field(t, c, "games")) != "2" {
				t.Fatalf("collection = %s", c)
			}
		}},
		"library.collection_platforms": {`{"kind":"regular","remote_id":"7"}`, func(t *testing.T, raw json.RawMessage) {
			wantEachKeys(t, "platform", raw, 2, platformKeys...)
		}},
		"library.game_state": {`{"game_id":1}`, func(t *testing.T, raw json.RawMessage) {
			wantKeys(t, "game_state", raw, "state")
			if st := string(field(t, raw, "state")); st != `"not_downloaded"` {
				t.Fatalf("state = %s", st)
			}
		}},
	}
	if len(cases) != len(tableMethods) {
		t.Fatalf("cases = %d", len(cases))
	}
	for _, m := range tableMethods {
		c, ok := cases[m]
		if !ok {
			t.Fatalf("no case for %s", m)
		}
		t.Run(m, func(t *testing.T) {
			c.check(t, mustOK(t, s, m, c.args))
		})
	}
}

func TestSettingsNeverReturnsToken(t *testing.T) {
	srv := fixtureServer(t)
	s := connectedServer(t, srv)
	raw := callRaw(t, s, "settings.get", nil, nil)
	if !strings.Contains(string(raw), `"has_token":true`) {
		t.Fatalf("settings = %s", raw)
	}
	if strings.Contains(string(raw), `"token"`) || strings.Contains(string(raw), "secret-token") {
		t.Fatalf("settings leaks token: %s", raw)
	}
}

func TestGamesPaging(t *testing.T) {
	srv := fixtureServer(t)
	s := connectedServer(t, srv)
	mustOK(t, s, "library.refresh", "")
	raw := mustOK(t, s, "library.games", `{"offset":1,"limit":1}`)
	if string(field(t, raw, "total")) != "3" {
		t.Fatalf("total = %s", field(t, raw, "total"))
	}
	games := items(t, field(t, raw, "games"))
	if len(games) != 1 || string(field(t, games[0], "id")) != "2" {
		t.Fatalf("games = %s", field(t, raw, "games"))
	}
	raw = mustOK(t, s, "library.games", `{"offset":5,"limit":1}`)
	if string(field(t, raw, "games")) != "[]" || string(field(t, raw, "total")) != "3" {
		t.Fatalf("past end = %s", raw)
	}
	wantError(t, call(t, s, "library.games", `{"offset":-1}`), CodeBadRequest)
}

func TestPartialRefreshCarriesResultAndError(t *testing.T) {
	srv := fixtureServer(t)
	s := connectedServer(t, srv)
	srv.SetFailAfter(2, 1)
	r := call(t, s, "library.refresh", "")
	if !r.OK {
		t.Fatalf("refresh = %+v", r.Error)
	}
	if got := string(field(t, field(t, r.Result, "refresh"), "failed_platforms")); got != `["snes"]` {
		t.Fatalf("failed_platforms = %s", got)
	}

	failing := fixtureServer(t)
	failing.FailPath("/api/platforms")
	s2 := connectedServer(t, failing)
	raw := callRaw(t, s2, "library.refresh", nil, nil)
	r = decode(t, raw)
	wantError(t, r, CodeServerUnreachable)
	if len(r.Result) == 0 {
		t.Fatalf("no result: %s", raw)
	}
	wantKeys(t, "refresh", field(t, r.Result, "refresh"), refreshKeys...)
}

func noResult(t *testing.T, raw []byte, code Code) {
	t.Helper()
	wantError(t, decode(t, raw), code)
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil {
		t.Fatal(err)
	}
	if _, has := keys["result"]; has {
		t.Fatalf("early failure has result: %s", raw)
	}
}

func TestEarlyRefreshFailureHasNoResult(t *testing.T) {
	noResult(t, callRaw(t, openServer(t, t.TempDir()), "library.refresh", nil, nil), CodeNotConfigured)
	noResult(t, callRaw(t, openServer(t, t.TempDir()), "library.refresh", []byte(`{"bogus":1}`), nil), CodeBadRequest)

	down := fixtureServer(t)
	s := connectedServer(t, down)
	down.FailPath("/api/heartbeat")
	noResult(t, callRaw(t, s, "library.refresh", nil, nil), CodeServerUnreachable)

	srv := fixtureServer(t)
	s = connectedServer(t, srv)
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	srv.SetBeforePage(func(int, int) {
		once.Do(func() {
			close(entered)
			<-release
		})
	})
	done := make(chan []byte, 1)
	go func() { done <- s.Call(context.Background(), "library.refresh", nil, nil) }()
	<-entered
	noResult(t, callRaw(t, s, "library.refresh", nil, nil), CodeBusy)
	close(release)
	if r := decode(t, <-done); !r.OK {
		t.Fatalf("refresh = %+v", r.Error)
	}
}

func TestRefreshEmitsProgressEvents(t *testing.T) {
	srv := fixtureServer(t)
	s := connectedServer(t, srv)
	var stages []library.Stage
	raw := callRaw(t, s, "library.refresh", []byte(`{"scan":true}`), func(e Event) {
		if e.Type != EventProgress {
			t.Errorf("event type = %q", e.Type)
		}
		if len(stages) == 0 || stages[len(stages)-1] != e.Stage {
			stages = append(stages, e.Stage)
		}
	})
	if r := decode(t, raw); !r.OK {
		t.Fatalf("refresh = %+v", r.Error)
	}
	want := []library.Stage{library.StagePlatforms, library.StageGames, library.StagePurge, library.StageCollections, library.StageScan}
	if !reflect.DeepEqual(stages, want) {
		t.Fatalf("stages = %v", stages)
	}

	b, err := json.Marshal(Event{Type: EventProgress, Stage: library.StageGames, Done: 1, Total: 3})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"type":"progress","stage":"games","done":1,"total":3}` {
		t.Fatalf("event = %s", b)
	}
}

func TestCallAfterCloseIsBadRequest(t *testing.T) {
	s, err := Open(context.Background(), testOpenOptions(t, t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	wantError(t, call(t, s, "library.platforms", ""), CodeBadRequest)
}
