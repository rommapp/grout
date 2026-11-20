package romm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestCollectionsKindsAndIDs(t *testing.T) {
	queries := map[string]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries[r.URL.Path] = r.URL.RawQuery
		switch r.URL.Path {
		case "/api/collections":
			w.Write([]byte(`[{"id":7,"name":"Faves","rom_ids":[1,2],"updated_at":"2026-11-19T10:00:00Z"}]`))
		case "/api/collections/smart":
			w.Write([]byte(`[{"id":9,"name":"Smart","rom_ids":[3],"updated_at":"2026-11-19T10:00:00Z"}]`))
		case "/api/collections/virtual":
			w.Write([]byte(`[{"id":"recent","name":"Recent","rom_ids":[4],"updated_at":"2026-11-19T10:00:00Z"}]`))
		default:
			t.Errorf("path=%s", r.URL.Path)
		}
	}))
	defer srv.Close()
	c := New(Options{BaseURL: srv.URL})
	cases := []struct {
		kind CollectionKind
		id   string
		ids  []int
	}{
		{CollectionRegular, "7", []int{1, 2}},
		{CollectionSmart, "9", []int{3}},
		{CollectionVirtual, "recent", []int{4}},
	}
	for _, tc := range cases {
		got, err := c.Collections(context.Background(), tc.kind, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].Kind != tc.kind || got[0].RemoteID != tc.id || !reflect.DeepEqual(got[0].RomIDs, tc.ids) || got[0].UpdatedAt.IsZero() {
			t.Fatalf("%s: got=%+v", tc.kind, got)
		}
	}
	if queries["/api/collections/virtual"] != "type=collection" {
		t.Fatalf("virtual query=%q", queries["/api/collections/virtual"])
	}
}

func TestCollectionsUpdatedAfterOnRegularAndSmart(t *testing.T) {
	var query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	c := New(Options{BaseURL: srv.URL})
	for _, kind := range []CollectionKind{CollectionRegular, CollectionSmart} {
		if _, err := c.Collections(context.Background(), kind, "x"); err != nil {
			t.Fatal(err)
		}
		if query != "updated_after=x" {
			t.Fatalf("%s query=%q", kind, query)
		}
	}
}

func TestCollectionsVirtualIgnoresUpdatedAfter(t *testing.T) {
	var query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	if _, err := New(Options{BaseURL: srv.URL}).Collections(context.Background(), CollectionVirtual, "x"); err != nil {
		t.Fatal(err)
	}
	if query != "type=collection" {
		t.Fatalf("query=%q", query)
	}
}

func TestCollectionIDs(t *testing.T) {
	paths := []string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.URL.Path == "/api/collections/virtual/identifiers" {
			w.Write([]byte(`["recent","top"]`))
			return
		}
		w.Write([]byte(`[1,2]`))
	}))
	defer srv.Close()
	c := New(Options{BaseURL: srv.URL})
	v, err := c.CollectionIDs(context.Background(), CollectionVirtual)
	if err != nil || !reflect.DeepEqual(v, []string{"recent", "top"}) {
		t.Fatalf("v=%v err=%v", v, err)
	}
	r, err := c.CollectionIDs(context.Background(), CollectionRegular)
	if err != nil || !reflect.DeepEqual(r, []string{"1", "2"}) {
		t.Fatalf("r=%v err=%v", r, err)
	}
	s, err := c.CollectionIDs(context.Background(), CollectionSmart)
	if err != nil || !reflect.DeepEqual(s, []string{"1", "2"}) {
		t.Fatalf("s=%v err=%v", s, err)
	}
	want := []string{"/api/collections/virtual/identifiers", "/api/collections/identifiers", "/api/collections/smart/identifiers"}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("paths=%v", paths)
	}
}
