package romm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestPlatformsSendsNoQuery(t *testing.T) {
	var path, query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, query = r.URL.Path, r.URL.RawQuery
		w.Write([]byte(`[{"id":1,"slug":"gb","fs_slug":"gb","name":"Game Boy","custom_name":null,"rom_count":4,"updated_at":"2026-11-19T10:00:00Z"}]`))
	}))
	defer srv.Close()
	got, err := New(Options{BaseURL: srv.URL}).Platforms(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if path != "/api/platforms" || query != "" {
		t.Fatalf("path=%q query=%q", path, query)
	}
	if len(got) != 1 || got[0].ID != 1 || got[0].FSSlug != "gb" || got[0].RomCount != 4 || got[0].UpdatedAt.IsZero() {
		t.Fatalf("got=%+v", got)
	}
}

func TestPlatformIDs(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Write([]byte(`[3,5,8]`))
	}))
	defer srv.Close()
	got, err := New(Options{BaseURL: srv.URL}).PlatformIDs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if path != "/api/platforms/identifiers" || !reflect.DeepEqual(got, []int{3, 5, 8}) {
		t.Fatalf("path=%q got=%v", path, got)
	}
}
