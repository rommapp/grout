package fetch

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/rommapp/grout/core/firmware"
	"github.com/rommapp/grout/core/firmware/host"
	"github.com/rommapp/grout/core/library"
	"github.com/rommapp/grout/core/romm"
	"github.com/rommapp/grout/test/fakeromm"
)

type gameEnv struct {
	w      host.Writable
	root   string
	srv    *fakeromm.Server
	fw     *firmware.Firmware
	romDir string
}

func newGameEnv(t *testing.T, fwID, slug string) *gameEnv {
	t.Helper()
	def, err := firmware.Load(fwID)
	if err != nil {
		t.Fatal(err)
	}
	fw, err := firmware.Resolve(def, &host.Fake{Files: fstest.MapFS{}}, firmware.Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	dirs := fw.RomDirs(slug)
	if len(dirs) == 0 {
		t.Fatalf("no rom dir for %s on %s", slug, fwID)
	}
	w, root := newHost(t)
	srv := fakeromm.New(t)
	srv.Content = map[int][]byte{}
	return &gameEnv{w: w, root: root, srv: srv, fw: fw, romDir: dirs[0]}
}

func (e *gameEnv) info(kind library.GameKind, files ...library.GameFile) library.FetchInfo {
	return library.FetchInfo{
		GameID:      1,
		Name:        "Game",
		Kind:        kind,
		FSName:      "Game",
		FSNameNoExt: "Game",
		Platform:    library.Platform{ID: 7, Slug: "gba", FSSlug: "gba", Name: "GBA", RomDirs: []string{e.romDir}},
		Files:       files,
	}
}

func (e *gameEnv) input(info library.FetchInfo) GameInput {
	return GameInput{Info: info, Client: e.srv.Client, Firmware: e.fw, Host: e.w, Limits: DefaultLimits()}
}

type recorder struct {
	mu     sync.Mutex
	events []Progress
}

func (r *recorder) add(p Progress) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, p)
}

func (r *recorder) stage(s library.Stage) []Progress {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []Progress
	for _, p := range r.events {
		if p.Stage == s {
			out = append(out, p)
		}
	}
	return out
}

