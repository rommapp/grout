package gamelist

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/beevik/etree"

	"github.com/rommapp/grout/core/fetch"
	"github.com/rommapp/grout/core/firmware/host"
	"github.com/rommapp/grout/core/gamelist/muostext"
)

type recordingHost struct {
	host.Writable
	mu   sync.Mutex
	real []string
}

func (h *recordingHost) Real(p string) (string, error) {
	h.mu.Lock()
	h.real = append(h.real, p)
	h.mu.Unlock()
	return h.Writable.Real(p)
}

func (h *recordingHost) staged() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []string
	for _, p := range h.real {
		if strings.HasPrefix(path.Base(p), fetch.StagePrefix) {
			out = append(out, p)
		}
	}
	return out
}

func readGames(t *testing.T, data string) map[string]*etree.Element {
	t.Helper()
	doc := etree.NewDocument()
	if err := doc.ReadFromString(data); err != nil {
		t.Fatalf("%v:\n%s", err, data)
	}
	out := map[string]*etree.Element{}
	for _, g := range doc.FindElements("/gameList/game") {
		p := g.SelectElement("path")
		if p == nil {
			t.Fatalf("game without path:\n%s", data)
		}
		if _, dup := out[p.Text()]; dup {
			t.Fatalf("duplicate %s:\n%s", p.Text(), data)
		}
		out[p.Text()] = g
	}
	return out
}

func text(g *etree.Element, tag string) string {
	if e := g.SelectElement(tag); e != nil {
		return e.Text()
	}
	return ""
}

func emptyResult(t *testing.T, r Result) {
	t.Helper()
	if r.Written == nil || r.Failed == nil {
		t.Fatalf("nil slices in %+v", r)
	}
	if len(r.Written) != 0 || len(r.Failed) != 0 {
		t.Fatalf("result = %+v, want empty", r)
	}
}

func TestWriterESWritesAndReportsFile(t *testing.T) {
	fw := loadFW(t, "knulli")
	w, root := newHost(t)
	g := gbaGame(t, fw, 1, "Game.gba")
	g.Meta.Summary = "About"
	res := NewWriter(fw, w).Write(context.Background(), []Game{g}, false)
	file := g.RomDir + "/gamelist.xml"
	if !reflect.DeepEqual(res.Written, []string{file}) {
		t.Fatalf("Written = %v", res.Written)
	}
	if res.Failed == nil || len(res.Failed) != 0 {
		t.Fatalf("Failed = %#v", res.Failed)
	}
	games := readGames(t, get(t, root, file))
	e, ok := games["./Game.gba"]
	if !ok || text(e, "name") != "Game" || text(e, "desc") != "About" {
		t.Fatalf("games = %v\n%s", games, get(t, root, file))
	}
}

func TestWriterMiyooFile(t *testing.T) {
	fw := loadFW(t, "onion")
	w, root := newHost(t)
	g := gbaGame(t, fw, 1, "Game.gba")
	g.Meta.Summary = "About"
	res := NewWriter(fw, w).Write(context.Background(), []Game{g}, false)
	file := g.RomDir + "/miyoogamelist.xml"
	if !reflect.DeepEqual(res.Written, []string{file}) {
		t.Fatalf("Written = %v", res.Written)
	}
	games := readGames(t, get(t, root, file))
	e, ok := games["./Game.gba"]
	if !ok || text(e, "name") != "Game" || e.SelectElement("desc") != nil {
		t.Fatalf("%s", get(t, root, file))
	}
}

func TestWriterMuOSTextFile(t *testing.T) {
	fw := loadFW(t, "muos")
	w, root := newHost(t)
	g := gbaGame(t, fw, 1, "Game.gba")
	g.Meta.Summary = "About"
	g.Meta.Genres = []string{"Action"}
	res := NewWriter(fw, w).Write(context.Background(), []Game{g}, false)
	file := "/run/muos/storage/info/catalogue/Nintendo Game Boy Advance/text/Game.txt"
	if !reflect.DeepEqual(res.Written, []string{file}) {
		t.Fatalf("Written = %v, Failed = %v", res.Written, res.Failed)
	}
	e, err := Build(fw, w, g, false)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := get(t, root, file), string(muostext.Render(e)); got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
	if !strings.Contains(get(t, root, file), "Genres: Action\n") {
		t.Fatalf("text = %q", get(t, root, file))
	}
}

func TestWriterNoneWritesNothing(t *testing.T) {
	fw := loadFW(t, "nextui")
	w, root := newHost(t)
	g := gbaGame(t, fw, 1, "Game.gba")
	res := NewWriter(fw, w).Write(context.Background(), []Game{g}, false)
	emptyResult(t, res)
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("card has %v", entries)
	}
}

