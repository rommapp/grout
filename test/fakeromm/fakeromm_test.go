package fakeromm

import (
	"context"
	"io"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rommapp/grout/core/romm"
)

func TestFakeServerPagingAndUpdatedAfter(t *testing.T) {
	f := New(t)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	f.Platforms = []romm.Platform{{ID: 1, Slug: "gb"}, {ID: 2, Slug: "gba"}}
	for i := 1; i <= 5; i++ {
		f.Roms = append(f.Roms, romm.Rom{ID: i, PlatformID: 1, UpdatedAt: base.Add(time.Duration(i) * time.Hour)})
	}
	f.Roms = append(f.Roms, romm.Rom{ID: 9, PlatformID: 2, UpdatedAt: base})
	ctx := context.Background()

	page, err := f.Client.Roms(ctx, romm.RomsQuery{PlatformID: 1, Limit: 2, Offset: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 || page.Items[0].ID != 3 || page.Items[1].ID != 4 {
		t.Fatalf("page = %+v", page.Items)
	}

	last, err := f.Client.Roms(ctx, romm.RomsQuery{PlatformID: 1, Limit: 2, Offset: 4})
	if err != nil || len(last.Items) != 1 {
		t.Fatalf("last = %+v err = %v", last.Items, err)
	}

	cursor := base.Add(3 * time.Hour).Format(time.RFC3339Nano)
	after, err := f.Client.Roms(ctx, romm.RomsQuery{PlatformID: 1, UpdatedAfter: cursor, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Items) != 2 || after.Items[0].ID != 4 {
		t.Fatalf("strict updated_after returned %+v", after.Items)
	}

	if f.RomPages(1) != 3 {
		t.Fatalf("romPages = %d", f.RomPages(1))
	}
	if len(f.CallsTo("/api/roms")) != 3 {
		t.Fatalf("calls = %+v", f.Calls())
	}
}

func TestFakeServerIdentifiersAndCollections(t *testing.T) {
	f := New(t)
	ctx := context.Background()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	f.Platforms = []romm.Platform{{ID: 3}, {ID: 1}}
	f.Roms = []romm.Rom{{ID: 7, PlatformID: 1}, {ID: 2, PlatformID: 1}}
	f.Collections[romm.CollectionRegular] = []Collection{
		{ID: "10", Name: "a", RomIDs: []int{2}, UpdatedAt: base},
		{ID: "11", Name: "b", RomIDs: []int{7}, UpdatedAt: base.Add(time.Hour)},
	}
	f.Collections[romm.CollectionVirtual] = []Collection{{ID: "abc", Name: "v", RomIDs: []int{2}, UpdatedAt: base}}

	ids, err := f.Client.RomIDs(ctx)
	if err != nil || !reflect.DeepEqual(ids, []int{2, 7}) {
		t.Fatalf("rom ids = %v err = %v", ids, err)
	}
	pids, err := f.Client.PlatformIDs(ctx)
	if err != nil || !reflect.DeepEqual(pids, []int{3, 1}) {
		t.Fatalf("platform ids = %v err = %v", pids, err)
	}
	cids, err := f.Client.CollectionIDs(ctx, romm.CollectionRegular)
	if err != nil || !reflect.DeepEqual(cids, []string{"10", "11"}) {
		t.Fatalf("collection ids = %v err = %v", cids, err)
	}
	vids, err := f.Client.CollectionIDs(ctx, romm.CollectionVirtual)
	if err != nil || !reflect.DeepEqual(vids, []string{"abc"}) {
		t.Fatalf("virtual ids = %v err = %v", vids, err)
	}
	cols, err := f.Client.Collections(ctx, romm.CollectionRegular, base.Format(time.RFC3339Nano))
	if err != nil || len(cols) != 1 || cols[0].RemoteID != "11" {
		t.Fatalf("collections = %+v err = %v", cols, err)
	}
	virt, err := f.Client.Collections(ctx, romm.CollectionVirtual, "")
	if err != nil || len(virt) != 1 || virt[0].RemoteID != "abc" {
		t.Fatalf("virtual = %+v err = %v", virt, err)
	}
}

func TestFakeServerFailureAndHook(t *testing.T) {
	f := New(t)
	ctx := context.Background()
	f.Roms = []romm.Rom{{ID: 1, PlatformID: 5}}
	f.SetFailAfter(5, 2)
	var seen []int
	f.SetBeforePage(func(platformID, offset int) { seen = append(seen, offset) })

	if _, err := f.Client.Roms(ctx, romm.RomsQuery{PlatformID: 5, Limit: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Client.Roms(ctx, romm.RomsQuery{PlatformID: 5, Limit: 1, Offset: 1}); err == nil {
		t.Fatal("second page should fail")
	}
	if !reflect.DeepEqual(seen, []int{0, 1}) {
		t.Fatalf("hook saw %v", seen)
	}

	f.FailPath("/api/platforms")
	if _, err := f.Client.Platforms(ctx); err == nil {
		t.Fatal("forced path should fail")
	}
}

func TestFakeServerContentDetailAndFirmware(t *testing.T) {
	f := New(t)
	ctx := context.Background()
	f.Roms = []romm.Rom{{ID: 1, PlatformID: 2, Name: "plain"}, {ID: 2, PlatformID: 2, Name: "listed"}}
	f.Details = map[int]romm.RomDetail{2: {Rom: romm.Rom{ID: 2, Name: "detailed"}, PathManual: "m.pdf"}}
	f.Content = map[int][]byte{1: []byte("rom-bytes")}
	f.FirmwareList = []romm.Firmware{{ID: 5, PlatformID: 2, FileName: "bios.bin"}, {ID: 6, PlatformID: 3, FileName: "other.bin"}}
	f.FirmwareContent = map[int][]byte{5: []byte("bios")}

	d, err := f.Client.RomDetail(ctx, 1)
	if err != nil || d.Name != "plain" {
		t.Fatalf("fallback detail = %+v err = %v", d.Rom, err)
	}
	d, err = f.Client.RomDetail(ctx, 2)
	if err != nil || d.Name != "detailed" || d.PathManual != "m.pdf" {
		t.Fatalf("detail = %+v err = %v", d, err)
	}
	if _, err := f.Client.RomDetail(ctx, 99); !romm.IsNotFound(err) {
		t.Fatalf("missing detail err = %v", err)
	}

	body, err := f.Client.Download(ctx, romm.RomContentPath(1, "a b.gb"), url.Values{"file_ids": {"3,4"}, "purpose": {"download"}})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(body)
	body.Close()
	if string(data) != "rom-bytes" || body.Size != 9 {
		t.Fatalf("data = %q size = %d", data, body.Size)
	}
	calls := f.CallsTo("/api/roms/1/content/a b.gb")
	if len(calls) != 1 || !strings.Contains(calls[0].Query, "file_ids=3%2C4") {
		t.Fatalf("calls = %+v", calls)
	}
	if _, err := f.Client.Download(ctx, romm.RomContentPath(7, "x"), nil); !romm.IsNotFound(err) {
		t.Fatalf("missing content err = %v", err)
	}

	list, err := f.Client.PlatformFirmware(ctx, 2)
	if err != nil || len(list) != 1 || list[0].ID != 5 {
		t.Fatalf("firmware = %+v err = %v", list, err)
	}
	body, err = f.Client.Download(ctx, romm.FirmwareContentPath(5, "bios.bin"), nil)
	if err != nil {
		t.Fatal(err)
	}
	data, _ = io.ReadAll(body)
	body.Close()
	if string(data) != "bios" || body.Size != 4 {
		t.Fatalf("firmware data = %q size = %d", data, body.Size)
	}
}

func TestFakeServerContentDelayHonorsCancel(t *testing.T) {
	f := New(t)
	f.Content = map[int][]byte{1: []byte("x")}
	f.SetContentDelay(5 * time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := f.Client.Download(ctx, romm.RomContentPath(1, "x"), nil); err == nil {
		t.Fatal("expected cancellation")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("delay did not honor cancel")
	}
}

func TestFakeServerServesAssets(t *testing.T) {
	f := New(t)
	f.Assets = map[string][]byte{"/assets/romm/resources/a.png": []byte("img")}
	body, err := f.Client.Download(context.Background(), "/assets/romm/resources/a.png", nil)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(body)
	body.Close()
	if string(data) != "img" || body.Size != 3 {
		t.Fatalf("data = %q size = %d", data, body.Size)
	}
	if _, err := f.Client.Download(context.Background(), "/assets/romm/resources/none.png", nil); !romm.IsNotFound(err) {
		t.Fatalf("err = %v", err)
	}
}