func zipBytes(t *testing.T, entries ...zipEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		fw, err := zw.CreateHeader(&zip.FileHeader{Name: e.name, Method: zip.Deflate})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(e.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func noStaging(t *testing.T, root string) {
	t.Helper()
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasPrefix(d.Name(), StagePrefix) {
			t.Errorf("staging left behind: %s", p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func emptyDir(t *testing.T, root, p string) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(p)))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("%s not empty: %v", p, names)
	}
}

func contentQuery(t *testing.T, srv *fakeromm.Server, p string) url.Values {
	t.Helper()
	calls := srv.CallsTo(p)
	if len(calls) != 1 {
		t.Fatalf("calls to %s: %+v", p, srv.Calls())
	}
	q, err := url.ParseQuery(calls[0].Query)
	if err != nil {
		t.Fatal(err)
	}
	return q
}

func TestGameSingle(t *testing.T) {
	e := newGameEnv(t, "testfw", "gba")
	e.srv.Content[1] = []byte("ROMDATA")
	var rec recorder
	res, err := Game(context.Background(), e.input(e.info(library.KindSingle, library.GameFile{ID: 10, FileName: "Game.gba", Size: 7})), rec.add)
	if err != nil {
		t.Fatal(err)
	}
	launch := e.romDir + "/Game.gba"
	want := GameResult{GameID: 1, FileID: 10, RomDir: e.romDir, LaunchPath: launch, Placed: []string{launch}}
	if !reflect.DeepEqual(res, want) {
		t.Fatalf("%+v", res)
	}
	if got := read(t, e.root, launch); got != "ROMDATA" {
		t.Fatalf("%q", got)
	}
	q := contentQuery(t, e.srv, romm.RomContentPath(1, "Game.gba"))
	if q.Get("purpose") != "download" || q.Get("file_ids") != "10" {
		t.Fatalf("%v", q)
	}
	dl := rec.stage(library.StageDownload)
	last := dl[len(dl)-1]
	if last != (Progress{Stage: library.StageDownload, Done: 1, Total: 1, BytesDone: 7, BytesTotal: 7}) {
		t.Fatalf("%+v", dl)
	}
	if len(rec.stage(library.StageExtract)) != 0 {
		t.Fatalf("%+v", rec.events)
	}
	noStaging(t, e.root)
}

func TestGamePicksFileID(t *testing.T) {
	e := newGameEnv(t, "testfw", "gba")
	e.srv.Content[1] = []byte("B")
	info := e.info(library.KindNested, library.GameFile{ID: 10, FileName: "A.gba"}, library.GameFile{ID: 11, FileName: "B.gba"})
	in := e.input(info)
	in.FileID = 11
	res, err := Game(context.Background(), in, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.FileID != 11 || res.LaunchPath != e.romDir+"/B.gba" {
		t.Fatalf("%+v", res)
	}
	if exists(e.root, e.romDir+"/A.gba") {
		t.Fatal("A.gba written")
	}
	q := contentQuery(t, e.srv, romm.RomContentPath(1, "B.gba"))
	if q.Get("file_ids") != "11" || q.Get("purpose") != "download" {
		t.Fatalf("%v", q)
	}
}

func TestGameBadFileID(t *testing.T) {
	e := newGameEnv(t, "testfw", "gba")
	e.srv.Content[1] = []byte("x")
	in := e.input(e.info(library.KindSingle, library.GameFile{ID: 10, FileName: "Game.gba"}))
	in.FileID = 99
	if _, err := Game(context.Background(), in, nil); !errors.Is(err, ErrBadFile) {
		t.Fatalf("%v", err)
	}
	if len(e.srv.Calls()) != 0 {
		t.Fatalf("%+v", e.srv.Calls())
	}
}

func TestGameUnsafeFileName(t *testing.T) {
	for _, name := range []string{"../x.gba", "a/b.gba", "..", ".", ""} {
		t.Run(name, func(t *testing.T) {
			e := newGameEnv(t, "testfw", "gba")
			e.srv.Content[1] = []byte("x")
			if _, err := Game(context.Background(), e.input(e.info(library.KindSingle, library.GameFile{ID: 10, FileName: name})), nil); !errors.Is(err, ErrBadFile) {
				t.Fatalf("%v", err)
			}
			info := e.info(library.KindMulti, library.GameFile{ID: 10, FileName: "a.chd"})
			info.FSName = name
			if _, err := Game(context.Background(), e.input(info), nil); !errors.Is(err, ErrBadFile) {
				t.Fatalf("%v", err)
			}
			if len(e.srv.Calls()) != 0 {
				t.Fatalf("%+v", e.srv.Calls())
			}
		})
	}
}

func TestGameNoFiles(t *testing.T) {
	e := newGameEnv(t, "testfw", "gba")
	if _, err := Game(context.Background(), e.input(e.info(library.KindSingle)), nil); !errors.Is(err, ErrNoFiles) {
		t.Fatalf("%v", err)
	}
}

func TestGameNoRomDir(t *testing.T) {
	e := newGameEnv(t, "testfw", "gba")
	info := e.info(library.KindSingle, library.GameFile{ID: 10, FileName: "Game.gba"})
	info.Platform.RomDirs = nil
	if _, err := Game(context.Background(), e.input(info), nil); !errors.Is(err, ErrNoRomDir) {
		t.Fatalf("%v", err)
	}
}

func TestGameMultiWithM3U(t *testing.T) {
	e := newGameEnv(t, "testfw", "psx")
	e.srv.Content[1] = zipBytes(t,
		zipEntry{name: "Game.m3u", body: "Game (Disc 1).cue\nGame (Disc 2).cue\n"},
		zipEntry{name: "Game (Disc 1).cue", body: "c1"},
		zipEntry{name: "Game (Disc 2).cue", body: "c2"},
	)
	var rec recorder
	res, err := Game(context.Background(), e.input(e.info(library.KindMulti, library.GameFile{ID: 10, FileName: "Game (Disc 1).cue"})), rec.add)
	if err != nil {
		t.Fatal(err)
	}
	folder := e.romDir + "/Game"
	want := GameResult{GameID: 1, RomDir: e.romDir, LaunchPath: folder + "/Game.m3u", Placed: []string{
		folder + "/Game (Disc 1).cue", folder + "/Game (Disc 2).cue", folder + "/Game.m3u",
	}}
	if !reflect.DeepEqual(res, want) {
		t.Fatalf("%+v", res)
	}
	if got := read(t, e.root, folder+"/Game.m3u"); got != "Game (Disc 1).cue\nGame (Disc 2).cue\n" {
		t.Fatalf("%q", got)
	}
	q := contentQuery(t, e.srv, romm.RomContentPath(1, "Game"))
	if q.Get("purpose") != "download" || q.Has("file_ids") {
		t.Fatalf("%v", q)
	}
	ex := rec.stage(library.StageExtract)
	wantEx := []Progress{{Stage: library.StageExtract, Done: 0, Total: 1}, {Stage: library.StageExtract, Done: 1, Total: 1}}
	if !reflect.DeepEqual(ex, wantEx) {
		t.Fatalf("%+v", ex)
	}
	noStaging(t, e.root)
	entries, err := os.ReadDir(filepath.Join(e.root, filepath.FromSlash(e.romDir)))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "Game" {
		t.Fatalf("%v", entries)
	}
}

func TestGameMultiM3UCaseInsensitive(t *testing.T) {
	e := newGameEnv(t, "testfw", "psx")
	e.srv.Content[1] = zipBytes(t,
		zipEntry{name: "a.chd", body: "a"},
		zipEntry{name: "GAME.M3U", body: "a.chd\n"},
	)
	res, err := Game(context.Background(), e.input(e.info(library.KindMulti, library.GameFile{ID: 10, FileName: "a.chd"})), nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.LaunchPath != e.romDir+"/Game/GAME.M3U" || len(res.Placed) != 2 {
		t.Fatalf("%+v", res)
	}
}

func TestGameMultiWithoutM3U(t *testing.T) {
	e := newGameEnv(t, "testfw", "psx")
	e.srv.Content[1] = zipBytes(t,
		zipEntry{name: "a.chd", body: "a"},
		zipEntry{name: "b.chd", body: "b"},
	)
	res, err := Game(context.Background(), e.input(e.info(library.KindMulti, library.GameFile{ID: 10, FileName: "a.chd"})), nil)
	if err != nil {
		t.Fatal(err)
	}
	folder := e.romDir + "/Game"
	if res.LaunchPath != folder || !reflect.DeepEqual(res.Placed, []string{folder + "/a.chd", folder + "/b.chd"}) {
		t.Fatalf("%+v", res)
	}
	noStaging(t, e.root)
}

func TestGameMultiMuosLayout(t *testing.T) {
	e := newGameEnv(t, "muos", "psx")
	e.srv.Content[1] = zipBytes(t,
		zipEntry{name: "Game.m3u", body: "a.chd\nb.chd\n"},
		zipEntry{name: "a.chd", body: "a"},
		zipEntry{name: "b.chd", body: "b"},
	)
	res, err := Game(context.Background(), e.input(e.info(library.KindMulti, library.GameFile{ID: 10, FileName: "a.chd"})), nil)
	if err != nil {
		t.Fatal(err)
	}
	folder := e.romDir + "/_Game"
	playlist := e.romDir + "/Game.m3u"
	want := GameResult{GameID: 1, RomDir: e.romDir, LaunchPath: playlist, Placed: []string{
		playlist, folder + "/a.chd", folder + "/b.chd",
	}}
	if !reflect.DeepEqual(res, want) {
		t.Fatalf("%+v", res)
	}
	if got := read(t, e.root, playlist); got != "_Game/a.chd\n_Game/b.chd\n" {
		t.Fatalf("%q", got)
	}
	if exists(e.root, folder+"/Game.m3u") {
		t.Fatal("inner playlist kept")
	}
	noStaging(t, e.root)
}

func TestGameMultiMuosWithoutM3U(t *testing.T) {
	e := newGameEnv(t, "muos", "psx")
	e.srv.Content[1] = zipBytes(t, zipEntry{name: "a.chd", body: "a"})
	res, err := Game(context.Background(), e.input(e.info(library.KindMulti, library.GameFile{ID: 10, FileName: "a.chd"})), nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.LaunchPath != e.romDir+"/_Game" || !reflect.DeepEqual(res.Placed, []string{e.romDir + "/_Game/a.chd"}) {
		t.Fatalf("%+v", res)
	}
	if exists(e.root, e.romDir+"/Game.m3u") {
		t.Fatal("playlist written")
	}
}

func TestGameFileIDForMulti(t *testing.T) {
	e := newGameEnv(t, "testfw", "psx")
	e.srv.Content[1] = zipBytes(t, zipEntry{name: "a.chd", body: "a"})
	in := e.input(e.info(library.KindMulti, library.GameFile{ID: 10, FileName: "a.chd"}))
	in.FileID = 10
	if _, err := Game(context.Background(), in, nil); !errors.Is(err, ErrFileForMulti) {
		t.Fatalf("%v", err)
	}
	if len(e.srv.Calls()) != 0 {
		t.Fatalf("%+v", e.srv.Calls())
	}
}

func TestGameUnzipSingle(t *testing.T) {
	e := newGameEnv(t, "testfw", "gba")
	e.srv.Content[1] = zipBytes(t,
		zipEntry{name: "disc/a.bin", body: "a"},
		zipEntry{name: "x.cue", body: "c"},
		zipEntry{name: "Game.m3u", body: "x.cue\n"},
	)
	var rec recorder
	in := e.input(e.info(library.KindSingle, library.GameFile{ID: 10, FileName: "Game.zip"}))
	in.Unzip = true
	res, err := Game(context.Background(), in, rec.add)
	if err != nil {
		t.Fatal(err)
	}
	want := GameResult{GameID: 1, FileID: 10, RomDir: e.romDir, LaunchPath: e.romDir + "/Game.m3u", Placed: []string{
		e.romDir + "/Game.m3u", e.romDir + "/disc/a.bin", e.romDir + "/x.cue",
	}}
	if !reflect.DeepEqual(res, want) {
		t.Fatalf("%+v", res)
	}
	if exists(e.root, e.romDir+"/Game.zip") {
		t.Fatal("archive kept")
	}
	if len(rec.stage(library.StageExtract)) != 2 {
		t.Fatalf("%+v", rec.events)
	}
	noStaging(t, e.root)
}

func TestGameUnzipSingleFirstFile(t *testing.T) {
	e := newGameEnv(t, "testfw", "gba")
	e.srv.Content[1] = zipBytes(t, zipEntry{name: "Game.gba", body: "g"}, zipEntry{name: "readme.txt", body: "r"})
	in := e.input(e.info(library.KindSingle, library.GameFile{ID: 10, FileName: "Game.zip"}))
	in.Unzip = true
	res, err := Game(context.Background(), in, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.LaunchPath != e.romDir+"/Game.gba" {
		t.Fatalf("%+v", res)
	}
}

func TestGameUnzipSingle7z(t *testing.T) {
	data, err := os.ReadFile("testdata/ok.7z")
	if err != nil {
		t.Fatal(err)
	}
	e := newGameEnv(t, "testfw", "gba")
	e.srv.Content[1] = data
	in := e.input(e.info(library.KindSingle, library.GameFile{ID: 10, FileName: "Game.7z"}))
	in.Unzip = true
	res, err := Game(context.Background(), in, nil)
	if err != nil {
		t.Fatal(err)
	}
	if exists(e.root, e.romDir+"/Game.7z") || len(res.Placed) == 0 {
		t.Fatalf("%+v", res)
	}
	noStaging(t, e.root)
}

func TestGameUnzipNonArchive(t *testing.T) {
	e := newGameEnv(t, "testfw", "gba")
	e.srv.Content[1] = []byte("plain rom")
	in := e.input(e.info(library.KindSingle, library.GameFile{ID: 10, FileName: "Game.gba"}))
	in.Unzip = true
	res, err := Game(context.Background(), in, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.LaunchPath != e.romDir+"/Game.gba" || read(t, e.root, res.LaunchPath) != "plain rom" {
		t.Fatalf("%+v", res)
	}
}

func TestGameUnzipOffKeepsArchive(t *testing.T) {
	e := newGameEnv(t, "testfw", "gba")
	data := zipBytes(t, zipEntry{name: "Game.gba", body: "g"})
	e.srv.Content[1] = data
	res, err := Game(context.Background(), e.input(e.info(library.KindSingle, library.GameFile{ID: 10, FileName: "Game.zip"})), nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.LaunchPath != e.romDir+"/Game.zip" || read(t, e.root, res.LaunchPath) != string(data) {
		t.Fatalf("%+v", res)
	}
	if exists(e.root, e.romDir+"/Game.gba") {
		t.Fatal("extracted")
	}
}

func TestGameRefetchKeepsSave(t *testing.T) {
	e := newGameEnv(t, "testfw", "psx")
	write(t, e.root, e.romDir+"/Game/Game.srm", "save")
	write(t, e.root, e.romDir+"/Game/a.chd", "old")
	e.srv.Content[1] = zipBytes(t, zipEntry{name: "a.chd", body: "new"}, zipEntry{name: "b.chd", body: "b"})
	res, err := Game(context.Background(), e.input(e.info(library.KindMulti, library.GameFile{ID: 10, FileName: "a.chd"})), nil)
	if err != nil {
		t.Fatal(err)
	}
	if read(t, e.root, e.romDir+"/Game/Game.srm") != "save" || read(t, e.root, e.romDir+"/Game/a.chd") != "new" {
		t.Fatal("merge wrong")
	}
	if !reflect.DeepEqual(res.Placed, []string{e.romDir + "/Game/a.chd", e.romDir + "/Game/b.chd"}) {
		t.Fatalf("%+v", res)
	}
}

func TestGameCancelLeavesNothing(t *testing.T) {
	t.Run("before body", func(t *testing.T) {
		e := newGameEnv(t, "testfw", "gba")
		write(t, e.root, e.romDir+"/Game.gba", "old")
		e.srv.Content[1] = []byte("new")
		e.srv.SetContentDelay(10 * time.Second)
		ctx, cancel := context.WithCancel(context.Background())
		time.AfterFunc(50*time.Millisecond, cancel)
		_, err := Game(ctx, e.input(e.info(library.KindSingle, library.GameFile{ID: 10, FileName: "Game.gba"})), nil)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("%v", err)
		}
		noStaging(t, e.root)
		if read(t, e.root, e.romDir+"/Game.gba") != "old" {
			t.Fatal("prior file changed")
		}
	})
	t.Run("mid body", func(t *testing.T) {
		e := newGameEnv(t, "testfw", "gba")
		write(t, e.root, e.romDir+"/Game.gba", "old")
		e.srv.Content[1] = bytes.Repeat([]byte("x"), 4<<20)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		progress := func(p Progress) {
			if p.Stage == library.StageDownload && p.BytesDone > 0 {
				cancel()
			}
		}
		_, err := Game(ctx, e.input(e.info(library.KindSingle, library.GameFile{ID: 10, FileName: "Game.gba"})), progress)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("%v", err)
		}
		noStaging(t, e.root)
		if read(t, e.root, e.romDir+"/Game.gba") != "old" {
			t.Fatal("prior file changed")
		}
	})
	t.Run("multi mid body", func(t *testing.T) {
		e := newGameEnv(t, "testfw", "psx")
		e.srv.Content[1] = zipBytes(t, zipEntry{name: "a.chd", body: strings.Repeat("y", 4<<20)})
		e.srv.Content[1] = append(e.srv.Content[1], bytes.Repeat([]byte{0}, 4<<20)...)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		progress := func(p Progress) {
			if p.Stage == library.StageDownload && p.BytesDone > 0 {
				cancel()
			}
		}
		_, err := Game(ctx, e.input(e.info(library.KindMulti, library.GameFile{ID: 10, FileName: "a.chd"})), progress)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("%v", err)
		}
		noStaging(t, e.root)
		emptyDir(t, e.root, e.romDir)
	})
}

func TestGameUnsafeArchive(t *testing.T) {
	evil := []zipEntry{{name: "ok.txt", body: "ok"}, {name: "../escape", body: "evil"}}
	t.Run("multi", func(t *testing.T) {
		e := newGameEnv(t, "testfw", "psx")
		e.srv.Content[1] = zipBytes(t, evil...)
		_, err := Game(context.Background(), e.input(e.info(library.KindMulti, library.GameFile{ID: 10, FileName: "a.chd"})), nil)
		if !errors.Is(err, ErrUnsafeArchive) {
			t.Fatalf("%v", err)
		}
		onlyUnder(t, e.root, e.romDir)
		emptyDir(t, e.root, e.romDir)
	})
	t.Run("single unzip", func(t *testing.T) {
		e := newGameEnv(t, "testfw", "gba")
		e.srv.Content[1] = zipBytes(t, evil...)
		in := e.input(e.info(library.KindSingle, library.GameFile{ID: 10, FileName: "Game.zip"}))
		in.Unzip = true
		if _, err := Game(context.Background(), in, nil); !errors.Is(err, ErrUnsafeArchive) {
			t.Fatalf("%v", err)
		}
		onlyUnder(t, e.root, e.romDir)
		emptyDir(t, e.root, e.romDir)
	})
}

func TestGameMissingOnServer(t *testing.T) {
	e := newGameEnv(t, "testfw", "gba")
	_, err := Game(context.Background(), e.input(e.info(library.KindSingle, library.GameFile{ID: 10, FileName: "Game.gba"})), nil)
	var se *romm.StatusError
	if !errors.As(err, &se) || !romm.IsNotFound(err) {
		t.Fatalf("%v", err)
	}
	noStaging(t, e.root)
	emptyDir(t, e.root, e.romDir)
}

func TestGameProgressThrottled(t *testing.T) {
	e := newGameEnv(t, "testfw", "gba")
	size := int64(1 << 20)
	e.srv.Content[1] = bytes.Repeat([]byte("z"), int(size))
	fixed := time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC)
	in := e.input(e.info(library.KindSingle, library.GameFile{ID: 10, FileName: "Game.gba", Size: size}))
	in.Clock = func() time.Time { return fixed }
	var rec recorder
	if _, err := Game(context.Background(), in, rec.add); err != nil {
		t.Fatal(err)
	}
	dl := rec.stage(library.StageDownload)
	if len(dl) != 2 {
		t.Fatalf("%d events: %+v", len(dl), dl)
	}
	if dl[0].Done != 0 || dl[0].Total != 1 || dl[0].BytesDone <= 0 || dl[0].BytesTotal != size {
		t.Fatalf("%+v", dl[0])
	}
	if dl[1] != (Progress{Stage: library.StageDownload, Done: 1, Total: 1, BytesDone: size, BytesTotal: size}) {
		t.Fatalf("%+v", dl[1])
	}
}

func TestGameProgressAdvancingClock(t *testing.T) {
	e := newGameEnv(t, "testfw", "gba")
	size := int64(1 << 20)
	e.srv.Content[1] = bytes.Repeat([]byte("z"), int(size))
	now := time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC)
	var mu sync.Mutex
	in := e.input(e.info(library.KindSingle, library.GameFile{ID: 10, FileName: "Game.gba", Size: size}))
	in.Clock = func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		now = now.Add(time.Second)
		return now
	}
	var rec recorder
	if _, err := Game(context.Background(), in, rec.add); err != nil {
		t.Fatal(err)
	}
	if dl := rec.stage(library.StageDownload); len(dl) < 3 {
		t.Fatalf("%d events", len(dl))
	}
}

func TestPickRomDir(t *testing.T) {
	w, root := newHost(t)
	write(t, root, "/r/File", "x")
	mkdest(t, root, "/r/B")
	mkdest(t, root, "/r/C")
	cases := []struct {
		dirs []string
		want string
		ok   bool
	}{
		{[]string{"/r/A", "/r/B", "/r/C"}, "/r/B", true},
		{[]string{"/r/File", "/r/C"}, "/r/C", true},
		{[]string{"/r/A", "/r/Z"}, "/r/A", true},
		{nil, "", false},
	}
	for _, c := range cases {
		got, ok := PickRomDir(w, c.dirs)
		if got != c.want || ok != c.ok {
			t.Fatalf("%v: %q %v", c.dirs, got, ok)
		}
	}
}
