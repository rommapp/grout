package fetch

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rommapp/grout/core/firmware/host"
)

func newHost(t *testing.T) (host.Writable, string) {
	dir := t.TempDir()
	return host.OS(func(k string) string {
		if k == "GROUT_ROOT" {
			return dir
		}
		return ""
	}), dir
}

func write(t *testing.T, root, p, s string) {
	full := filepath.Join(root, filepath.FromSlash(p))
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(s), 0644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, root, p string) string {
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func exists(root, p string) bool {
	_, err := os.Stat(filepath.Join(root, filepath.FromSlash(p)))
	return err == nil
}

func TestStageFileThenCommitReplaces(t *testing.T) {
	w, root := newHost(t)
	write(t, root, "/roms/gba/game.gba", "old")
	f, staged, err := StageFile(w, "/roms/gba/game.gba")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(filepath.Base(staged), StagePrefix) || !strings.HasSuffix(staged, "-game.gba") || filepath.Dir(staged) != "/roms/gba" {
		t.Fatalf("%q", staged)
	}
	f.WriteString("new")
	f.Close()
	if read(t, root, "/roms/gba/game.gba") != "old" {
		t.Fatal("final path touched early")
	}
	if err := Commit(w, staged, "/roms/gba/game.gba"); err != nil {
		t.Fatal(err)
	}
	if read(t, root, "/roms/gba/game.gba") != "new" || exists(root, staged) {
		t.Fatal("commit failed")
	}
}

func TestStageFileCreatesParent(t *testing.T) {
	w, root := newHost(t)
	f, staged, err := StageFile(w, "/a/b/c.bin")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if err := Commit(w, staged, "/a/b/c.bin"); err != nil || !exists(root, "/a/b/c.bin") {
		t.Fatalf("%v", err)
	}
}

func TestCommitCreatesDestParent(t *testing.T) {
	w, root := newHost(t)
	write(t, root, "/stage/x", "v")
	if err := Commit(w, "/stage/x", "/new/dir/x"); err != nil || read(t, root, "/new/dir/x") != "v" {
		t.Fatalf("%v", err)
	}
}

func TestDiscardRemovesStaging(t *testing.T) {
	w, root := newHost(t)
	f, staged, err := StageFile(w, "/roms/x.bin")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	d, err := StageDir(w, "/roms/gba")
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, d+"/inner", "x")
	Discard(w, staged)
	Discard(w, d)
	if exists(root, staged) || exists(root, d) {
		t.Fatal("staging remains")
	}
}

func TestMoveMergeKeepsOtherFiles(t *testing.T) {
	w, root := newHost(t)
	write(t, root, "/roms/gba/game.srm", "save")
	write(t, root, "/roms/gba/game.gba", "old")
	write(t, root, "/stage/game.gba", "new")
	write(t, root, "/stage/disc/2.bin", "two")
	placed, err := MoveMerge(w, "/stage", "/roms/gba")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/roms/gba/disc/2.bin", "/roms/gba/game.gba"}
	if !reflect.DeepEqual(placed, want) {
		t.Fatalf("%v", placed)
	}
	if read(t, root, "/roms/gba/game.srm") != "save" || read(t, root, "/roms/gba/game.gba") != "new" || read(t, root, "/roms/gba/disc/2.bin") != "two" {
		t.Fatal("bad merge")
	}
	if exists(root, "/stage") {
		t.Fatal("src remains")
	}
}

func TestSweepStagingRemovesOnlyStaging(t *testing.T) {
	w, root := newHost(t)
	write(t, root, "/r/.grout-a", "x")
	write(t, root, "/r/sub/.grout-b", "x")
	write(t, root, "/r/.hidden", "x")
	write(t, root, "/r/game.gba", "x")
	n, err := SweepStaging(w, "/r")
	if err != nil || n != 2 {
		t.Fatalf("%d %v", n, err)
	}
	if exists(root, "/r/.grout-a") || exists(root, "/r/sub/.grout-b") || !exists(root, "/r/.hidden") || !exists(root, "/r/game.gba") {
		t.Fatal("wrong removal")
	}
	if n, err := SweepStaging(w, "/missing"); err != nil || n != 0 {
		t.Fatalf("%d %v", n, err)
	}
}

func TestMoveMergeReturnsPlacedOnError(t *testing.T) {
	w, root := newHost(t)
	write(t, root, "/stage/a.bin", "a")
	write(t, root, "/stage/b.bin", "b")
	write(t, root, "/dest/b.bin/keep", "k")
	placed, err := MoveMerge(w, "/stage", "/dest")
	if err == nil {
		t.Fatal("expected error")
	}
	if !reflect.DeepEqual(placed, []string{"/dest/a.bin"}) {
		t.Fatalf("%v", placed)
	}
	if read(t, root, "/dest/a.bin") != "a" || !exists(root, "/stage/b.bin") {
		t.Fatal("bad state")
	}
}

func TestStageDirName(t *testing.T) {
	w, root := newHost(t)
	d, err := StageDir(w, "/roms/gba")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(filepath.Base(d), StagePrefix) || filepath.Dir(d) != "/roms/gba" || !exists(root, d) {
		t.Fatalf("%q", d)
	}
}
