package gamelist

import (
	"context"
	"os"
	"reflect"
	"testing"
)

func TestLauncherKnulli(t *testing.T) {
	fw := loadFW(t, "knulli")
	w, root := newHost(t)
	file := "/userdata/roms/tools/gamelist.xml"
	put(t, root, file, "<?xml version=\"1.0\"?>\n<gameList>\n    <game>\n        <path>./Other.sh</path>\n        <name>Other</name>\n    </game>\n</gameList>\n")
	res, err := NewWriter(fw, w).Launcher(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(res.Written, []string{file}) || res.Failed == nil || len(res.Failed) != 0 {
		t.Fatalf("res = %+v", res)
	}
	games := readGames(t, get(t, root, file))
	if len(games) != 2 || text(games["./Other.sh"], "name") != "Other" {
		t.Fatalf("%s", get(t, root, file))
	}
	g, ok := games["./Grout/Grout.sh"]
	if !ok {
		t.Fatalf("%s", get(t, root, file))
	}
	want := map[string]string{
		"name":      "Grout",
		"desc":      "Download games wirelessly from your RomM instance",
		"players":   "1",
		"genre":     "Rom Manager",
		"developer": "The RomM Community",
		"image":     "./Grout/logo.png",
	}
	for tag, v := range want {
		if got := text(g, tag); got != v {
			t.Fatalf("%s = %q, want %q", tag, got, v)
		}
	}
}

func TestLauncherUnchangedSecondTime(t *testing.T) {
	fw := loadFW(t, "knulli")
	w, root := newHost(t)
	wr := NewWriter(fw, w)
	first, err := wr.Launcher(context.Background())
	if err != nil || len(first.Written) != 1 {
		t.Fatalf("first = %+v, %v", first, err)
	}
	before := get(t, root, "/userdata/roms/tools/gamelist.xml")
	second, err := wr.Launcher(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	emptyResult(t, second)
	if get(t, root, "/userdata/roms/tools/gamelist.xml") != before {
		t.Fatal("file changed")
	}
}

func TestLauncherNoneForMuOS(t *testing.T) {
	fw := loadFW(t, "muos")
	w, root := newHost(t)
	res, err := NewWriter(fw, w).Launcher(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	emptyResult(t, res)
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("card has %v", entries)
	}
}
