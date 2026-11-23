package library

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io/fs"
	"log/slog"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"testing/fstest"

	"github.com/rommapp/grout/core/config"
	"github.com/rommapp/grout/core/firmware"
	"github.com/rommapp/grout/core/firmware/host"
	"github.com/rommapp/grout/core/romm"
)

const romRoot = "mnt/SDCARD/Roms/"

func scanLibrary(t *testing.T, files fstest.MapFS, roms ...romm.Rom) (*Library, *host.Fake) {
	t.Helper()
	h := &host.Fake{Files: files}
	def, err := firmware.Load("nextui")
	if err != nil {
		t.Fatal(err)
	}
	fw, err := firmware.Resolve(def, h, firmware.Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{DirectoryMappings: mappings("gba")}
	l := newPlatformsLibrary(t, cfg, fw,
		romm.Platform{ID: 1, Slug: "gba", FSSlug: "gba", Name: "GBA", RomCount: 5},
		romm.Platform{ID: 2, Slug: "n64", FSSlug: "n64", Name: "N64", RomCount: 1},
	)
	inTx(t, l.Store, func(tx *sql.Tx) error { return saveGames(tx, roms) })
	return l, h
}

func fileGame(id int, name string, files ...string) romm.Rom {
	r := gameRom(id, 1, name, "", nil, nil)
	for i, f := range files {
		r.Files = append(r.Files, romm.RomFile{ID: id*100 + i, FileName: f})
	}
	return r
}

func mustScan(t *testing.T, l *Library) ScanResult {
	t.Helper()
	res, err := l.Scan(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func mustState(t *testing.T, l *Library, id int) DownloadState {
	t.Helper()
	s, err := l.State(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func gameState(t *testing.T, l *Library, id int) DownloadState {
	t.Helper()
	for _, g := range mustGames(t, l, Query{}) {
		if g.ID == id {
			return g.State
		}
	}
	t.Fatalf("game %d not listed", id)
	return NotDownloaded
}

func TestScanMatchesAlternateFolder(t *testing.T) {
	l, _ := scanLibrary(t, fstest.MapFS{
		romRoot + "Game Boy Advance (MGBA)/Tetris.gba": {Data: []byte("x")},
	}, fileGame(1, "Tetris", "Tetris.gba"))
	res := mustScan(t, l)
	if res.Files != 1 || res.Matched != 1 || res.Platforms != 1 {
		t.Fatalf("res = %+v", res)
	}
	if got := gameState(t, l, 1); got != Downloaded {
		t.Fatalf("state = %v", got)
	}
	if got := mustState(t, l, 1); got != Downloaded {
		t.Fatalf("State = %v", got)
	}
}

func TestScanMultiM3UMatches(t *testing.T) {
	g := fileGame(1, "Big Game", "Disc 1.chd")
	g.HasMultipleFiles = true
	files := fstest.MapFS{romRoot + "Game Boy Advance (GBA)/Other.chd": {Data: []byte("x")}}
	l, h := scanLibrary(t, files, g)
	mustScan(t, l)
	if got := gameState(t, l, 1); got != NotDownloaded {
		t.Fatalf("state = %v", got)
	}
	h.Files[romRoot+"Game Boy Advance (GBA)/Big Game.m3u"] = &fstest.MapFile{Data: []byte("Other.chd")}
	mustScan(t, l)
	if got := gameState(t, l, 1); got != Downloaded {
		t.Fatalf("state = %v", got)
	}
}

func TestScanNestedPartial(t *testing.T) {
	g := fileGame(1, "Nested", "a.gba", "b.gba")
	g.HasNestedSingleFile = true
	l, _ := scanLibrary(t, fstest.MapFS{
		romRoot + "Game Boy Advance (GBA)/Nested/a.gba": {Data: []byte("x")},
	}, g)
	mustScan(t, l)
	if got := gameState(t, l, 1); got != Partial {
		t.Fatalf("state = %v", got)
	}
}

func TestScanCaseInsensitive(t *testing.T) {
	l, _ := scanLibrary(t, fstest.MapFS{
		romRoot + "Game Boy Advance (GBA)/TETRIS.GB": {Data: []byte("x")},
	}, fileGame(1, "Tetris", "Tetris.gb"))
	mustScan(t, l)
	if got := gameState(t, l, 1); got != Downloaded {
		t.Fatalf("state = %v", got)
	}
}

func TestScanIgnoresDotFilesAndDepth3(t *testing.T) {
	l, _ := scanLibrary(t, fstest.MapFS{
		romRoot + "Game Boy Advance (GBA)/.hidden.gb":  {Data: []byte("x")},
		romRoot + "Game Boy Advance (GBA)/a/b/c.gb":    {Data: []byte("x")},
		romRoot + "Game Boy Advance (GBA)/.d/inner.gb": {Data: []byte("x")},
		romRoot + "Game Boy Advance (GBA)/a/ok.gb":     {Data: []byte("x")},
	})
	res := mustScan(t, l)
	if res.Files != 1 {
		t.Fatalf("res = %+v", res)
	}
	var rel string
	if err := l.Store.DB.QueryRow(`SELECT rel FROM local_files`).Scan(&rel); err != nil || rel != "a/ok.gb" {
		t.Fatalf("rel = %q err = %v", rel, err)
	}
}

func TestScanMissingFolderIsFine(t *testing.T) {
	l, _ := scanLibrary(t, fstest.MapFS{romRoot + "other/x.gb": {Data: []byte("x")}})
	res := mustScan(t, l)
	if res.Files != 0 || res.Platforms != 1 {
		t.Fatalf("res = %+v", res)
	}
}

func TestGamesHideDownloadedKeepsPartial(t *testing.T) {
	nested := fileGame(2, "Nested", "a.gba", "b.gba")
	nested.HasNestedSingleFile = true
	l, _ := scanLibrary(t, fstest.MapFS{
		romRoot + "Game Boy Advance (GBA)/Done.gba":     {Data: []byte("x")},
		romRoot + "Game Boy Advance (GBA)/Nested/a.gba": {Data: []byte("x")},
	}, fileGame(1, "Done", "Done.gba"), nested, fileGame(3, "Missing", "Missing.gba"))
	mustScan(t, l)
	got := gameIDs(mustGames(t, l, Query{HideDownloaded: true}))
	if len(got) != 2 || got[0] != 3 || got[1] != 2 {
		t.Fatalf("got %v", got)
	}
}

func TestRescanReplacesPlatformRows(t *testing.T) {
	l, h := scanLibrary(t, fstest.MapFS{
		romRoot + "Game Boy Advance (GBA)/Tetris.gba": {Data: []byte("x")},
	}, fileGame(1, "Tetris", "Tetris.gba"))
	mustScan(t, l)
	delete(h.Files, romRoot+"Game Boy Advance (GBA)/Tetris.gba")
	mustScan(t, l)
	var n int
	if err := l.Store.DB.QueryRow(`SELECT COUNT(*) FROM local_files`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("n = %d err = %v", n, err)
	}
	if got := gameState(t, l, 1); got != NotDownloaded {
		t.Fatalf("state = %v", got)
	}
}

func TestScanSharedBasenameGoesToLowestGameID(t *testing.T) {
	l, _ := scanLibrary(t, fstest.MapFS{
		romRoot + "Game Boy Advance (GBA)/Same.gba": {Data: []byte("x")},
	}, fileGame(5, "B", "Same.gba"), fileGame(3, "A", "Same.gba"))
	mustScan(t, l)
	if gameState(t, l, 3) != Downloaded || gameState(t, l, 5) != NotDownloaded {
		t.Fatal("expected only game 3 downloaded")
	}
}

func TestScanDeletesDisabledPlatformRows(t *testing.T) {
	l, _ := scanLibrary(t, fstest.MapFS{
		romRoot + "Game Boy Advance (GBA)/Tetris.gba": {Data: []byte("x")},
	}, fileGame(1, "Tetris", "Tetris.gba"))
	mustScan(t, l)
	l.Config.DirectoryMappings = mappings()
	mustScan(t, l)
	var n int
	if err := l.Store.DB.QueryRow(`SELECT COUNT(*) FROM local_files`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("n = %d err = %v", n, err)
	}
}

type brokenHost struct {
	*host.Fake
	broken map[string]bool
}

func (b *brokenHost) ReadDir(p string) ([]fs.DirEntry, error) {
	if b.broken[p] {
		return nil, &fs.PathError{Op: "readdir", Path: p, Err: syscall.EIO}
	}
	return b.Fake.ReadDir(p)
}

func TestScanSkipsUnreadableFolder(t *testing.T) {
	badTop := "/" + romRoot + "Game Boy Advance (GBA)"
	badSub := "/" + romRoot + "Game Boy (GB)/Broken"
	h := &brokenHost{
		Fake: &host.Fake{Files: fstest.MapFS{
			romRoot + "Game Boy Advance (GBA)/Old.gba":   {Data: []byte("x")},
			romRoot + "Game Boy Advance (MGBA)/Mgba.gba": {Data: []byte("x")},
			romRoot + "Game Boy (GB)/Tetris.gb":          {Data: []byte("x")},
			romRoot + "Game Boy (GB)/Broken/inner.gb":    {Data: []byte("x")},
		}},
		broken: map[string]bool{},
	}
	def, err := firmware.Load("nextui")
	if err != nil {
		t.Fatal(err)
	}
	fw, err := firmware.Resolve(def, h, firmware.Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{DirectoryMappings: mappings("gba", "gb")}
	l := newPlatformsLibrary(t, cfg, fw,
		romm.Platform{ID: 1, Slug: "gba", FSSlug: "gba", Name: "A GBA", RomCount: 1},
		romm.Platform{ID: 2, Slug: "gb", FSSlug: "gb", Name: "B GB", RomCount: 1},
	)
	if res := mustScan(t, l); res.Files != 4 || len(res.SkippedFolders) != 0 {
		t.Fatalf("first res = %+v", res)
	}
	h.broken[badTop] = true
	h.broken[badSub] = true

	res := mustScan(t, l)

	if res.Files != 2 || res.Platforms != 2 {
		t.Fatalf("res = %+v", res)
	}
	if !reflect.DeepEqual(res.SkippedFolders, []string{badTop, badSub}) {
		t.Fatalf("skipped = %v", res.SkippedFolders)
	}
	rows, err := l.Store.DB.Query(`SELECT rel FROM local_files ORDER BY rel`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var rels []string
	for rows.Next() {
		var rel string
		if err := rows.Scan(&rel); err != nil {
			t.Fatal(err)
		}
		rels = append(rels, rel)
	}
	if !reflect.DeepEqual(rels, []string{"Mgba.gba", "Tetris.gb"}) {
		t.Fatalf("rels = %v", rels)
	}
}

func TestScanResultSkippedFoldersEncodesEmpty(t *testing.T) {
	l, _ := scanLibrary(t, fstest.MapFS{})
	b, err := json.Marshal(mustScan(t, l))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"skipped_folders":[]`) {
		t.Fatalf("json = %s", b)
	}
}

func TestScanLogsUnreadableFolderToLibraryLogger(t *testing.T) {
	bad := "/" + romRoot + "Game Boy (GB)"
	h := &brokenHost{
		Fake:   &host.Fake{Files: fstest.MapFS{romRoot + "Game Boy (GB)/Tetris.gb": {Data: []byte("x")}}},
		broken: map[string]bool{bad: true},
	}
	def, err := firmware.Load("nextui")
	if err != nil {
		t.Fatal(err)
	}
	fw, err := firmware.Resolve(def, h, firmware.Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{DirectoryMappings: mappings("gb")}
	l := newPlatformsLibrary(t, cfg, fw, romm.Platform{ID: 1, Slug: "gb", FSSlug: "gb", Name: "GB", RomCount: 1})
	var buf bytes.Buffer
	l.Log = slog.New(slog.NewTextHandler(&buf, nil))

	mustScan(t, l)

	if !strings.Contains(buf.String(), "scan: skipping unreadable folder") {
		t.Fatalf("log = %q", buf.String())
	}
}
