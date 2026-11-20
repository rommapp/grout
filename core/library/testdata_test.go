package library

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"grout/core/romm"
)

type fakeCall struct {
	Path  string
	Query string
}

type fakeCollection struct {
	ID        string
	Name      string
	RomIDs    []int
	UpdatedAt time.Time
}

type fakeServer struct {
	*httptest.Server
	Client *romm.Client

	mu          sync.Mutex
	Platforms   []romm.Platform
	Roms        []romm.Rom
	Collections map[romm.CollectionKind][]fakeCollection
	Calls       []fakeCall
	Hidden      map[int]bool
	OmitTotal   bool
	pageCounts  map[int]int

	failAfter  map[int]int
	failPaths  map[string]bool
	beforePage func(platformID, offset int)
	clock      time.Time
}

func newFakeServer(t *testing.T) *fakeServer {
	t.Helper()
	f := &fakeServer{
		Collections: map[romm.CollectionKind][]fakeCollection{},
		Hidden:      map[int]bool{},
		pageCounts:  map[int]int{},
		failAfter:   map[int]int{},
		failPaths:   map[string]bool{},
	}
	f.Server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.Server.Close)
	f.Client = romm.New(romm.Options{BaseURL: f.Server.URL})
	return f
}

func (f *fakeServer) mutate(fn func()) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn()
}

func (f *fakeServer) setFailAfter(platformID, n int) {
	f.mutate(func() { f.failAfter[platformID] = n })
}

func (f *fakeServer) failPath(path string) {
	f.mutate(func() { f.failPaths[path] = true })
}

func (f *fakeServer) setBeforePage(fn func(platformID, offset int)) {
	f.mutate(func() { f.beforePage = fn })
}

func (f *fakeServer) setClock(t time.Time) {
	f.mutate(func() { f.clock = t })
}

func (f *fakeServer) calls() []fakeCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]fakeCall(nil), f.Calls...)
}

func (f *fakeServer) callsTo(path string) []fakeCall {
	var out []fakeCall
	for _, c := range f.calls() {
		if c.Path == path {
			out = append(out, c)
		}
	}
	return out
}

func (f *fakeServer) romPages(platformID int) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.pageCounts[platformID]
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func parseAfter(q string) (time.Time, bool) {
	if q == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339Nano, q)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func (f *fakeServer) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.Calls = append(f.Calls, fakeCall{Path: r.URL.Path, Query: r.URL.RawQuery})
	forced := f.failPaths[r.URL.Path]
	clock := f.clock
	f.mu.Unlock()
	if !clock.IsZero() {
		w.Header().Set("Date", clock.UTC().Format(http.TimeFormat))
	}
	if forced {
		http.Error(w, "forced failure", http.StatusInternalServerError)
		return
	}
	q := r.URL.Query()
	switch r.URL.Path {
	case "/api/platforms":
		f.mu.Lock()
		out := append([]romm.Platform{}, f.Platforms...)
		f.mu.Unlock()
		writeJSON(w, out)
	case "/api/platforms/identifiers":
		f.mu.Lock()
		out := []int{}
		for _, p := range f.Platforms {
			out = append(out, p.ID)
		}
		f.mu.Unlock()
		writeJSON(w, out)
	case "/api/roms":
		f.serveRoms(w, q.Get("platform_ids"), q.Get("updated_after"), q.Get("limit"), q.Get("offset"), q.Get("with_total") == "true")
	case "/api/roms/identifiers":
		f.mu.Lock()
		out := []int{}
		for _, rom := range f.Roms {
			if !f.Hidden[rom.ID] {
				out = append(out, rom.ID)
			}
		}
		f.mu.Unlock()
		sort.Ints(out)
		writeJSON(w, out)
	case "/api/collections":
		f.serveCollections(w, romm.CollectionRegular, q.Get("updated_after"))
	case "/api/collections/smart":
		f.serveCollections(w, romm.CollectionSmart, q.Get("updated_after"))
	case "/api/collections/virtual":
		f.serveCollections(w, romm.CollectionVirtual, "")
	case "/api/collections/identifiers":
		f.serveCollectionIDs(w, romm.CollectionRegular)
	case "/api/collections/smart/identifiers":
		f.serveCollectionIDs(w, romm.CollectionSmart)
	case "/api/collections/virtual/identifiers":
		f.serveCollectionIDs(w, romm.CollectionVirtual)
	default:
		http.NotFound(w, r)
	}
}

func (f *fakeServer) serveRoms(w http.ResponseWriter, platformIDs, updatedAfter, limitRaw, offsetRaw string, withTotal bool) {
	limit, _ := strconv.Atoi(limitRaw)
	offset, _ := strconv.Atoi(offsetRaw)
	want := map[int]bool{}
	first := 0
	for _, part := range strings.Split(platformIDs, ",") {
		if id, err := strconv.Atoi(part); err == nil {
			want[id] = true
			if first == 0 {
				first = id
			}
		}
	}

	f.mu.Lock()
	hook := f.beforePage
	f.mu.Unlock()
	if hook != nil {
		hook(first, offset)
	}

	f.mu.Lock()
	f.pageCounts[first]++
	failAt, failing := f.failAfter[first]
	if failing && f.pageCounts[first] == failAt {
		f.mu.Unlock()
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	after, hasAfter := parseAfter(updatedAfter)
	var matched []romm.Rom
	for _, rom := range f.Roms {
		if len(want) > 0 && !want[rom.PlatformID] {
			continue
		}
		if hasAfter && !rom.UpdatedAt.After(after) {
			continue
		}
		if f.Hidden[rom.ID] {
			continue
		}
		matched = append(matched, rom)
	}
	omitTotal := f.OmitTotal
	f.mu.Unlock()

	sort.Slice(matched, func(i, j int) bool { return matched[i].ID < matched[j].ID })
	items := []romm.Rom{}
	if offset < len(matched) {
		end := offset + limit
		if limit <= 0 || end > len(matched) {
			end = len(matched)
		}
		items = matched[offset:end]
	}
	body := map[string]any{"items": items, "limit": limit, "offset": offset}
	if withTotal && !omitTotal {
		body["total"] = len(matched)
	}
	writeJSON(w, body)
}

func (f *fakeServer) collectionID(kind romm.CollectionKind, id string) any {
	if kind == romm.CollectionVirtual {
		return id
	}
	n, _ := strconv.Atoi(id)
	return n
}

func (f *fakeServer) serveCollections(w http.ResponseWriter, kind romm.CollectionKind, updatedAfter string) {
	after, hasAfter := parseAfter(updatedAfter)
	f.mu.Lock()
	out := []map[string]any{}
	for _, c := range f.Collections[kind] {
		if hasAfter && !c.UpdatedAt.After(after) {
			continue
		}
		ids := c.RomIDs
		if ids == nil {
			ids = []int{}
		}
		out = append(out, map[string]any{
			"id":         f.collectionID(kind, c.ID),
			"name":       c.Name,
			"rom_ids":    ids,
			"updated_at": c.UpdatedAt,
		})
	}
	f.mu.Unlock()
	writeJSON(w, out)
}

func (f *fakeServer) serveCollectionIDs(w http.ResponseWriter, kind romm.CollectionKind) {
	f.mu.Lock()
	out := []any{}
	for _, c := range f.Collections[kind] {
		out = append(out, f.collectionID(kind, c.ID))
	}
	f.mu.Unlock()
	writeJSON(w, out)
}
