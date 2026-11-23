package library

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/rommapp/grout/core/config"
	"github.com/rommapp/grout/core/firmware"
	"github.com/rommapp/grout/core/firmware/host"
	"github.com/rommapp/grout/core/romm"
)

const gbaDir = "/" + romRoot + "Game Boy Advance (GBA)"

func TestFetchInfoUnknownGame(t *testing.T) {
	l, _ := scanLibrary(t, fstest.MapFS{}, fileGame(1, "Tetris", "Tetris.gba"))
	ctx := context.Background()
	if _, err := l.FetchInfo(ctx, 99); !errors.Is(err, ErrUnknownGame) {
		t.Fatalf("FetchInfo err = %v", err)
	}
	if _, err := l.GameFiles(ctx, 99); !errors.Is(err, ErrUnknownGame) {
		t.Fatalf("GameFiles err = %v", err)
	}
	if _, _, err := l.LaunchFile(ctx, 99); !errors.Is(err, ErrUnknownGame) {
		t.Fatalf("LaunchFile err = %v", err)
	}
	info, err := l.FetchInfo(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if info.GameID != 1 || info.Name != "Tetris" || info.Kind != KindSingle || info.FSNameNoExt != "Tetris" {
		t.Fatalf("info = %+v", info)
	}
	if info.Platform.ID != 1 || info.Platform.FSSlug != "gba" || len(info.Platform.RomDirs) == 0 || info.Platform.RomDirs[0] != gbaDir {
		t.Fatalf("platform = %+v", info.Platform)
	}
	if len(info.Files) != 1 || info.Files[0].FileName != "Tetris.gba" {
		t.Fatalf("files = %+v", info.Files)
	}
}

func TestGameKindsList(t *testing.T) {
	if got := GameKinds(); !reflect.DeepEqual(got, []GameKind{KindSingle, KindNested, KindMulti}) {
		t.Fatalf("kinds = %v", got)
	}
}

func TestGameFilesPlayableOnly(t *testing.T) {
	g := gameRom(1, 1, "Big", "", nil, nil)
	g.HasMultipleFiles = true
	g.Files = []romm.RomFile{
		{ID: 12, FileName: "b.chd", FileSizeBytes: 20},
		{ID: 11, FileName: "manual.pdf", Category: "manual", FileSizeBytes: 5},
		{ID: 10, FileName: "a.chd", FileSizeBytes: 10},
	}
	l, _ := scanLibrary(t, fstest.MapFS{}, g)
	got, err := l.GameFiles(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	want := []GameFile{{ID: 10, FileName: "a.chd", Size: 10}, {ID: 12, FileName: "b.chd", Size: 20}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("files = %+v", got)
	}
	info, err := l.FetchInfo(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if info.Kind != KindMulti || !reflect.DeepEqual(info.Files, want) {
		t.Fatalf("info = %+v", info)
	}
}

func TestRecordFetchMarksDownloaded(t *testing.T) {
	launch := gbaDir + "/Tetris.gba"
	l, _ := scanLibrary(t, fstest.MapFS{
		romRoot + "Game Boy Advance (GBA)/Tetris.gba":       {Data: []byte("abc")},
		romRoot + "Game Boy Advance (GBA)/Sub/extra.sav":    {Data: []byte("x")},
		romRoot + "Game Boy Advance (GBA)/Sub/Deep/x.bin":   {Data: []byte("x")},
		romRoot + "Game Boy Advance (GBA)/Other/Tetris.txt": {Data: []byte("x")},
	}, fileGame(1, "Tetris", "Tetris.zip"))
	ctx := context.Background()
	if got := mustState(t, l, 1); got != NotDownloaded {
		t.Fatalf("state before = %v", got)
	}
	d := Download{GameID: 1, PlatformID: 1, FileID: 100, LaunchPath: launch,
		Placed: []string{launch, gbaDir + "/Sub/extra.sav", gbaDir + "/Sub/Deep/x.bin"}}
	if err := l.RecordFetch(ctx, d); err != nil {
		t.Fatal(err)
	}
	if got := mustState(t, l, 1); got != Downloaded {
		t.Fatalf("state = %v", got)
	}
	if got := gameState(t, l, 1); got != Downloaded {
		t.Fatalf("listed state = %v", got)
	}
	got, ok, err := l.Downloaded(ctx, 1)
	if err != nil || !ok {
		t.Fatalf("ok = %v err = %v", ok, err)
	}
	if got.GameID != 1 || got.PlatformID != 1 || got.FileID != 100 || got.LaunchPath != launch {
		t.Fatalf("download = %+v", got)
	}
	if !reflect.DeepEqual(got.Placed, []string{gbaDir + "/Sub/extra.sav", launch}) {
		t.Fatalf("placed = %v", got.Placed)
	}
	var rel string
	var size int64
	var gameID int
	if err := l.Store.DB.QueryRow(`SELECT rel, size_bytes, game_id FROM local_files WHERE path = ?`, launch).Scan(&rel, &size, &gameID); err != nil {
		t.Fatal(err)
	}
	if rel != "Tetris.gba" || size != 3 || gameID != 1 {
		t.Fatalf("rel = %q size = %d game = %d", rel, size, gameID)
	}
	if n := count(t, l.Store, `SELECT COUNT(*) FROM local_files WHERE path LIKE '%x.bin'`); n != 0 {
		t.Fatalf("deep rows = %d", n)
	}
	var fetchedAt string
	if err := l.Store.DB.QueryRow(`SELECT fetched_at FROM downloads WHERE game_id = 1`).Scan(&fetchedAt); err != nil || !strings.HasSuffix(fetchedAt, "Z") {
		t.Fatalf("fetched_at = %q err = %v", fetchedAt, err)
	}
	path, ok, err := l.LaunchFile(ctx, 1)
	if err != nil || !ok || path != launch {
		t.Fatalf("launch = %q ok = %v err = %v", path, ok, err)
	}
	if _, ok, err := l.Downloaded(ctx, 2); ok || err != nil {
		t.Fatalf("game 2 ok = %v err = %v", ok, err)
	}
}

func TestLaunchFileFromScan(t *testing.T) {
	multi := fileGame(1, "Big", "a.chd")
	multi.HasMultipleFiles = true
	m3u := fileGame(2, "Disc", "d1.chd")
	m3u.HasMultipleFiles = true
	l, _ := scanLibrary(t, fstest.MapFS{
		romRoot + "Game Boy Advance (GBA)/Big/a.chd":   {Data: []byte("x")},
		romRoot + "Game Boy Advance (GBA)/Disc.m3u":    {Data: []byte("x")},
		romRoot + "Game Boy Advance (GBA)/Disc/d1.chd": {Data: []byte("x")},
		romRoot + "Game Boy Advance (GBA)/Zed.gba":     {Data: []byte("x")},
	}, multi, m3u, fileGame(3, "Zed", "Zed.gba"), fileGame(4, "None", "None.gba"))
	mustScan(t, l)
	ctx := context.Background()
	for id, want := range map[int]string{1: gbaDir + "/Big", 2: gbaDir + "/Disc.m3u", 3: gbaDir + "/Zed.gba"} {
		got, ok, err := l.LaunchFile(ctx, id)
		if err != nil || !ok || got != want {
			t.Fatalf("game %d launch = %q ok = %v err = %v", id, got, ok, err)
		}
	}
	if got, ok, err := l.LaunchFile(ctx, 4); err != nil || ok || got != "" {
		t.Fatalf("game 4 launch = %q ok = %v err = %v", got, ok, err)
	}
}

func TestMultiWithoutM3UFolderIsDownloaded(t *testing.T) {
	g := fileGame(1, "Game", "x.chd")
	g.HasMultipleFiles = true
	l, _ := scanLibrary(t, fstest.MapFS{
		romRoot + "Game Boy Advance (GBA)/Game/a.chd": {Data: []byte("x")},
	}, g)
	res := mustScan(t, l)
	if res.Matched != 1 {
		t.Fatalf("res = %+v", res)
	}
	if got := gameState(t, l, 1); got != Downloaded {
		t.Fatalf("state = %v", got)
	}
}

func TestMuosHiddenFolderMatches(t *testing.T) {
	def, err := firmware.Load("muos")
	if err != nil {
		t.Fatal(err)
	}
	probe, err := firmware.Resolve(def, &host.Fake{Files: fstest.MapFS{}}, firmware.Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	dirs := probe.RomDirs("gba")
	if len(dirs) == 0 {
		t.Fatal("no muOS gba rom dir")
	}
	dir := strings.TrimPrefix(dirs[0], "/")
	h := &host.Fake{Files: fstest.MapFS{
		dir + "/_Game/a.chd":  {Data: []byte("x")},
		dir + "/Other.m3u":    {Data: []byte("x")},
		dir + "/_Other/b.chd": {Data: []byte("x")},
	}}
	fw, err := firmware.Resolve(def, h, firmware.Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{DirectoryMappings: mappings("gba")}
	l := newPlatformsLibrary(t, cfg, fw, romm.Platform{ID: 1, Slug: "gba", FSSlug: "gba", Name: "GBA", RomCount: 2})
	game := fileGame(1, "Game", "zz.chd")
	game.HasMultipleFiles = true
	game.FSName = "Game"
	other := fileGame(2, "Other", "yy.chd")
	other.HasMultipleFiles = true
	other.FSName = "Other"
	inTx(t, l.Store, func(tx *sql.Tx) error { return saveGames(tx, []romm.Rom{game, other}) })
	mustScan(t, l)
	if got := gameState(t, l, 1); got != Downloaded {
		t.Fatalf("game state = %v", got)
	}
	if got := gameState(t, l, 2); got != Downloaded {
		t.Fatalf("other state = %v", got)
	}
	if n := count(t, l.Store, `SELECT COUNT(*) FROM local_files WHERE game_id = 2`); n != 2 {
		t.Fatalf("other rows = %d", n)
	}
	got, ok, err := l.LaunchFile(context.Background(), 2)
	if err != nil || !ok || got != dirs[0]+"/Other.m3u" {
		t.Fatalf("launch = %q ok = %v err = %v", got, ok, err)
	}
}

func TestUnzippedArchiveSurvivesRescan(t *testing.T) {
	launch := gbaDir + "/Game.gba"
	l, h := scanLibrary(t, fstest.MapFS{
		romRoot + "Game Boy Advance (GBA)/Game.gba": {Data: []byte("x")},
	}, fileGame(1, "Game", "Game.zip"))
	ctx := context.Background()
	if err := l.RecordFetch(ctx, Download{GameID: 1, PlatformID: 1, FileID: 100, LaunchPath: launch, Placed: []string{launch}}); err != nil {
		t.Fatal(err)
	}
	mustScan(t, l)
	if got := gameState(t, l, 1); got != Downloaded {
		t.Fatalf("state after rescan = %v", got)
	}
	delete(h.Files, romRoot+"Game Boy Advance (GBA)/Game.gba")
	mustScan(t, l)
	if got := gameState(t, l, 1); got != NotDownloaded {
		t.Fatalf("state after delete = %v", got)
	}
	if n := count(t, l.Store, `SELECT COUNT(*) FROM downloads`); n != 0 {
		t.Fatalf("downloads = %d", n)
	}
}

func TestScanKeepsDisabledPlatformDownloads(t *testing.T) {
	launch := gbaDir + "/Game.gba"
	l, _ := scanLibrary(t, fstest.MapFS{}, fileGame(1, "Game", "Game.zip"))
	if err := l.RecordFetch(context.Background(), Download{GameID: 1, PlatformID: 1, LaunchPath: launch}); err != nil {
		t.Fatal(err)
	}
	l.Config.DirectoryMappings = mappings()
	mustScan(t, l)
	if n := count(t, l.Store, `SELECT COUNT(*) FROM downloads`); n != 1 {
		t.Fatalf("downloads = %d", n)
	}
}

func TestScanSweepsStaging(t *testing.T) {
	root := t.TempDir()
	h := host.OS(func(k string) string {
		if k == "GROUT_ROOT" {
			return root
		}
		return ""
	})
	def, err := firmware.Load("nextui")
	if err != nil {
		t.Fatal(err)
	}
	fw, err := firmware.Resolve(def, h, firmware.Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, filepath.FromSlash(romRoot), "Game Boy Advance (GBA)")
	for _, p := range []string{".grout-x/inner.gba", "Sub/.grout-y", "Keep.gba", "Sub/keep.gba", "Sub/Deep/.grout-z"} {
		full := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := &config.Config{DirectoryMappings: mappings("gba")}
	l := newPlatformsLibrary(t, cfg, fw, romm.Platform{ID: 1, Slug: "gba", FSSlug: "gba", Name: "GBA", RomCount: 1})
	res := mustScan(t, l)
	if res.Files != 2 {
		t.Fatalf("res = %+v", res)
	}
	for p, want := range map[string]bool{".grout-x": false, "Sub/.grout-y": false, "Keep.gba": true, "Sub/keep.gba": true, "Sub/Deep/.grout-z": true} {
		_, err := os.Stat(filepath.Join(dir, filepath.FromSlash(p)))
		if exists := err == nil; exists != want {
			t.Fatalf("%s exists = %v err = %v", p, exists, err)
		}
	}
}

func TestStagesList(t *testing.T) {
	want := []Stage{StagePlatforms, StageGames, StageCollections, StagePurge, StageScan, StageDownload, StageExtract, StageArt, StageBios}
	if got := Stages(); !reflect.DeepEqual(got, want) {
		t.Fatalf("stages = %v", got)
	}
	p := Progress{Stage: StageDownload, BytesDone: 1, BytesTotal: 2}
	if p.BytesDone != 1 || p.BytesTotal != 2 {
		t.Fatalf("progress = %+v", p)
	}
}

func TestStrayDiscDoesNotMatchMultiGame(t *testing.T) {
	a := fileGame(1, "Alpha", "Disc 1.chd")
	a.HasMultipleFiles = true
	b := fileGame(2, "Beta", "Disc 1.chd")
	b.HasMultipleFiles = true
	l, _ := scanLibrary(t, fstest.MapFS{
		romRoot + "Game Boy Advance (GBA)/Disc 1.chd":       {Data: []byte("x")},
		romRoot + "Game Boy Advance (GBA)/Other/Disc 1.chd": {Data: []byte("x")},
		romRoot + "Game Boy Advance (GBA)/Solo.chd":         {Data: []byte("x")},
	}, a, b, fileGame(3, "Solo", "Solo.chd"), fileGame(4, "SoloDisc", "Disc 1.chd"))
	mustScan(t, l)
	for _, id := range []int{1, 2} {
		if got := gameState(t, l, id); got != NotDownloaded {
			t.Fatalf("multi game %d state = %v", id, got)
		}
	}
	if got := gameState(t, l, 4); got != Downloaded {
		t.Fatalf("single game sharing disc name state = %v", got)
	}
	if n := count(t, l.Store, `SELECT COUNT(*) FROM local_files WHERE game_id IN (1, 2)`); n != 0 {
		t.Fatalf("multi rows = %d", n)
	}
}

func TestLaunchFileMultiPrefersM3UThenFolder(t *testing.T) {
	g := fileGame(1, "Big", "a.chd")
	g.HasMultipleFiles = true
	l, h := scanLibrary(t, fstest.MapFS{
		romRoot + "Game Boy Advance (GBA)/Big/a.chd": {Data: []byte("x")},
		romRoot + "Game Boy Advance (GBA)/Big.m3u":   {Data: []byte("x")},
	}, g)
	mustScan(t, l)
	ctx := context.Background()
	if got, ok, err := l.LaunchFile(ctx, 1); err != nil || !ok || got != gbaDir+"/Big.m3u" {
		t.Fatalf("with m3u launch = %q ok = %v err = %v", got, ok, err)
	}
	delete(h.Files, romRoot+"Game Boy Advance (GBA)/Big.m3u")
	mustScan(t, l)
	if _, err := l.Store.DB.Exec(`INSERT INTO local_files (path, platform_id, rel, basename, game_id) VALUES (?, 1, 'A.bin', 'A.bin', 1)`, gbaDir+"/A.bin"); err != nil {
		t.Fatal(err)
	}
	if got, ok, err := l.LaunchFile(ctx, 1); err != nil || !ok || got != gbaDir+"/Big" {
		t.Fatalf("folder launch = %q ok = %v err = %v", got, ok, err)
	}
}
