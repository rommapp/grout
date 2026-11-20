package romm

import (
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

const romsBaseQuery = "limit=250&offset=500&order_by=id&order_dir=asc&platform_ids=3&with_char_index=false&with_files=true&with_filter_values=false&with_provider_metadata=false&with_rom_id_index=false&with_total=false"

func TestRomsQueryEncoding(t *testing.T) {
	var path, query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, query = r.URL.Path, r.URL.RawQuery
		w.Write([]byte(`{"items":[],"limit":250,"offset":500}`))
	}))
	defer srv.Close()
	c := New(Options{BaseURL: srv.URL})
	if _, err := c.Roms(context.Background(), RomsQuery{PlatformID: 3, Limit: 250, Offset: 500}); err != nil {
		t.Fatal(err)
	}
	if path != "/api/roms" || query != romsBaseQuery {
		t.Fatalf("path=%q query=%q", path, query)
	}
	if _, err := c.Roms(context.Background(), RomsQuery{PlatformID: 3, Limit: 250, Offset: 500, UpdatedAfter: "2026-11-19T10:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if query != "limit=250&offset=500&order_by=id&order_dir=asc&platform_ids=3&updated_after=2026-11-19T10%3A00%3A00Z&with_char_index=false&with_files=true&with_filter_values=false&with_provider_metadata=false&with_rom_id_index=false&with_total=false" {
		t.Fatalf("query=%q", query)
	}
}

func TestRomsWithTotal(t *testing.T) {
	var query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		w.Write([]byte(`{"items":[],"total":42,"limit":250,"offset":0}`))
	}))
	defer srv.Close()
	page, err := New(Options{BaseURL: srv.URL}).Roms(context.Background(), RomsQuery{PlatformID: 3, Limit: 250, Offset: 500, WithTotal: true})
	if err != nil {
		t.Fatal(err)
	}
	if query != "limit=250&offset=500&order_by=id&order_dir=asc&platform_ids=3&with_char_index=false&with_files=true&with_filter_values=false&with_provider_metadata=false&with_rom_id_index=false&with_total=true" {
		t.Fatalf("query=%q", query)
	}
	if page.Total != 42 {
		t.Fatalf("total=%d", page.Total)
	}
}

func TestRomsServerTime(t *testing.T) {
	sent := time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC)
	date := sent.Format(http.TimeFormat)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if date == "" {
			w.Header()["Date"] = nil
		} else {
			w.Header().Set("Date", date)
		}
		w.Write([]byte(`{"items":[],"limit":250,"offset":0}`))
	}))
	defer srv.Close()
	c := New(Options{BaseURL: srv.URL})
	page, err := c.Roms(context.Background(), RomsQuery{Limit: 250})
	if err != nil {
		t.Fatal(err)
	}
	if !page.ServerTime.Equal(sent) {
		t.Fatalf("server time = %v", page.ServerTime)
	}
	for _, d := range []string{"", "not a date"} {
		date = d
		page, err := c.Roms(context.Background(), RomsQuery{Limit: 250})
		if err != nil {
			t.Fatal(err)
		}
		if !page.ServerTime.IsZero() {
			t.Fatalf("date %q server time = %v", d, page.ServerTime)
		}
	}
}

func TestRequestsAcceptGzip(t *testing.T) {
	var accept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accept = r.Header.Get("Accept-Encoding")
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		gz.Write([]byte(`{"items":[{"id":9,"name":"Tetris"}],"limit":250,"offset":0}`))
		gz.Close()
	}))
	defer srv.Close()
	page, err := New(Options{BaseURL: srv.URL}).Roms(context.Background(), RomsQuery{Limit: 250})
	if err != nil {
		t.Fatal(err)
	}
	if accept != "gzip" {
		t.Fatalf("accept=%q", accept)
	}
	if len(page.Items) != 1 || page.Items[0].ID != 9 || page.Items[0].Name != "Tetris" {
		t.Fatalf("page=%+v", page)
	}
}

func TestRomsDecodesFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"items":[{
			"id":1,"platform_id":3,"name":null,"fs_name_no_ext":"Tetris (USA)","fs_extension":"gb",
			"fs_size_bytes":32768,"regions":["USA"],"has_simple_single_file":true,
			"metadatum":{"genres":["Puzzle"],"first_release_date":631152000000,"average_rating":82.5},
			"files":[
				{"id":1,"file_name":"Tetris (USA).gb","category":null,"file_size_bytes":32768,"md5_hash":"abc"},
				{"id":2,"file_name":"Tetris.pdf","category":"manual"}
			],
			"updated_at":"2026-11-19T10:00:00Z"
		}],"limit":250,"offset":0}`))
	}))
	defer srv.Close()
	page, err := New(Options{BaseURL: srv.URL}).Roms(context.Background(), RomsQuery{Limit: 250})
	if err != nil {
		t.Fatal(err)
	}
	r := page.Items[0]
	if r.DisplayName() != "Tetris (USA)" {
		t.Fatalf("display=%q", r.DisplayName())
	}
	if len(r.PlayableFiles()) != 1 || r.PlayableFiles()[0].ID != 1 {
		t.Fatalf("playable=%+v", r.PlayableFiles())
	}
	if r.Metadatum.FirstReleaseDate == nil || *r.Metadatum.FirstReleaseDate != 631152000000 {
		t.Fatalf("release=%v", r.Metadatum.FirstReleaseDate)
	}
	if r.Metadatum.AverageRating == nil || *r.Metadatum.AverageRating != 82.5 {
		t.Fatalf("rating=%v", r.Metadatum.AverageRating)
	}
	if r.PlatformID != 3 || r.FSSizeBytes != 32768 || !r.HasSimpleSingleFile || r.UpdatedAt.IsZero() {
		t.Fatalf("rom=%+v", r)
	}
}

func TestDisplayNamePrefersName(t *testing.T) {
	if (Rom{Name: "A", FSNameNoExt: "B"}).DisplayName() != "A" {
		t.Fatal("name")
	}
}

func TestPlayableFilesKeepsAllWhenEveryFileIsExtra(t *testing.T) {
	r := Rom{Files: []RomFile{{ID: 1, Category: "manual"}, {ID: 2, Category: "manual"}}}
	if len(r.PlayableFiles()) != 2 {
		t.Fatalf("playable=%+v", r.PlayableFiles())
	}
}

func TestRomIDs(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Write([]byte(`[1,2,3]`))
	}))
	defer srv.Close()
	got, err := New(Options{BaseURL: srv.URL}).RomIDs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if path != "/api/roms/identifiers" || !reflect.DeepEqual(got, []int{1, 2, 3}) {
		t.Fatalf("path=%q got=%v", path, got)
	}
}
