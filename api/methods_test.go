package api

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"reflect"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rommapp/grout/core/auth"
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
	"auth.pair_start",
	"auth.pair_wait",
	"auth.exchange_code",
	"auth.logout",
	"device.register",
	"settings.update",
	"fetch.game",
	"fetch.art",
	"fetch.bios",
	"library.bios",
	"library.game_files",
}

var pairWaitKeys = []string{"outcome", "username", "device_id", "token_expires_at"}

func checkLoggedOutSettings(t *testing.T, raw json.RawMessage) {
	t.Helper()
	wantKeys(t, "settings", raw, settingsKeys...)
	if !strings.Contains(string(raw), `"has_token":false`) {
		t.Fatalf("settings = %s", raw)
	}
}

func approvedReply(token string) fakeromm.TokenReply {
	return fakeromm.TokenReply{Token: romm.DeviceToken{
		AccessToken: token,
		DeviceID:    "dev-9",
		Scopes:      romm.Scopes,
		ExpiresAt:   "2027-01-01T00:00:00Z",
	}}
}

func exchangeToken() romm.ClientToken {
	return romm.ClientToken{
		ID:        4,
		Name:      "living room",
		Scopes:    romm.Scopes,
		ExpiresAt: "2027-02-01T00:00:00Z",
		DeviceID:  "dev-3",
		RawToken:  "raw-1",
	}
}

func instantOptions(t *testing.T, dataDir string) OpenOptions {
	t.Helper()
	o := testOpenOptions(t, dataDir)
	o.timing = auth.Timing{Sleep: func(ctx context.Context, d time.Duration) error { return ctx.Err() }}
	return o
}

