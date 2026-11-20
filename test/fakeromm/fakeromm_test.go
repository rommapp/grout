package fakeromm

import (
	"context"
	"reflect"
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
