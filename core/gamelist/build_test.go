package gamelist

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/rommapp/grout/core/firmware"
	"github.com/rommapp/grout/core/firmware/host"
	"github.com/rommapp/grout/core/library"
)

func newHost(t *testing.T) (host.Writable, string) {
	t.Helper()
	dir := t.TempDir()
	return host.OS(func(k string) string {
		if k == "GROUT_ROOT" {
			return dir
		}
		return ""
	}), dir
}

func put(t *testing.T, root, p, s string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(p))
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(s), 0644); err != nil {
		t.Fatal(err)
	}
}

func get(t *testing.T, root, p string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func loadFW(t *testing.T, id string) *firmware.Firmware {
	t.Helper()
	def, err := firmware.Load(id)
	if err != nil {
		t.Fatal(err)
	}
	files := fstest.MapFS{"mnt/union/ROMS": &fstest.MapFile{Mode: fs.ModeDir | 0o755}}
	fw, err := firmware.Resolve(def, &host.Fake{Files: files}, firmware.Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	return fw
}

func gbaGame(t *testing.T, fw *firmware.Firmware, id int, file string) Game {
	t.Helper()
	dirs := fw.RomDirs("gba")
	if len(dirs) == 0 {
		t.Fatal("no gba rom dir")
	}
	romDir := dirs[0]
	return Game{
		Meta:       library.GameMeta{GameID: id, PlatformID: 7, Name: "Game", MD5ByName: map[string]string{}},
		Platform:   library.Platform{ID: 7, Slug: "gba", FSSlug: "gba", Name: "Game Boy Advance", RomDirs: []string{romDir}},
		RomDir:     romDir,
		LaunchPath: romDir + "/" + file,
	}
}

func TestBuildNameWithRegions(t *testing.T) {
	fw := loadFW(t, "knulli")
	w, _ := newHost(t)
	g := gbaGame(t, fw, 1, "Game.gba")
	g.Meta.Regions = []string{"USA", "Europe"}
	e, err := Build(fw, w, g, false)
	if err != nil {
		t.Fatal(err)
	}
	if e.Name != "Game (USA, Europe)" {
		t.Fatalf("Name = %q", e.Name)
	}
	if !reflect.DeepEqual(e.Regions, []string{"USA", "Europe"}) {
		t.Fatalf("Regions = %v", e.Regions)
	}
}

func TestBuildNameOmitRegion(t *testing.T) {
	fw := loadFW(t, "knulli")
	w, _ := newHost(t)
	g := gbaGame(t, fw, 1, "Game.gba")
	g.Meta.Regions = []string{"USA", "Europe"}
	e, err := Build(fw, w, g, true)
	if err != nil {
		t.Fatal(err)
	}
	if e.Name != "Game" {
		t.Fatalf("Name = %q", e.Name)
	}
}

func TestBuildNameFallsBackToStem(t *testing.T) {
	fw := loadFW(t, "knulli")
	w, _ := newHost(t)
	g := gbaGame(t, fw, 1, "Some Game (Rev 1).gba")
	g.Meta.Name = ""
	e, err := Build(fw, w, g, true)
	if err != nil {
		t.Fatal(err)
	}
	if e.Name != "Some Game (Rev 1)" {
		t.Fatalf("Name = %q", e.Name)
	}
}

func TestBuildRelativePathAndMD5(t *testing.T) {
	fw := loadFW(t, "knulli")
	w, _ := newHost(t)
	g := gbaGame(t, fw, 1, "Multi/Game (Disc).m3u")
	g.Meta.MD5ByName = map[string]string{"Game (Disc).m3u": "abc123", "other.bin": "zzz"}
	g.Meta.Summary = "About"
	g.Meta.Companies = []string{"Dev A", "Dev B"}
	e, err := Build(fw, w, g, false)
	if err != nil {
		t.Fatal(err)
	}
	if e.Path != "./Multi/Game (Disc).m3u" {
		t.Fatalf("Path = %q", e.Path)
	}
	if e.MD5 != "abc123" {
		t.Fatalf("MD5 = %q", e.MD5)
	}
	if e.Desc != "About" || !reflect.DeepEqual(e.Developers, []string{"Dev A", "Dev B"}) {
		t.Fatalf("Desc = %q, Developers = %v", e.Desc, e.Developers)
	}
}

func TestBuildArtPresentOnly(t *testing.T) {
	fw := loadFW(t, "knulli")
	w, root := newHost(t)
	g := gbaGame(t, fw, 1, "Game.gba")
	coverDir, ok := fw.ArtDir(firmware.ArtCover, "gba", g.RomDir, g.Platform.Name)
	if !ok {
		t.Fatal("no cover dir")
	}
	if _, ok := fw.ArtDir(firmware.ArtVideo, "gba", g.RomDir, g.Platform.Name); !ok {
		t.Fatal("no video dir")
	}
	put(t, root, coverDir+"/"+fw.ArtFileName(firmware.ArtCover, "Game"), "png")
	e, err := Build(fw, w, g, false)
	if err != nil {
		t.Fatal(err)
	}
	want := map[firmware.ArtKind]string{firmware.ArtCover: "./images/Game.png"}
	if !reflect.DeepEqual(e.Art, want) {
		t.Fatalf("Art = %v, want %v", e.Art, want)
	}
}

func TestBuildArtOutsideRomDirNotListed(t *testing.T) {
	fw := loadFW(t, "muos")
	w, root := newHost(t)
	g := gbaGame(t, fw, 1, "Game.gba")
	coverDir, ok := fw.ArtDir(firmware.ArtCover, "gba", g.RomDir, g.Platform.Name)
	if !ok {
		t.Fatal("no cover dir")
	}
	put(t, root, coverDir+"/"+fw.ArtFileName(firmware.ArtCover, "Game"), "png")
	e, err := Build(fw, w, g, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Art) != 0 {
		t.Fatalf("Art = %v, want none", e.Art)
	}
}

func TestBuildFolderIsNoPlaylist(t *testing.T) {
	fw := loadFW(t, "knulli")
	w, _ := newHost(t)
	g := gbaGame(t, fw, 1, "Game")
	g.Folder = true
	if _, err := Build(fw, w, g, false); !errors.Is(err, ErrNoPlaylist) {
		t.Fatalf("err = %v, want ErrNoPlaylist", err)
	}
}
