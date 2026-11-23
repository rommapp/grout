package fetch

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"path"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/rommapp/grout/core/firmware"
	"github.com/rommapp/grout/core/firmware/host"
	"github.com/rommapp/grout/core/library"
	"github.com/rommapp/grout/core/romm"
	"github.com/rommapp/grout/test/fakeromm"
)

func resolveFW(t *testing.T, id string) *firmware.Firmware {
	t.Helper()
	def, err := firmware.Load(id)
	if err != nil {
		t.Fatal(err)
	}
	fw, err := firmware.Resolve(def, &host.Fake{Files: fstest.MapFS{}}, firmware.Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	return fw
}

func sum(s string) string {
	h := md5.Sum([]byte(s))
	return hex.EncodeToString(h[:])
}

type biosEnv struct {
	w    host.Writable
	root string
	srv  *fakeromm.Server
	fw   *firmware.Firmware
}

func newBiosEnv(t *testing.T, fwID string) *biosEnv {
	t.Helper()
	w, root := newHost(t)
	srv := fakeromm.New(t)
	srv.FirmwareContent = map[int][]byte{}
	return &biosEnv{w: w, root: root, srv: srv, fw: resolveFW(t, fwID)}
}

func (e *biosEnv) input(slug string, entries ...romm.Firmware) BiosInput {
	return BiosInput{PlatformSlug: slug, Entries: entries, Client: e.srv.Client, Firmware: e.fw, Host: e.w}
}

func TestBiosChecksList(t *testing.T) {
	want := []BiosCheck{BiosVerified, BiosMismatched, BiosUnchecked}
	if !reflect.DeepEqual(BiosChecks(), want) {
		t.Fatalf("%v", BiosChecks())
	}
}

func TestBiosDestinationsDumpFolder(t *testing.T) {
	fw := resolveFW(t, "muos")
	got := BiosDestinations(fw, "dc", "dc_boot.bin")
	want := []string{fw.Bios + "/dc/dc_boot.bin"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestBiosDestinationsPerTag(t *testing.T) {
	fw := resolveFW(t, "nextui")
	got := BiosDestinations(fw, "gba", "gba_bios.bin")
	if len(got) == 0 {
		t.Fatal("no destinations")
	}
	seen := map[string]bool{}
	for _, p := range got {
		if !strings.HasPrefix(p, fw.Bios+"/") || !strings.HasSuffix(p, "/gba_bios.bin") {
			t.Fatalf("bad destination %q", p)
		}
		if seen[p] {
			t.Fatalf("duplicate %q", p)
		}
		seen[p] = true
	}
}

func TestBiosVerified(t *testing.T) {
	e := newBiosEnv(t, "muos")
	e.srv.FirmwareContent[5] = []byte("bootdata")
	entry := romm.Firmware{ID: 5, FileName: "dc_boot.bin", MD5: strings.ToUpper(sum("bootdata")), IsVerified: true}
	var events []Progress
	out, err := Bios(context.Background(), e.input("dc", entry), func(p Progress) { events = append(events, p) })
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Check != BiosVerified || !out[0].Present || !out[0].IsVerified || out[0].FirmwareID != 5 {
		t.Fatalf("%+v", out)
	}
	dest := e.fw.Bios + "/dc/dc_boot.bin"
	if !reflect.DeepEqual(out[0].Paths, []string{dest}) {
		t.Fatalf("%v", out[0].Paths)
	}
	if read(t, e.root, dest) != "bootdata" {
		t.Fatal("content")
	}
	noStaging(t, e.root)
	last := events[len(events)-1]
	if last.Stage != library.StageBios || last.Done != 1 || last.Total != 1 || last.BytesDone != 8 {
		t.Fatalf("%+v", last)
	}
	if contentQuery(t, e.srv, romm.FirmwareContentPath(5, "dc_boot.bin")).Get("purpose") != "download" {
		t.Fatal("purpose")
	}
}

func TestBiosFansOutToEveryDestination(t *testing.T) {
	e := newBiosEnv(t, "nextui")
	e.srv.FirmwareContent[9] = []byte("gbabios")
	dests := BiosDestinations(e.fw, "gba", "gba_bios.bin")
	out, err := Bios(context.Background(), e.input("gba", romm.Firmware{ID: 9, FileName: "gba_bios.bin", MD5: sum("gbabios")}), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(out[0].Paths, dests) || !out[0].Present {
		t.Fatalf("%+v", out)
	}
	for _, d := range dests {
		if read(t, e.root, d) != "gbabios" {
			t.Fatalf("%s", d)
		}
	}
	noStaging(t, e.root)
}

func TestBiosMismatchNotPlaced(t *testing.T) {
	e := newBiosEnv(t, "muos")
	e.srv.FirmwareContent[5] = []byte("bootdata")
	out, err := Bios(context.Background(), e.input("dc", romm.Firmware{ID: 5, FileName: "dc_boot.bin", MD5: sum("other")}), nil)
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Check != BiosMismatched || out[0].Present {
		t.Fatalf("%+v", out)
	}
	if exists(e.root, e.fw.Bios+"/dc/dc_boot.bin") {
		t.Fatal("placed")
	}
	noStaging(t, e.root)
}

func TestBiosMismatchKeepsExisting(t *testing.T) {
	e := newBiosEnv(t, "muos")
	dest := e.fw.Bios + "/dc/dc_boot.bin"
	write(t, e.root, dest, "old")
	e.srv.FirmwareContent[5] = []byte("bootdata")
	out, err := Bios(context.Background(), e.input("dc", romm.Firmware{ID: 5, FileName: "dc_boot.bin", MD5: sum("other")}), nil)
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Check != BiosMismatched || !out[0].Present || read(t, e.root, dest) != "old" {
		t.Fatalf("%+v", out)
	}
}

func TestBiosUnchecked(t *testing.T) {
	e := newBiosEnv(t, "muos")
	e.srv.FirmwareContent[5] = []byte("bootdata")
	out, err := Bios(context.Background(), e.input("dc", romm.Firmware{ID: 5, FileName: "dc_boot.bin"}), nil)
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Check != BiosUnchecked || !out[0].Present {
		t.Fatalf("%+v", out)
	}
	if read(t, e.root, e.fw.Bios+"/dc/dc_boot.bin") != "bootdata" {
		t.Fatal("content")
	}
}

func TestBiosUnsafeFileName(t *testing.T) {
	e := newBiosEnv(t, "muos")
	e.srv.FirmwareContent[6] = []byte("good")
	entries := []romm.Firmware{
		{ID: 1, FileName: "../evil.bin"},
		{ID: 2, FileName: ".grout-x"},
		{ID: 3, FileName: ""},
		{ID: 6, FileName: "good.bin"},
	}
	out, err := Bios(context.Background(), e.input("dc", entries...), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 4 {
		t.Fatalf("%+v", out)
	}
	for _, f := range out[:3] {
		if f.Check != BiosUnchecked || len(f.Paths) != 0 || f.Present {
			t.Fatalf("%+v", f)
		}
	}
	if !out[3].Present || read(t, e.root, e.fw.Bios+"/good.bin") != "good" {
		t.Fatalf("%+v", out[3])
	}
	if len(e.srv.Calls()) != 1 {
		t.Fatalf("%+v", e.srv.Calls())
	}
}

func TestBiosTransferFailureKeepsEarlier(t *testing.T) {
	e := newBiosEnv(t, "muos")
	e.srv.FirmwareContent[1] = []byte("one")
	e.srv.FirmwareContent[2] = []byte("two")
	e.srv.FailPath(romm.FirmwareContentPath(2, "b.bin"))
	out, err := Bios(context.Background(), e.input("dc",
		romm.Firmware{ID: 1, FileName: "a.bin"},
		romm.Firmware{ID: 2, FileName: "b.bin"},
		romm.Firmware{ID: 3, FileName: "c.bin"},
	), nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if len(out) != 1 || out[0].FirmwareID != 1 || !out[0].Present {
		t.Fatalf("%+v", out)
	}
	if read(t, e.root, e.fw.Bios+"/a.bin") != "one" {
		t.Fatal("earlier file lost")
	}
	noStaging(t, e.root)
}

func TestBiosStatusPresentNeedsEveryDestination(t *testing.T) {
	e := newBiosEnv(t, "nextui")
	dests := BiosDestinations(e.fw, "gba", "gba_bios.bin")
	if len(dests) < 2 {
		t.Fatalf("need two destinations, have %d", len(dests))
	}
	entries := []romm.Firmware{{ID: 9, FileName: "gba_bios.bin", IsVerified: true}}
	write(t, e.root, dests[0], "x")
	got := BiosStatus(e.w, e.fw, "gba", entries)
	if len(got) != 1 || got[0].Present || got[0].Check != BiosUnchecked || !got[0].IsVerified || !reflect.DeepEqual(got[0].Paths, dests) {
		t.Fatalf("%+v", got)
	}
	for _, d := range dests[1:] {
		write(t, e.root, d, "x")
	}
	got = BiosStatus(e.w, e.fw, "gba", entries)
	if !got[0].Present {
		t.Fatalf("%+v", got)
	}
}

func TestBiosStatusRequiresRegularFile(t *testing.T) {
	e := newBiosEnv(t, "muos")
	dest := e.fw.Bios + "/dc/dc_boot.bin"
	write(t, e.root, dest+"/inner", "x")
	got := BiosStatus(e.w, e.fw, "dc", []romm.Firmware{{ID: 5, FileName: "dc_boot.bin"}})
	if got[0].Present {
		t.Fatalf("%+v", got)
	}
}

func TestBiosStatusWritesNothing(t *testing.T) {
	e := newBiosEnv(t, "muos")
	BiosStatus(e.w, e.fw, "dc", []romm.Firmware{{ID: 5, FileName: "dc_boot.bin"}})
	if exists(e.root, e.fw.Bios) {
		t.Fatal("created")
	}
}

func TestBiosCancel(t *testing.T) {
	t.Run("between entries", func(t *testing.T) {
		e := newBiosEnv(t, "muos")
		e.srv.FirmwareContent[1] = []byte("one")
		e.srv.FirmwareContent[2] = []byte("two")
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		progress := func(p Progress) {
			if p.Stage == library.StageBios && p.Done == 1 {
				cancel()
			}
		}
		out, err := Bios(ctx, e.input("dc",
			romm.Firmware{ID: 1, FileName: "a.bin"},
			romm.Firmware{ID: 2, FileName: "b.bin"},
		), progress)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("%v", err)
		}
		if len(out) != 1 || !exists(e.root, e.fw.Bios+"/a.bin") || exists(e.root, e.fw.Bios+"/b.bin") {
			t.Fatalf("%+v", out)
		}
		noStaging(t, e.root)
	})
	t.Run("mid body", func(t *testing.T) {
		e := newBiosEnv(t, "muos")
		e.srv.FirmwareContent[1] = []byte(strings.Repeat("x", 4<<20))
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		progress := func(p Progress) {
			if p.BytesDone > 0 {
				cancel()
			}
		}
		out, err := Bios(ctx, e.input("dc", romm.Firmware{ID: 1, FileName: "a.bin"}), progress)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("%v", err)
		}
		if len(out) != 0 || exists(e.root, e.fw.Bios+"/a.bin") {
			t.Fatalf("%+v", out)
		}
		noStaging(t, e.root)
	})
}

func TestBiosLaterDestinationFailureReportsPlaced(t *testing.T) {
	e := newBiosEnv(t, "nextui")
	e.srv.FirmwareContent[9] = []byte("gbabios")
	dests := BiosDestinations(e.fw, "gba", "gba_bios.bin")
	if len(dests) < 2 {
		t.Fatalf("need two destinations, have %d", len(dests))
	}
	write(t, e.root, path.Dir(dests[1]), "blocker")
	out, err := Bios(context.Background(), e.input("gba", romm.Firmware{ID: 9, FileName: "gba_bios.bin"}), nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if len(out) != 1 || !reflect.DeepEqual(out[0].Paths, dests[:1]) || !out[0].Present {
		t.Fatalf("%+v", out)
	}
	if read(t, e.root, dests[0]) != "gbabios" {
		t.Fatal("first destination lost")
	}
	noStaging(t, e.root)
}

func TestBiosThrottlesProgressWithClock(t *testing.T) {
	e := newBiosEnv(t, "muos")
	e.srv.FirmwareContent[1] = []byte(strings.Repeat("x", 1<<20))
	in := e.input("dc", romm.Firmware{ID: 1, FileName: "a.bin"})
	in.Clock = func() time.Time { return time.Unix(0, 0) }
	var events []Progress
	if _, err := Bios(context.Background(), in, func(p Progress) { events = append(events, p) }); err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[1].Done != 1 || events[1].BytesDone != 1<<20 {
		t.Fatalf("%+v", events)
	}
}

func TestBiosSkipsEntryMissingOnServer(t *testing.T) {
	e := newBiosEnv(t, "muos")
	gone := e.fw.Bios + "/a.bin"
	write(t, e.root, gone, "old")
	e.srv.FirmwareContent[2] = []byte("two")
	out, err := Bios(context.Background(), e.input("dc",
		romm.Firmware{ID: 1, FileName: "a.bin", MD5: sum("new")},
		romm.Firmware{ID: 2, FileName: "b.bin", MD5: sum("two")},
	), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("%+v", out)
	}
	if out[0].FirmwareID != 1 || out[0].Check != BiosUnchecked || !out[0].Present || !reflect.DeepEqual(out[0].Paths, []string{gone}) {
		t.Fatalf("%+v", out[0])
	}
	if out[1].FirmwareID != 2 || out[1].Check != BiosVerified || !out[1].Present {
		t.Fatalf("%+v", out[1])
	}
	if read(t, e.root, gone) != "old" || read(t, e.root, e.fw.Bios+"/b.bin") != "two" {
		t.Fatal("content")
	}
	noStaging(t, e.root)
}
