package grout

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rommapp/grout/core/firmware"
	"github.com/rommapp/grout/core/gamelist"
	"github.com/rommapp/grout/core/romm"
	"github.com/rommapp/grout/test/fakeromm"
)

func (e *fetchEnv) write(t *testing.T, p string, data []byte) {
	t.Helper()
	real, err := e.w.Real(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(real), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(real, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func firedOf(t *testing.T, g *Grout) []firmware.ExitCondition {
	t.Helper()
	got, err := g.snapshot().FiredConditions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return got.Conditions
}

func wantFired(t *testing.T, g *Grout, want ...firmware.ExitCondition) {
	t.Helper()
	got := firedOf(t, g)
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fired = %v, want %v", got, want)
	}
}

func clearFired(t *testing.T, g *Grout) {
	t.Helper()
	l := g.snapshot()
	read, err := l.FiredConditions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := l.ClearConditions(context.Background(), read); err != nil {
		t.Fatal(err)
	}
}

func (e *fetchEnv) gamelistPath(t *testing.T) string {
	t.Helper()
	return path.Join(e.romDir(t), "gamelist.xml")
}

func TestFetchGameWritesGamelist(t *testing.T) {
	e := newFetchEnv(t, "knulli")
	res, err := e.g.FetchGame(context.Background(), 1, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	file := e.gamelistPath(t)
	if !reflect.DeepEqual(res.Gamelist.Written, []string{file}) || len(res.Gamelist.Failed) != 0 {
		t.Fatalf("gamelist = %+v", res.Gamelist)
	}
	body := string(e.read(t, file))
	if !strings.Contains(body, "<path>./game1.gba</path>") || !strings.Contains(body, "<name>game 1</name>") {
		t.Fatalf("gamelist.xml = %s", body)
	}
	wantFired(t, e.g, firmware.CondGamelistChanged, firmware.CondGamesChanged)
}

func TestFetchGameGamelistFailureDoesNotFail(t *testing.T) {
	e := newFetchEnv(t, "knulli")
	file := e.gamelistPath(t)
	broken := []byte("<gameList>\n  <game>\n    <path>./other.gba</pa")
	e.write(t, file, broken)
	res, err := e.g.FetchGame(context.Background(), 1, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(e.read(t, path.Join(e.romDir(t), "game1.gba"))) != "rom data 1" {
		t.Fatal("game not placed")
	}
	if got := e.read(t, file); string(got) != string(broken) {
		t.Fatalf("unparseable gamelist changed: %q", got)
	}
	if len(res.Gamelist.Written) != 0 || len(res.Gamelist.Failed) != 1 || res.Gamelist.Failed[0].GameID != 1 || res.Gamelist.Failed[0].Message == "" {
		t.Fatalf("gamelist = %+v", res.Gamelist)
	}
	wantFired(t, e.g, firmware.CondGamesChanged)
	noStagingUnder(t, e.w)
}

func TestFetchGameTwoGamesSameGamelist(t *testing.T) {
	e := newFetchEnv(t, "knulli")
	e.srv.SetContentDelay(20 * time.Millisecond)
	first, second := fetchAsync(e.g, 1), fetchAsync(e.g, 2)
	for _, ch := range []<-chan fetchOutcome{first, second} {
		out := within(t, ch, "FetchGame")
		if out.err != nil {
			t.Fatal(out.err)
		}
		if len(out.res.Gamelist.Failed) != 0 {
			t.Fatalf("gamelist = %+v", out.res.Gamelist)
		}
	}
	body := string(e.read(t, e.gamelistPath(t)))
	for _, p := range []string{"<path>./game1.gba</path>", "<path>./game2.gba</path>"} {
		if strings.Count(body, p) != 1 {
			t.Fatalf("%s appears %d times in %s", p, strings.Count(body, p), body)
		}
	}
	noStagingUnder(t, e.w)
}

func TestFetchGameNoneFormatFiresOnlyGamesChanged(t *testing.T) {
	e := newFetchEnv(t, "nextui")
	res, err := e.g.FetchGame(context.Background(), 1, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Gamelist.Written == nil || res.Gamelist.Failed == nil || len(res.Gamelist.Written)+len(res.Gamelist.Failed) != 0 {
		t.Fatalf("gamelist = %#v", res.Gamelist)
	}
	wantFired(t, e.g, firmware.CondGamesChanged)
}

func TestFetchGameTwiceFiresGamelistOnce(t *testing.T) {
	ctx := context.Background()
	e := newFetchEnv(t, "knulli")
	if _, err := e.g.FetchGame(ctx, 1, 0, nil); err != nil {
		t.Fatal(err)
	}
	clearFired(t, e.g)
	res, err := e.g.FetchGame(ctx, 1, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Gamelist.Written) != 0 || len(res.Gamelist.Failed) != 0 {
		t.Fatalf("gamelist = %+v", res.Gamelist)
	}
	if n := strings.Count(string(e.read(t, e.gamelistPath(t))), "<game>"); n != 1 {
		t.Fatalf("%d entries", n)
	}
	wantFired(t, e.g, firmware.CondGamesChanged)
}

func TestFetchArtUpdatesGamelist(t *testing.T) {
	ctx := context.Background()
	e := newFetchEnv(t, "knulli")
	e.srv.Mutate(func() {
		e.srv.Roms[0].PathCoverSmall = coverAsset
		e.srv.Assets[coverAsset] = pngData(t)
	})
	if _, err := e.g.FetchGame(ctx, 1, 0, nil); err != nil {
		t.Fatal(err)
	}
	file := e.gamelistPath(t)
	if strings.Contains(string(e.read(t, file)), "<image>") {
		t.Fatal("image written without art")
	}
	clearFired(t, e.g)
	if _, err := e.g.UpdateSettings(SettingsPatch{DownloadArt: ptr(true)}); err != nil {
		t.Fatal(err)
	}
	res, err := e.g.FetchArt(ctx, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Written) != 1 {
		t.Fatalf("art = %+v", res.ArtResult)
	}
	if !reflect.DeepEqual(res.Gamelist.Written, []string{file}) || len(res.Gamelist.Failed) != 0 {
		t.Fatalf("gamelist = %+v", res.Gamelist)
	}
	want := "<image>./images/" + e.g.fw.ArtFileName(firmware.ArtCover, "game1") + "</image>"
	if body := string(e.read(t, file)); !strings.Contains(body, want) {
		t.Fatalf("gamelist.xml = %s, want %s", body, want)
	}
	wantFired(t, e.g, firmware.CondGamelistChanged)

	clearFired(t, e.g)
	res, err = e.g.FetchArt(ctx, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Gamelist.Written == nil || len(res.Gamelist.Written) != 0 || len(res.Gamelist.Failed) != 0 {
		t.Fatalf("gamelist = %#v", res.Gamelist)
	}
	wantFired(t, e.g)
}

func gamelistServer(t *testing.T) *fakeromm.Server {
	t.Helper()
	srv := fetchServer(t)
	srv.Platforms = append(srv.Platforms, romm.Platform{ID: 2, Slug: "psx", FSSlug: "psx", Name: "PlayStation", UpdatedAt: srv.Platforms[0].UpdatedAt})
	return srv
}

func TestGamelistRefreshAllAndPlatform(t *testing.T) {
	ctx := context.Background()
	e := newFetchEnvWith(t, "knulli", gamelistServer(t), rootedHost(t))
	for id := 1; id <= 2; id++ {
		if _, err := e.g.FetchGame(ctx, id, 0, nil); err != nil {
			t.Fatal(err)
		}
	}
	file := e.gamelistPath(t)
	e.remove(t, file)
	clearFired(t, e.g)
	if err := e.g.Logout(); err != nil {
		t.Fatal(err)
	}

	res, err := e.g.GamelistRefresh(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if res.Games != 2 || !reflect.DeepEqual(res.Written, []string{file}) || len(res.Failed) != 0 {
		t.Fatalf("refresh = %+v", res)
	}
	body := string(e.read(t, file))
	if !strings.Contains(body, "<path>./game1.gba</path>") || !strings.Contains(body, "<path>./game2.gba</path>") {
		t.Fatalf("gamelist.xml = %s", body)
	}
	wantFired(t, e.g, firmware.CondGamelistChanged)

	clearFired(t, e.g)
	res, err = e.g.GamelistRefresh(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if res.Games != 2 || res.Written == nil || len(res.Written) != 0 || res.Failed == nil || len(res.Failed) != 0 {
		t.Fatalf("refresh = %#v", res)
	}
	wantFired(t, e.g)

	res, err = e.g.GamelistRefresh(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	if res.Games != 0 || len(res.Written) != 0 || len(res.Failed) != 0 {
		t.Fatalf("refresh = %+v", res)
	}
	if len(e.srv.CallsTo("/api/roms/1")) != 0 {
		t.Fatal("server called")
	}
}

func TestGamelistRefreshBusyDuringRefresh(t *testing.T) {
	ctx := context.Background()
	e := newFetchEnv(t, "knulli")
	gt := block(t, e.srv)
	done := refreshAsync(e.g, ctx)
	<-gt.entered
	_, err := e.g.GamelistRefresh(ctx, 0)
	wantCode(t, err, CodeBusy)
	wantMessage(t, err, "a refresh, scan or download is running")
	gt.unblock()
	if out := <-done; out.err != nil {
		t.Fatal(out.err)
	}

	hold := e.hold(t, 1)
	fetched := fetchAsync(e.g, 1)
	<-hold.entered
	if _, err := e.g.GamelistRefresh(ctx, 0); err != nil {
		t.Fatal(err)
	}
	hold.unblock()
	if out := within(t, fetched, "FetchGame"); out.err != nil {
		t.Fatal(out.err)
	}

	e.g.fetchMu.Lock()
	e.g.fetching["gamelist"] = true
	e.g.fetchMu.Unlock()
	_, err = e.g.GamelistRefresh(ctx, 0)
	wantCode(t, err, CodeBusy)
	e.g.fetchMu.Lock()
	delete(e.g.fetching, "gamelist")
	e.g.fetchMu.Unlock()
}

func TestGamelistRefreshUnknownPlatform(t *testing.T) {
	e := newFetchEnv(t, "knulli")
	_, err := e.g.GamelistRefresh(context.Background(), 42)
	wantCode(t, err, CodeBadRequest)
}

func TestGamelistRefreshReadOnlyHost(t *testing.T) {
	e := newFetchEnvWith(t, "knulli", fetchServer(t), readOnlyHost{rootedHost(t)})
	_, err := e.g.GamelistRefresh(context.Background(), 0)
	wantCode(t, err, CodeInternal)
	wantMessage(t, err, "host is read-only")
}

func TestGamelistRefreshSkipsFolderGames(t *testing.T) {
	ctx := context.Background()
	srv := fetchServer(t)
	r := &srv.Roms[0]
	r.FSName, r.FSNameNoExt, r.FSExtension = "Game", "Game", ""
	r.HasSimpleSingleFile, r.HasMultipleFiles = false, true
	r.Files = []romm.RomFile{{ID: 101, FileName: "disc1.bin", FileSizeBytes: 15}, {ID: 102, FileName: "disc2.bin", FileSizeBytes: 15}}
	srv.Content[1] = zipOf(t, "disc1.bin", "disc2.bin")
	e := newFetchEnvWith(t, "knulli", srv, rootedHost(t))
	first, err := e.g.FetchGame(ctx, 1, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	folder := path.Join(e.romDir(t), "Game")
	if first.LaunchPath != folder {
		t.Fatalf("launch = %q", first.LaunchPath)
	}
	if len(first.Gamelist.Failed) != 1 || !strings.Contains(first.Gamelist.Failed[0].Message, gamelist.ErrNoPlaylist.Error()) {
		t.Fatalf("fetch gamelist = %+v", first.Gamelist)
	}
	wantFired(t, e.g, firmware.CondGamesChanged)
	if _, err := e.g.FetchGame(ctx, 2, 0, nil); err != nil {
		t.Fatal(err)
	}
	file := e.gamelistPath(t)
	e.remove(t, file)

	res, err := e.g.GamelistRefresh(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if res.Games != 2 || !reflect.DeepEqual(res.Written, []string{file}) || len(res.Failed) != 1 {
		t.Fatalf("refresh = %+v", res)
	}
	f := res.Failed[0]
	if f.GameID != 1 || f.Path != folder || !strings.Contains(f.Message, gamelist.ErrNoPlaylist.Error()) {
		t.Fatalf("failure = %+v", f)
	}
	body := string(e.read(t, file))
	if strings.Contains(body, "./Game<") || !strings.Contains(body, "<path>./game2.gba</path>") {
		t.Fatalf("gamelist.xml = %s", body)
	}
}

func TestOpenWritesLauncherEntryWhenAsked(t *testing.T) {
	h := rootedHost(t)
	dataDir := t.TempDir()
	o := Options{DataDir: dataDir, FirmwareID: "knulli", Host: h, LauncherEntry: true}
	g, err := Open(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	le, ok := g.fw.LauncherEntry()
	if !ok {
		t.Fatal("knulli has no launcher entry")
	}
	data, err := h.ReadFile(le.Gamelist)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "<name>Grout</name>") || !strings.Contains(string(data), "<path>"+le.Path+"</path>") {
		t.Fatalf("launcher gamelist = %s", data)
	}
	wantFired(t, g, firmware.CondGamelistChanged)
	clearFired(t, g)
	if err := g.Close(); err != nil {
		t.Fatal(err)
	}

	again := open(t, o)
	wantFired(t, again)
	same, err := h.ReadFile(le.Gamelist)
	if err != nil || string(same) != string(data) {
		t.Fatalf("rewritten: %q %v", same, err)
	}
}

func TestOpenLauncherEntryFailureDoesNotFailOpen(t *testing.T) {
	h := rootedHost(t)
	fw, err := firmware.Load("knulli")
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := firmware.Resolve(fw, h, firmware.Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	le, ok := resolved.LauncherEntry()
	if !ok {
		t.Fatal("knulli has no launcher entry")
	}
	broken := []byte("<gameList><game>")
	real, err := h.Real(le.Gamelist)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(real), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(real, broken, 0o644); err != nil {
		t.Fatal(err)
	}
	g := open(t, Options{DataDir: t.TempDir(), FirmwareID: "knulli", Host: h, LauncherEntry: true})
	if got, _ := os.ReadFile(real); string(got) != string(broken) {
		t.Fatalf("broken launcher gamelist changed: %q", got)
	}
	wantFired(t, g)
}

func TestOpenLauncherNotAsked(t *testing.T) {
	h := rootedHost(t)
	g := open(t, Options{DataDir: t.TempDir(), FirmwareID: "knulli", Host: h})
	le, ok := g.fw.LauncherEntry()
	if !ok {
		t.Fatal("knulli has no launcher entry")
	}
	if _, err := h.Stat(le.Gamelist); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("launcher gamelist written: %v", err)
	}
	wantFired(t, g)

	ro := open(t, Options{DataDir: t.TempDir(), FirmwareID: "knulli", Host: readOnlyHost{rootedHost(t)}, LauncherEntry: true})
	wantFired(t, ro)
}
