package library

import (
	"context"
	"errors"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/rommapp/grout/core/config"
	"github.com/rommapp/grout/core/firmware"
	"github.com/rommapp/grout/core/firmware/host"
	"github.com/rommapp/grout/core/ident"
)

type fakeExtractor struct {
	version string
	results map[string]ident.Result
	calls   []string
	onCall  func()
}

func (f *fakeExtractor) Extract(p, platform string) ident.Result {
	f.calls = append(f.calls, filepath.Base(p))
	if f.onCall != nil {
		f.onCall()
	}
	if r, ok := f.results[filepath.Base(p)]; ok {
		return r
	}
	return ident.Result{Status: ident.StatusOK, TitleID: "T-" + filepath.Base(p), Platform: platform}
}

func (f *fakeExtractor) Version() string { return f.version }

type identFixture struct {
	l    *Library
	root string
	fake *fakeExtractor
}

func newIdentFixture(t *testing.T) *identFixture {
	t.Helper()
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
	l := New(openTestStore(t), nil, &config.Config{}, fw)
	fake := &fakeExtractor{version: "1.0.0", results: map[string]ident.Result{}}
	l.Ident = fake
	for _, q := range []string{
		`INSERT INTO platforms (id, slug, fs_slug, name) VALUES (1, 'psp', 'psp', 'PSP')`,
		`INSERT INTO platforms (id, slug, fs_slug, name) VALUES (2, 'psx', 'psx', 'PSX')`,
		`INSERT INTO platforms (id, slug, fs_slug, name) VALUES (3, 'gba', 'gba', 'GBA')`,
	} {
		if _, err := l.Store.DB.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	return &identFixture{l: l, root: root, fake: fake}
}

func (f *identFixture) game(t *testing.T, id, platformID int, files ...string) []string {
	t.Helper()
	if _, err := f.l.Store.DB.Exec(`INSERT INTO games (id, platform_id, name, kind) VALUES (?, ?, 'G', 'single')`, id, platformID); err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, name := range files {
		p := path.Join("/mnt/SDCARD/Roms/X", name)
		real := f.real(p)
		if err := os.MkdirAll(filepath.Dir(real), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(real, []byte("data"), 0o644); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(real)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.l.Store.DB.Exec(`INSERT INTO local_files (path, platform_id, rel, basename, size_bytes, mod_time, game_id) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			p, platformID, name, name, info.Size(), info.ModTime().Unix(), id); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	return paths
}

func (f *identFixture) real(p string) string {
	return filepath.Join(f.root, filepath.FromSlash(p))
}

func mustIdentities(t *testing.T, l *Library, id int) []Identity {
	t.Helper()
	got, err := l.Identities(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestIdentitiesExtractsOnceAndCaches(t *testing.T) {
	f := newIdentFixture(t)
	f.game(t, 1, 1, "Game.iso")
	for i := 0; i < 2; i++ {
		got := mustIdentities(t, f.l, 1)
		if len(got) != 1 || got[0].TitleID != "T-Game.iso" || got[0].Status != ident.StatusOK {
			t.Fatalf("call %d: %+v", i, got)
		}
	}
	if len(f.fake.calls) != 1 {
		t.Fatalf("calls = %v", f.fake.calls)
	}
	var status string
	if err := f.l.Store.DB.QueryRow(`SELECT status FROM identities`).Scan(&status); err != nil || status != "ok" {
		t.Fatalf("status = %q err = %v", status, err)
	}
}

func TestIdentitiesReidentifiesAChangedFile(t *testing.T) {
	f := newIdentFixture(t)
	paths := f.game(t, 1, 1, "Game.iso")
	mustIdentities(t, f.l, 1)
	if err := os.WriteFile(f.real(paths[0]), []byte("DATA"), 0o644); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(f.real(paths[0]), later, later); err != nil {
		t.Fatal(err)
	}
	mustIdentities(t, f.l, 1)
	if len(f.fake.calls) != 2 {
		t.Fatalf("calls = %v", f.fake.calls)
	}
}

func TestIdentitiesSkipsPlaylistsAndUnmappedPlatforms(t *testing.T) {
	f := newIdentFixture(t)
	f.game(t, 1, 2, "Game.m3u", "Game (Disc 1).chd", "Game (Disc 2).chd", "Other.M3U")
	got := mustIdentities(t, f.l, 1)
	if len(got) != 2 || len(f.fake.calls) != 2 {
		t.Fatalf("got = %+v calls = %v", got, f.fake.calls)
	}
	for _, c := range f.fake.calls {
		if c == "Game.m3u" || c == "Other.M3U" {
			t.Fatalf("playlist extracted: %s", c)
		}
	}
	f.fake.calls = nil
	f.game(t, 2, 3, "Tetris.gba")
	if got := mustIdentities(t, f.l, 2); len(got) != 0 || len(f.fake.calls) != 0 {
		t.Fatalf("got = %+v calls = %v", got, f.fake.calls)
	}
}

func TestIdentitiesStoresFailures(t *testing.T) {
	f := newIdentFixture(t)
	f.game(t, 1, 1, "Game.iso")
	f.fake.results["Game.iso"] = ident.Result{Status: ident.StatusNotFound}
	mustIdentities(t, f.l, 1)
	got := mustIdentities(t, f.l, 1)
	if len(f.fake.calls) != 1 || len(got) != 1 || got[0].Status != ident.StatusNotFound {
		t.Fatalf("got = %+v calls = %v", got, f.fake.calls)
	}
}

func TestIdentitiesAfterSigilUpgrade(t *testing.T) {
	f := newIdentFixture(t)
	f.game(t, 1, 1, "a.iso", "b.iso", "c.iso")
	f.fake.results["b.iso"] = ident.Result{Status: ident.StatusOK, TitleID: "B", Experimental: true}
	f.fake.results["c.iso"] = ident.Result{Status: ident.StatusNotFound}
	mustIdentities(t, f.l, 1)
	f.fake.calls = nil
	f.fake.version = "1.1.0"
	mustIdentities(t, f.l, 1)
	if len(f.fake.calls) != 2 || f.fake.calls[0] != "b.iso" || f.fake.calls[1] != "c.iso" {
		t.Fatalf("calls = %v", f.fake.calls)
	}
}

func TestIdentitiesCancelledKeepsWhatFinished(t *testing.T) {
	f := newIdentFixture(t)
	f.game(t, 1, 1, "a.iso", "b.iso")
	ctx, cancel := context.WithCancel(context.Background())
	f.fake.onCall = cancel
	got, err := f.l.Identities(ctx, 1)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got = %+v", got)
	}
	var n int
	if err := f.l.Store.DB.QueryRow(`SELECT COUNT(*) FROM identities`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("rows = %d err = %v", n, err)
	}
}

func TestIdentitiesSkipsAFileGoneFromDisk(t *testing.T) {
	f := newIdentFixture(t)
	paths := f.game(t, 1, 1, "Game.iso")
	if err := os.Remove(f.real(paths[0])); err != nil {
		t.Fatal(err)
	}
	got := mustIdentities(t, f.l, 1)
	if len(got) != 0 || len(f.fake.calls) != 0 {
		t.Fatalf("got = %+v calls = %v", got, f.fake.calls)
	}
}

func TestIdentitiesNilExtractor(t *testing.T) {
	f := newIdentFixture(t)
	f.game(t, 1, 1, "Game.iso")
	f.l.Ident = nil
	got, err := f.l.Identities(context.Background(), 1)
	if got != nil || err != nil {
		t.Fatalf("got = %+v err = %v", got, err)
	}
}

func TestIdentitiesUnknownGame(t *testing.T) {
	f := newIdentFixture(t)
	if _, err := f.l.Identities(context.Background(), 99); !errors.Is(err, ErrUnknownGame) {
		t.Fatalf("err = %v", err)
	}
}