func hostServer(t *testing.T, srv *fakeromm.Server, o OpenOptions) *Server {
	t.Helper()
	if err := config.Save(o.DataDir, &config.Config{}); err != nil {
		t.Fatal(err)
	}
	s, err := Open(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	mustOK(t, s, "settings.set_host", fmt.Sprintf(`{"root_uri":%q}`, srv.URL()))
	return s
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
	return connectedServerWith(t, srv, "testfw")
}

func connectedServerWith(t *testing.T, srv *fakeromm.Server, firmwareID string) *Server {
	t.Helper()
	dir := t.TempDir()
	if err := config.Save(dir, &config.Config{ShowRegularCollections: true}); err != nil {
		t.Fatal(err)
	}
	o := instantOptions(t, dir)
	o.Firmware.ID = firmwareID
	s, err := Open(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
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
	"download_art", "unzip_downloads", "art_kind", "download_art_screenshot_preview", "download_splash_art", "additional_downloads",
}

var hostKeys = []string{"display_name", "root_uri", "port", "username", "has_token", "token_name", "token_expires_at", "device_id", "device_name", "insecure_skip_verify"}

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
	srv := biosServer(t)
	srv.SetTokenReplies(approvedReply("tok-1"))
	srv.AddExchangeCode("ABCD2345", exchangeToken())
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
	cases["auth.pair_start"] = struct {
		args  string
		check func(t *testing.T, raw json.RawMessage)
	}{`{"device_name":"slot"}`, func(t *testing.T, raw json.RawMessage) {
		wantKeys(t, "pair start", raw, "user_code", "verification_url", "expires_at")
	}}
	cases["auth.pair_wait"] = struct {
		args  string
		check func(t *testing.T, raw json.RawMessage)
	}{"", func(t *testing.T, raw json.RawMessage) {
		wantKeys(t, "pair wait", raw, pairWaitKeys...)
	}}
	cases["auth.exchange_code"] = struct {
		args  string
		check func(t *testing.T, raw json.RawMessage)
	}{`{"code":"ABCD2345"}`, func(t *testing.T, raw json.RawMessage) {
		wantKeys(t, "exchange", raw, "username", "device_id", "token_expires_at", "scopes")
	}}
	cases["device.register"] = struct {
		args  string
		check func(t *testing.T, raw json.RawMessage)
	}{`{"name":"slot"}`, func(t *testing.T, raw json.RawMessage) {
		wantKeys(t, "device", raw, "device_id", "name")
	}}
	cases["settings.update"] = struct {
		args  string
		check func(t *testing.T, raw json.RawMessage)
	}{`{"show_regular_collections":true,"show_smart_collections":false,"show_virtual_collections":false,"downloaded_games":"mark","api_timeout":30,"download_timeout":600,"log_level":"INFO","platform_order":["gba"]}`, checkSettings}
	cases["fetch.game"] = struct {
		args  string
		check func(t *testing.T, raw json.RawMessage)
	}{`{"game_id":1}`, func(t *testing.T, raw json.RawMessage) {
		wantKeys(t, "fetch.game", raw, "game_id", "file_id", "rom_dir", "launch_path", "placed", "art")
	}}
	cases["fetch.art"] = struct {
		args  string
		check func(t *testing.T, raw json.RawMessage)
	}{`{"game_id":1}`, func(t *testing.T, raw json.RawMessage) {
		wantKeys(t, "fetch.art", raw, "written", "failed")
	}}
	cases["fetch.bios"] = struct {
		args  string
		check func(t *testing.T, raw json.RawMessage)
	}{`{"platform_id":1}`, func(t *testing.T, raw json.RawMessage) {
		wantKeys(t, "fetch.bios", raw, "files")
	}}
	cases["library.bios"] = struct {
		args  string
		check func(t *testing.T, raw json.RawMessage)
	}{`{"platform_id":1}`, func(t *testing.T, raw json.RawMessage) {
		wantEachKeys(t, "bios file", raw, 1, "firmware_id", "file_name", "paths", "check", "is_verified", "present")
	}}
	cases["library.game_files"] = struct {
		args  string
		check func(t *testing.T, raw json.RawMessage)
	}{`{"game_id":1}`, func(t *testing.T, raw json.RawMessage) {
		wantEachKeys(t, "game file", raw, 1, "id", "file_name", "size_bytes", "category")
	}}
	cases["auth.logout"] = struct {
		args  string
		check func(t *testing.T, raw json.RawMessage)
	}{"", checkLoggedOutSettings}
	if len(cases) != len(tableMethods) {
		t.Fatalf("cases = %d", len(cases))
	}
	order := slices.Clone(tableMethods)
	order = slices.DeleteFunc(order, func(m string) bool { return m == "auth.logout" })
	order = append(order, "auth.logout")
	for _, m := range order {
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

func TestOpenOptionsClientReachesPairStart(t *testing.T) {
	srv := fakeromm.New(t)
	dir := t.TempDir()
	raw := fmt.Sprintf(`{"data_dir":%q,"firmware":{"id":"testfw"},"client":{"name":"slot","version":"1.4.0"},"env":{"GROUT_ROOT":%q}}`, dir, t.TempDir())
	o, err := DecodeOpenOptions([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	o.timing = instantOptions(t, dir).timing
	s := hostServer(t, srv, o)
	mustOK(t, s, "auth.pair_start", `{"device_name":"slot"}`)
	reqs := srv.InitRequests()
	if len(reqs) != 1 || reqs[0].Client != "slot" || reqs[0].ClientVersion != "1.4.0" {
		t.Fatalf("init requests = %+v", reqs)
	}
}

func TestAuthMethodsRoundTrip(t *testing.T) {
	srv := fakeromm.New(t)
	srv.SetTokenReplies(approvedReply("tok-1"))
	s := hostServer(t, srv, instantOptions(t, t.TempDir()))

	start := mustOK(t, s, "auth.pair_start", `{"device_name":"slot"}`)
	wantKeys(t, "pair start", start, "user_code", "verification_url", "expires_at")
	var expires string
	if err := json.Unmarshal(field(t, start, "expires_at"), &expires); err != nil {
		t.Fatal(err)
	}
	if _, err := time.Parse(time.RFC3339, expires); err != nil {
		t.Fatalf("expires_at = %q", expires)
	}
	wait := mustOK(t, s, "auth.pair_wait", "")
	wantKeys(t, "pair wait", wait, pairWaitKeys...)
	if string(field(t, wait, "outcome")) != `"approved"` || string(field(t, wait, "device_id")) != `"dev-9"` {
		t.Fatalf("pair wait = %s", wait)
	}

	wantError(t, call(t, s, "auth.exchange_code", `{"code":"WRONG234"}`), CodeInvalidCode)

	reg := mustOK(t, s, "device.register", `{"name":"slot"}`)
	wantKeys(t, "device", reg, "device_id", "name")
	if string(field(t, reg, "name")) != `"slot"` {
		t.Fatalf("device = %s", reg)
	}

	out := mustOK(t, s, "auth.logout", "")
	checkLoggedOutSettings(t, out)
}

func TestPairWaitDeniedIsAResult(t *testing.T) {
	srv := fakeromm.New(t)
	srv.SetTokenReplies(fakeromm.TokenReply{State: romm.PairDenied})
	s := hostServer(t, srv, instantOptions(t, t.TempDir()))
	mustOK(t, s, "auth.pair_start", `{"device_name":"slot"}`)
	wait := mustOK(t, s, "auth.pair_wait", "")
	if string(field(t, wait, "outcome")) != `"denied"` {
		t.Fatalf("pair wait = %s", wait)
	}
}

func TestSettingsUpdatePartialAndNullRejected(t *testing.T) {
	srv := fakeromm.New(t)
	s := hostServer(t, srv, instantOptions(t, t.TempDir()))
	before := mustOK(t, s, "settings.get", "")
	after := mustOK(t, s, "settings.update", `{"show_regular_collections":true}`)
	var b, a map[string]json.RawMessage
	if err := json.Unmarshal(before, &b); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(after, &a); err != nil {
		t.Fatal(err)
	}
	for k, v := range a {
		same := string(v) == string(b[k])
		if k == "show_regular_collections" {
			if string(v) != "true" {
				t.Fatalf("show_regular_collections = %s", v)
			}
			continue
		}
		if !same {
			t.Fatalf("%s changed from %s to %s", k, b[k], v)
		}
	}
	wantError(t, call(t, s, "settings.update", `{"log_level":null}`), CodeBadRequest)
	wantError(t, call(t, s, "settings.update", `{"api_timeout":0}`), CodeBadRequest)
	wantError(t, call(t, s, "settings.update", `{"bogus":1}`), CodeBadRequest)
	wantError(t, call(t, s, "settings.update", `{"log_level":"LOUD"}`), CodeBadRequest)
	wantError(t, call(t, s, "settings.update", `{"downloaded_games":"nope"}`), CodeBadRequest)
	wantError(t, call(t, s, "settings.update", `[1]`), CodeBadRequest)
	got := mustOK(t, s, "settings.update", `{"api_timeout":45,"log_level":"DEBUG","downloaded_games":"filter","platform_order":["snes","gba"]}`)
	if string(field(t, got, "api_timeout")) != "45" || string(field(t, got, "log_level")) != `"DEBUG"` ||
		string(field(t, got, "downloaded_games")) != `"filter"` || string(field(t, got, "platform_order")) != `["snes","gba"]` {
		t.Fatalf("settings = %s", got)
	}
}

func TestSettingsHostInfoNewFieldsNoToken(t *testing.T) {
	srv := fakeromm.New(t)
	srv.SetTokenReplies(approvedReply("tok-secret-1"))
	s := hostServer(t, srv, instantOptions(t, t.TempDir()))
	mustOK(t, s, "auth.pair_start", `{"device_name":"slot"}`)
	mustOK(t, s, "auth.pair_wait", "")
	raw := mustOK(t, s, "settings.get", "")
	host := items(t, field(t, raw, "hosts"))[0]
	wantKeys(t, "host", host, hostKeys...)
	if string(field(t, host, "token_name")) != `"slot"` || string(field(t, host, "device_id")) != `"dev-9"` ||
		string(field(t, host, "device_name")) != `"slot"` || string(field(t, host, "token_expires_at")) != `"2027-01-01T00:00:00Z"` ||
		string(field(t, host, "has_token")) != "true" {
		t.Fatalf("host = %s", host)
	}
	if strings.Contains(string(raw), "tok-secret-1") || strings.Contains(string(raw), `"token"`) {
		t.Fatalf("settings leaks token: %s", raw)
	}
}

func TestWireEnumListsAreHandled(t *testing.T) {
	for _, m := range DownloadedGamesModes() {
		c, err := m.mode()
		if err != nil || toDownloadedGamesMode(c) != m {
			t.Fatalf("mode %q", m)
		}
	}
	for _, l := range LogLevels() {
		c, err := l.level()
		if err != nil || toLogLevel(c) != l {
			t.Fatalf("level %q", l)
		}
	}
	for _, o := range auth.Outcomes() {
		if !slices.Contains(PairOutcomes(), toPairOutcome(o)) || string(toPairOutcome(o)) != string(o) {
			t.Fatalf("outcome %q", o)
		}
	}
}

func TestPairWaitFailedIsServerUnreachable(t *testing.T) {
	srv := fakeromm.New(t)
	srv.FailTimes("/api/auth/device/token", 3)
	s := hostServer(t, srv, instantOptions(t, t.TempDir()))
	mustOK(t, s, "auth.pair_start", `{"device_name":"slot"}`)
	noResult(t, callRaw(t, s, "auth.pair_wait", nil, nil), CodeServerUnreachable)
}

func TestSetHostSameURLKeepsToken(t *testing.T) {
	srv := fakeromm.New(t)
	s := hostServer(t, srv, instantOptions(t, t.TempDir()))
	mustOK(t, s, "settings.set_host", fmt.Sprintf(`{"root_uri":%q,"token":"secret-token"}`, srv.URL()))
	raw := mustOK(t, s, "settings.set_host", fmt.Sprintf(`{"display_name":"Den","root_uri":%q,"insecure_skip_verify":true}`, srv.URL()))
	host := items(t, field(t, raw, "hosts"))[0]
	if string(field(t, host, "has_token")) != "true" || string(field(t, host, "display_name")) != `"Den"` {
		t.Fatalf("host = %s", host)
	}
}

func downloadServer(t *testing.T) *fakeromm.Server {
	t.Helper()
	srv := fixtureServer(t)
	srv.Content = map[int][]byte{}
	srv.Assets = map[string][]byte{}
	for i := range srv.Roms {
		r := &srv.Roms[i]
		body := []byte(fmt.Sprintf("rom data %d", r.ID))
		r.FSName = fmt.Sprintf("game%d.gba", r.ID)
		r.FSNameNoExt = fmt.Sprintf("game%d", r.ID)
		r.FSExtension = "gba"
		r.HasSimpleSingleFile = true
		r.Files = []romm.RomFile{{ID: 100 + r.ID, FileName: r.FSName, FileSizeBytes: int64(len(body))}}
		srv.Content[r.ID] = body
	}
	return srv
}

func refreshed(t *testing.T, srv *fakeromm.Server) *Server {
	t.Helper()
	s := connectedServer(t, srv)
	mustOK(t, s, "library.refresh", "")
	return s
}

func TestFetchGameMethod(t *testing.T) {
	s := refreshed(t, downloadServer(t))
	raw := mustOK(t, s, "fetch.game", `{"game_id":1}`)
	wantKeys(t, "fetch.game", raw, "game_id", "file_id", "rom_dir", "launch_path", "placed", "art")
	wantKeys(t, "art", field(t, raw, "art"), "written", "failed")
	if string(field(t, raw, "game_id")) != "1" || string(field(t, raw, "file_id")) != "101" {
		t.Fatalf("result = %s", raw)
	}
	if len(items(t, field(t, raw, "placed"))) != 1 {
		t.Fatalf("placed = %s", field(t, raw, "placed"))
	}
	art := field(t, raw, "art")
	if string(field(t, art, "written")) != "[]" || string(field(t, art, "failed")) != "[]" {
		t.Fatalf("art = %s", art)
	}
	if string(field(t, mustOK(t, s, "library.game_state", `{"game_id":1}`), "state")) != `"downloaded"` {
		t.Fatal("game not downloaded")
	}
	raw = mustOK(t, s, "fetch.game", `{"game_id":2,"file_id":102}`)
	if string(field(t, raw, "file_id")) != "102" {
		t.Fatalf("result = %s", raw)
	}
}

func TestFetchGameArgsValidated(t *testing.T) {
	s := refreshed(t, downloadServer(t))
	for _, args := range []string{``, `{}`, `{"game_id":0}`, `{"game_id":-3}`, `{"game_id":1,"file_id":-1}`} {
		wantError(t, call(t, s, "fetch.game", args), CodeBadRequest)
	}
	wantError(t, call(t, s, "fetch.game", `{"game_id":1,"bogus":1}`), CodeBadRequest)
	wantError(t, call(t, s, "fetch.art", `{}`), CodeBadRequest)
	wantError(t, call(t, s, "fetch.bios", `{"platform_id":0}`), CodeBadRequest)
	wantError(t, call(t, s, "library.bios", `{}`), CodeBadRequest)
	wantError(t, call(t, s, "library.game_files", `{"game_id":0}`), CodeBadRequest)
	wantError(t, call(t, s, "fetch.game", `{"game_id":999}`), CodeBadRequest)
}

func zipBody(t *testing.T, names ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, n := range names {
		f, err := zw.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte("data " + n)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func coverPNG(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 8, 8))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestFetchGameEvents(t *testing.T) {
	srv := downloadServer(t)
	const cover = "/assets/romm/resources/roms/1/cover/small.png"
	body := zipBody(t, "disc1.bin", "disc2.bin")
	r := &srv.Roms[0]
	r.FSName, r.FSNameNoExt, r.FSExtension = "Game v1.2", "Game v1.2", ""
	r.HasSimpleSingleFile, r.HasMultipleFiles = false, true
	r.PathCoverSmall = cover
	r.Files = []romm.RomFile{{ID: 101, FileName: "disc1.bin", FileSizeBytes: 15}, {ID: 102, FileName: "disc2.bin", FileSizeBytes: 15}}
	srv.Content[1] = body
	srv.Assets[cover] = coverPNG(t)
	s := connectedServerWith(t, srv, "knulli")
	mustOK(t, s, "library.refresh", "")
	mustOK(t, s, "settings.update", `{"unzip_downloads":true}`)
	mustOK(t, s, "settings.update", `{"download_art":true}`)
	var events []Event
	raw := callRaw(t, s, "fetch.game", []byte(`{"game_id":1}`), func(e Event) { events = append(events, e) })
	if r := decode(t, raw); !r.OK {
		t.Fatalf("reply = %s", raw)
	}
	var order []string
	for _, e := range events {
		if len(order) == 0 || order[len(order)-1] != string(e.Stage) {
			order = append(order, string(e.Stage))
		}
		if e.Stage == library.StageDownload && (e.BytesDone == nil || e.BytesTotal == nil) {
			t.Fatalf("download event without bytes: %+v", e)
		}
		if e.Stage != library.StageDownload && e.Stage != library.StageBios && (e.BytesDone != nil || e.BytesTotal != nil) {
			t.Fatalf("event %+v carries bytes", e)
		}
	}
	if !reflect.DeepEqual(order, []string{"download", "extract", "art"}) {
		t.Fatalf("stage order = %v", order)
	}
	last := events[len(events)-1]
	if last.Stage != library.StageArt {
		t.Fatalf("last event = %+v", last)
	}
	var done *Event
	for i := range events {
		if events[i].Stage == library.StageDownload {
			done = &events[i]
		}
	}
	if *done.BytesDone != int64(len(body)) {
		t.Fatalf("final download event = %+v", done)
	}
	b, _ := json.Marshal(*done)
	if !strings.Contains(string(b), fmt.Sprintf(`"bytes_done":%d`, len(body))) || !strings.Contains(string(b), `"bytes_total"`) {
		t.Fatalf("event json = %s", b)
	}
}

func TestEventBytesOmittedForCounts(t *testing.T) {
	b, _ := json.Marshal(Event{Type: EventProgress, Stage: library.StageGames, Done: 3, Total: 9})
	if string(b) != `{"type":"progress","stage":"games","done":3,"total":9}` {
		t.Fatalf("event = %s", b)
	}
	var got []Event
	emit := progress(func(e Event) { got = append(got, e) })
	emit(library.Progress{Stage: library.StageGames, Done: 1, Total: 2, BytesDone: 5, BytesTotal: 6})
	emit(library.Progress{Stage: library.StageArt, Done: 1, Total: 2})
	emit(library.Progress{Stage: library.StageBios, Done: 1, Total: 2, BytesDone: 5, BytesTotal: 6})
	emit(library.Progress{Stage: library.StageDownload, Done: 1, Total: 2, BytesDone: 5})
	if got[0].BytesDone != nil || got[1].BytesDone != nil {
		t.Fatalf("count events carry bytes: %+v", got)
	}
	if got[2].BytesDone == nil || *got[2].BytesTotal != 6 || got[3].BytesTotal == nil || *got[3].BytesTotal != 0 {
		t.Fatalf("byte events = %+v", got[2:])
	}
}

func TestFetchArtMethod(t *testing.T) {
	s := refreshed(t, downloadServer(t))
	wantError(t, call(t, s, "fetch.art", `{"game_id":1}`), CodeBadRequest)
	mustOK(t, s, "fetch.game", `{"game_id":1}`)
	raw := mustOK(t, s, "fetch.art", `{"game_id":1}`)
	wantKeys(t, "fetch.art", raw, "written", "failed")
}

func biosServer(t *testing.T) *fakeromm.Server {
	t.Helper()
	srv := downloadServer(t)
	body := []byte("gba bios bytes")
	sum := md5.Sum(body)
	srv.FirmwareList = []romm.Firmware{{ID: 5, PlatformID: 1, FileName: "gba_bios.bin", SizeBytes: int64(len(body)), MD5: hex.EncodeToString(sum[:]), IsVerified: true}}
	srv.FirmwareContent = map[int][]byte{5: body}
	return srv
}

func TestFetchBiosMethod(t *testing.T) {
	s := refreshed(t, biosServer(t))
	var events []Event
	raw := callRaw(t, s, "fetch.bios", []byte(`{"platform_id":1}`), func(e Event) { events = append(events, e) })
	r := decode(t, raw)
	if !r.OK {
		t.Fatalf("reply = %s", raw)
	}
	wantKeys(t, "fetch.bios", r.Result, "files")
	files := items(t, field(t, r.Result, "files"))
	if len(files) != 1 {
		t.Fatalf("files = %s", r.Result)
	}
	wantKeys(t, "bios file", files[0], "firmware_id", "file_name", "paths", "check", "is_verified", "present")
	if string(field(t, files[0], "present")) != "true" || string(field(t, files[0], "check")) != `"verified"` {
		t.Fatalf("file = %s", files[0])
	}
	sawBios := false
	for _, e := range events {
		if e.Stage == library.StageBios {
			sawBios = true
		}
	}
	if !sawBios {
		t.Fatalf("events = %+v", events)
	}
	wantError(t, call(t, s, "fetch.bios", `{"platform_id":42}`), CodeBadRequest)
}

func TestFetchBiosErrorHasNoResult(t *testing.T) {
	srv := biosServer(t)
	srv.FailPath(romm.FirmwareContentPath(5, "gba_bios.bin"))
	s := refreshed(t, srv)
	noResult(t, callRaw(t, s, "fetch.bios", []byte(`{"platform_id":1}`), nil), CodeServerUnreachable)
}

func TestLibraryBiosMethod(t *testing.T) {
	s := refreshed(t, biosServer(t))
	raw := mustOK(t, s, "library.bios", `{"platform_id":1}`)
	files := items(t, raw)
	if len(files) != 1 || string(field(t, files[0], "present")) != "false" {
		t.Fatalf("bios = %s", raw)
	}
	mustOK(t, s, "fetch.bios", `{"platform_id":1}`)
	files = items(t, mustOK(t, s, "library.bios", `{"platform_id":1}`))
	if string(field(t, files[0], "present")) != "true" {
		t.Fatalf("bios = %s", files[0])
	}
	empty := mustOK(t, s, "library.bios", `{"platform_id":2}`)
	if string(empty) != "[]" {
		t.Fatalf("empty bios = %s", empty)
	}
}

func TestLibraryGameFilesMethod(t *testing.T) {
	srv := downloadServer(t)
	s := refreshed(t, srv)
	raw := mustOK(t, s, "library.game_files", `{"game_id":1}`)
	files := items(t, raw)
	if len(files) != 1 {
		t.Fatalf("files = %s", raw)
	}
	wantKeys(t, "game file", files[0], "id", "file_name", "size_bytes", "category")
	if string(field(t, files[0], "file_name")) != `"game1.gba"` {
		t.Fatalf("file = %s", files[0])
	}
	wantError(t, call(t, s, "library.game_files", `{"game_id":999}`), CodeBadRequest)
}

func TestSettingsUpdateArtFields(t *testing.T) {
	s := refreshed(t, downloadServer(t))
	got := mustOK(t, s, "settings.update", `{"download_art":true,"unzip_downloads":true,"art_kind":"Box3D","download_art_screenshot_preview":true,"download_splash_art":"Title","additional_downloads":{"marquee":"Logo","video":true,"thumbnail":"Marquee","bezel":true,"manual":true,"boxback":true,"fanart":true}}`)
	for k, want := range map[string]string{
		"download_art": "true", "unzip_downloads": "true", "art_kind": `"Box3D"`,
		"download_art_screenshot_preview": "true", "download_splash_art": `"Title"`,
	} {
		if string(field(t, got, k)) != want {
			t.Fatalf("%s = %s, want %s", k, field(t, got, k), want)
		}
	}
	ad := field(t, got, "additional_downloads")
	wantKeys(t, "additional_downloads", ad, "marquee", "video", "thumbnail", "bezel", "manual", "boxback", "fanart")
	if string(field(t, ad, "marquee")) != `"Logo"` || string(field(t, ad, "thumbnail")) != `"Marquee"` || string(field(t, ad, "boxback")) != "true" {
		t.Fatalf("additional_downloads = %s", ad)
	}
	got = mustOK(t, s, "settings.update", `{"additional_downloads":{"video":false}}`)
	ad = field(t, got, "additional_downloads")
	if string(field(t, ad, "video")) != "false" || string(field(t, ad, "marquee")) != `"Logo"` {
		t.Fatalf("partial additional_downloads = %s", ad)
	}
	wantError(t, call(t, s, "settings.update", `{"additional_downloads":{"bogus":true}}`), CodeBadRequest)
	wantError(t, call(t, s, "settings.update", `{"additional_downloads":null}`), CodeBadRequest)
	wantError(t, call(t, s, "settings.update", `{"additional_downloads":{"video":null}}`), CodeBadRequest)
	wantError(t, call(t, s, "settings.update", `{"additional_downloads":{"marquee":"Nope"}}`), CodeBadRequest)
	wantError(t, call(t, s, "settings.update", `{"art_kind":"Nope"}`), CodeBadRequest)
	wantError(t, call(t, s, "settings.update", `{"download_splash_art":"Logo"}`), CodeBadRequest)
	wantError(t, call(t, s, "settings.update", `{"download_art":null}`), CodeBadRequest)
}

func TestSettingsGetReportsArtFields(t *testing.T) {
	s := refreshed(t, downloadServer(t))
	raw := mustOK(t, s, "settings.get", "")
	wantKeys(t, "settings", raw, settingsKeys...)
	wantKeys(t, "additional_downloads", field(t, raw, "additional_downloads"), "marquee", "video", "thumbnail", "bezel", "manual", "boxback", "fanart")
}