func TestWriterUnchangedNotWritten(t *testing.T) {
	fw := loadFW(t, "knulli")
	base, root := newHost(t)
	rec := &recordingHost{Writable: base}
	g := gbaGame(t, fw, 1, "Game.gba")
	wr := NewWriter(fw, rec)
	first := wr.Write(context.Background(), []Game{g}, false)
	if len(first.Written) != 1 {
		t.Fatalf("first = %+v", first)
	}
	file := g.RomDir + "/gamelist.xml"
	before := get(t, root, file)
	stagedBefore := len(rec.staged())
	second := wr.Write(context.Background(), []Game{g}, false)
	emptyResult(t, second)
	if got := get(t, root, file); got != before {
		t.Fatalf("file changed:\n%s\nwas\n%s", got, before)
	}
	if n := len(rec.staged()); n != stagedBefore {
		t.Fatalf("second write staged %d files", n-stagedBefore)
	}
	if len(readGames(t, before)) != 1 {
		t.Fatalf("%s", before)
	}
}

func TestWriterUnparseableLeftAlone(t *testing.T) {
	fw := loadFW(t, "knulli")
	w, root := newHost(t)
	g := gbaGame(t, fw, 1, "Game.gba")
	file := g.RomDir + "/gamelist.xml"
	broken := "<?xml version=\"1.0\"?>\n<gameList>\n    <game>\n        <path>./Other.gba</path>\n        <na"
	put(t, root, file, broken)
	res := NewWriter(fw, w).Write(context.Background(), []Game{g}, false)
	if res.Written == nil || len(res.Written) != 0 {
		t.Fatalf("Written = %#v", res.Written)
	}
	if len(res.Failed) != 1 || res.Failed[0].GameID != 1 || res.Failed[0].Message == "" {
		t.Fatalf("Failed = %+v", res.Failed)
	}
	if got := get(t, root, file); got != broken {
		t.Fatalf("file changed: %q", got)
	}
}

func TestWriterBuildFailureReported(t *testing.T) {
	fw := loadFW(t, "knulli")
	w, root := newHost(t)
	bad := gbaGame(t, fw, 1, "Folder")
	bad.Folder = true
	good := gbaGame(t, fw, 2, "Good.gba")
	res := NewWriter(fw, w).Write(context.Background(), []Game{bad, good}, false)
	if len(res.Failed) != 1 || res.Failed[0].GameID != 1 || !strings.Contains(res.Failed[0].Message, ErrNoPlaylist.Error()) {
		t.Fatalf("Failed = %+v", res.Failed)
	}
	if len(res.Written) != 1 {
		t.Fatalf("Written = %v", res.Written)
	}
	games := readGames(t, get(t, root, good.RomDir+"/gamelist.xml"))
	if _, ok := games["./Good.gba"]; !ok || len(games) != 1 {
		t.Fatalf("games = %v", games)
	}
}

func TestWriterConcurrentSameFile(t *testing.T) {
	fw := loadFW(t, "knulli")
	w, root := newHost(t)
	wr := NewWriter(fw, w)
	var wg sync.WaitGroup
	var romDir string
	for i := range 8 {
		g := gbaGame(t, fw, i+1, fmt.Sprintf("Game %d.gba", i))
		g.Meta.Name = fmt.Sprintf("Game %d", i)
		romDir = g.RomDir
		wg.Add(1)
		go func() {
			defer wg.Done()
			res := wr.Write(context.Background(), []Game{g}, false)
			if len(res.Failed) != 0 {
				t.Errorf("Failed = %+v", res.Failed)
			}
		}()
	}
	wg.Wait()
	games := readGames(t, get(t, root, romDir+"/gamelist.xml"))
	if len(games) != 8 {
		t.Fatalf("games = %d, want 8", len(games))
	}
	for i := range 8 {
		g, ok := games[fmt.Sprintf("./Game %d.gba", i)]
		if !ok || text(g, "name") != fmt.Sprintf("Game %d", i) {
			t.Fatalf("game %d missing", i)
		}
	}
}

func TestWriterUsesStaging(t *testing.T) {
	fw := loadFW(t, "knulli")
	base, root := newHost(t)
	rec := &recordingHost{Writable: base}
	g := gbaGame(t, fw, 1, "Game.gba")
	res := NewWriter(fw, rec).Write(context.Background(), []Game{g}, false)
	if len(res.Written) != 1 {
		t.Fatalf("res = %+v", res)
	}
	staged := rec.staged()
	if len(staged) == 0 {
		t.Fatal("no staged file used")
	}
	for _, s := range staged {
		if path.Dir(s) != g.RomDir || !strings.HasSuffix(s, "-gamelist.xml") {
			t.Fatalf("staged %q", s)
		}
	}
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(g.RomDir)))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), fetch.StagePrefix) {
			t.Fatalf("left %s", e.Name())
		}
	}
}
