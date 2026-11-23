package fetch

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type zipEntry struct {
	name string
	body string
	mode fs.FileMode
	set  bool
}

func makeZip(t *testing.T, root, p string, entries []zipEntry) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		if e.set {
			h.SetMode(e.mode)
		}
		fw, err := zw.CreateHeader(h)
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
	write(t, root, p, buf.String())
}

func mkdest(t *testing.T, root, p string) {
	if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(p)), 0755); err != nil {
		t.Fatal(err)
	}
}

func onlyUnder(t *testing.T, root string, allowed ...string) {
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || p == root {
			return err
		}
		rel := "/" + filepath.ToSlash(strings.TrimPrefix(p, root+string(filepath.Separator)))
		for _, a := range allowed {
			if rel == a || strings.HasPrefix(rel, a+"/") || strings.HasPrefix(a, rel+"/") {
				return nil
			}
		}
		t.Errorf("unexpected path %s", rel)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestExtractZipNested(t *testing.T) {
	w, root := newHost(t)
	makeZip(t, root, "/dl/game.zip", []zipEntry{
		{name: "top.cue", body: "cue"},
		{name: "disc/", body: ""},
		{name: "disc/track 1.bin", body: "one"},
		{name: "disc\\sub\\track2.bin", body: "two"},
	})
	mkdest(t, root, "/roms/psx/.grout-x")
	got, err := Extract(context.Background(), w, "/dl/game.zip", "/roms/psx/.grout-x", DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"top.cue", "disc/track 1.bin", "disc/sub/track2.bin"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%q", got)
	}
	if read(t, root, "/roms/psx/.grout-x/disc/sub/track2.bin") != "two" || read(t, root, "/roms/psx/.grout-x/top.cue") != "cue" {
		t.Fatal("content")
	}
}

func TestExtract7z(t *testing.T) {
	w, root := newHost(t)
	b, err := os.ReadFile("testdata/ok.7z")
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, "/dl/ok.7z", string(b))
	mkdest(t, root, "/s")
	got, err := Extract(context.Background(), w, "/dl/ok.7z", "/s", DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"a.txt", "d/b.txt"}) {
		t.Fatalf("%q", got)
	}
	if read(t, root, "/s/a.txt") != "alpha" || read(t, root, "/s/d/b.txt") != "bravo" {
		t.Fatal("content")
	}
}

func TestExtractRejectsTraversal(t *testing.T) {
	for _, name := range []string{"../escape", "a/../../escape", "../../../escape", "/etc/x", "C:\\x", "..\\escape"} {
		t.Run(name, func(t *testing.T) {
			w, root := newHost(t)
			makeZip(t, root, "/dl/a.zip", []zipEntry{{name: "ok.txt", body: "ok"}, {name: name, body: "evil"}})
			mkdest(t, root, "/stage/x")
			_, err := Extract(context.Background(), w, "/dl/a.zip", "/stage/x", DefaultLimits())
			if !errors.Is(err, ErrUnsafeArchive) {
				t.Fatalf("%v", err)
			}
			onlyUnder(t, root, "/dl/a.zip", "/stage/x")
		})
	}
}

func TestExtractSkipsSymlink(t *testing.T) {
	w, root := newHost(t)
	makeZip(t, root, "/dl/a.zip", []zipEntry{
		{name: "link", body: "../../outside", mode: fs.ModeSymlink | 0777, set: true},
		{name: "real.txt", body: "real"},
	})
	mkdest(t, root, "/s")
	got, err := Extract(context.Background(), w, "/dl/a.zip", "/s", DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"real.txt"}) {
		t.Fatalf("%q", got)
	}
	if _, err := os.Lstat(filepath.Join(root, "s", "link")); !os.IsNotExist(err) {
		t.Fatal("symlink written")
	}
	onlyUnder(t, root, "/dl/a.zip", "/s")
}

func TestExtractCapsBytes(t *testing.T) {
	w, root := newHost(t)
	makeZip(t, root, "/dl/a.zip", []zipEntry{{name: "big", body: "01234567890"}})
	mkdest(t, root, "/s")
	_, err := Extract(context.Background(), w, "/dl/a.zip", "/s", Limits{MaxBytes: 10, MaxEntries: 10})
	if !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("%v", err)
	}
}

func TestExtract7zSkippedEntryCountsBytes(t *testing.T) {
	w, root := newHost(t)
	b, err := os.ReadFile("testdata/link.7z")
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, "/dl/link.7z", string(b))
	mkdest(t, root, "/s")
	got, err := Extract(context.Background(), w, "/dl/link.7z", "/s", DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"b.txt"}) {
		t.Fatalf("%q", got)
	}
	if _, err := os.Lstat(filepath.Join(root, "s", "a-link")); !os.IsNotExist(err) {
		t.Fatal("symlink written")
	}
	mkdest(t, root, "/s2")
	got, err = Extract(context.Background(), w, "/dl/link.7z", "/s2", Limits{MaxBytes: 20, MaxEntries: 10})
	if !errors.Is(err, ErrUnsafeArchive) || got != nil {
		t.Fatalf("%q %v", got, err)
	}
}

func TestExtractRejectsMalformed(t *testing.T) {
	cases := map[string][]zipEntry{
		"file then child":   {{name: "a", body: "1"}, {name: "a/b", body: "2"}},
		"dir then file":     {{name: "a/", body: ""}, {name: "a", body: "2"}},
		"implied dir":       {{name: "a/b", body: "1"}, {name: "a", body: "2"}},
		"duplicate":         {{name: "a.bin", body: "1"}, {name: "a.bin", body: "2"}},
		"case duplicate":    {{name: "Game.bin", body: "1"}, {name: "game.BIN", body: "2"}},
		"case parent clash": {{name: "Disc", body: "1"}, {name: "disc/t.bin", body: "2"}},
		"nul":               {{name: "a\x00b", body: "1"}},
	}
	for name, entries := range cases {
		t.Run(name, func(t *testing.T) {
			w, root := newHost(t)
			makeZip(t, root, "/dl/a.zip", entries)
			mkdest(t, root, "/s")
			got, err := Extract(context.Background(), w, "/dl/a.zip", "/s", DefaultLimits())
			if !errors.Is(err, ErrUnsafeArchive) || got != nil {
				t.Fatalf("%q %v", got, err)
			}
		})
	}
}

func TestExtractAllowsRepeatedDir(t *testing.T) {
	w, root := newHost(t)
	makeZip(t, root, "/dl/a.zip", []zipEntry{{name: "d/", body: ""}, {name: "D/x", body: "x"}, {name: "d/", body: ""}, {name: "d/y", body: "y"}})
	mkdest(t, root, "/s")
	got, err := Extract(context.Background(), w, "/dl/a.zip", "/s", DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"D/x", "d/y"}) {
		t.Fatalf("%q", got)
	}
}

func TestExtractCapsEntries(t *testing.T) {
	w, root := newHost(t)
	makeZip(t, root, "/dl/a.zip", []zipEntry{{name: "a", body: "a"}, {name: "b", body: "b"}, {name: "c", body: "c"}})
	mkdest(t, root, "/s")
	got, err := Extract(context.Background(), w, "/dl/a.zip", "/s", Limits{MaxBytes: 1 << 20, MaxEntries: 2})
	if !errors.Is(err, ErrUnsafeArchive) || got != nil {
		t.Fatalf("%q %v", got, err)
	}
	if exists(root, "/s/a") {
		t.Fatal("wrote before entry count check")
	}
}

func TestExtractNotAnArchive(t *testing.T) {
	w, root := newHost(t)
	write(t, root, "/dl/a.zip", "this is not a zip file at all")
	mkdest(t, root, "/s")
	_, err := Extract(context.Background(), w, "/dl/a.zip", "/s", DefaultLimits())
	if !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("%v", err)
	}
}

func TestExtractNormalizesMode(t *testing.T) {
	w, root := newHost(t)
	makeZip(t, root, "/dl/a.zip", []zipEntry{{name: "locked.bin", body: "data", mode: 0, set: true}})
	mkdest(t, root, "/s")
	if _, err := Extract(context.Background(), w, "/dl/a.zip", "/s", DefaultLimits()); err != nil {
		t.Fatal(err)
	}
	if read(t, root, "/s/locked.bin") != "data" {
		t.Fatal("content")
	}
	st, err := os.Stat(filepath.Join(root, "s", "locked.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm()&0400 == 0 || st.Mode().Perm()&0111 != 0 {
		t.Fatalf("%v", st.Mode())
	}
}

func TestExtractCancelled(t *testing.T) {
	w, root := newHost(t)
	makeZip(t, root, "/dl/a.zip", []zipEntry{{name: "a", body: "a"}})
	mkdest(t, root, "/s")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Extract(ctx, w, "/dl/a.zip", "/s", DefaultLimits())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("%v", err)
	}
	if exists(root, "/s/a") {
		t.Fatal("wrote after cancel")
	}
}

func TestDefaultLimits(t *testing.T) {
	l := DefaultLimits()
	if l.MaxBytes != 64<<30 || l.MaxEntries != 100000 {
		t.Fatalf("%+v", l)
	}
}
