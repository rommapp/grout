package main

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rommapp/grout/api"
	"github.com/rommapp/grout/core/romm"
	"github.com/rommapp/grout/test/fakeromm"
)

func goroutineID() uint64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	f := strings.Fields(strings.TrimPrefix(string(buf[:n]), "goroutine "))
	id, err := strconv.ParseUint(f[0], 10, 64)
	if err != nil {
		panic(err)
	}
	return id
}

func newTestTable() *table {
	return newTable(goroutineID)
}

func optionsJSON(t *testing.T, dataDir string) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"data_dir": dataDir,
		"firmware": map[string]string{"id": "testfw"},
		"env":      map[string]string{"GROUT_ROOT": t.TempDir()},
	})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func openHandle(t *testing.T, tb *table, dataDir string) uintptr {
	t.Helper()
	h, failure := tb.open(optionsJSON(t, dataDir))
	if h == 0 {
		t.Fatalf("open failed: %s", failure)
	}
	t.Cleanup(func() { tb.close(h) })
	return h
}

type envelope struct {
	OK     bool           `json:"ok"`
	Result any            `json:"result"`
	Error  *api.ErrorBody `json:"error"`
}

func decodeEnvelope(t *testing.T, raw []byte) envelope {
	t.Helper()
	var e envelope
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatalf("decoding %s: %v", raw, err)
	}
	return e
}

func wantCode(t *testing.T, raw []byte, code api.Code) {
	t.Helper()
	e := decodeEnvelope(t, raw)
	if e.OK || e.Error == nil || e.Error.Code != code {
		t.Fatalf("envelope %s, want code %s", raw, code)
	}
}

func wantOK(t *testing.T, raw []byte) {
	t.Helper()
	if e := decodeEnvelope(t, raw); !e.OK {
		t.Fatalf("envelope %s, want ok", raw)
	}
}

type recorded struct {
	callID  int64
	payload string
}

type recorder struct {
	mu     sync.Mutex
	events []recorded
}

func (r *recorder) sink(callID int64, payload []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, recorded{callID, string(payload)})
}

func (r *recorder) count(callID int64) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, e := range r.events {
		if e.callID == callID {
			n++
		}
	}
	return n
}

func (r *recorder) total() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.events)
}

type gate struct {
	entered chan struct{}
	release chan struct{}
	enter   sync.Once
	open    sync.Once
}

func block(t *testing.T, srv *fakeromm.Server) *gate {
	t.Helper()
	g := &gate{entered: make(chan struct{}), release: make(chan struct{})}
	srv.SetBeforePage(func(int, int) {
		g.enter.Do(func() { close(g.entered) })
		<-g.release
	})
	t.Cleanup(g.unblock)
	return g
}

func (g *gate) unblock() {
	g.open.Do(func() { close(g.release) })
}

func connectedHandle(t *testing.T, tb *table) (uintptr, *fakeromm.Server, string) {
	t.Helper()
	srv := fakeromm.New(t)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	srv.Platforms = []romm.Platform{{ID: 1, Slug: "gba", FSSlug: "gba", Name: "Game Boy Advance", RomCount: 2, UpdatedAt: base}}
	srv.Roms = fakeromm.Roms(1, 1, 2, base)
	dir := t.TempDir()
	h := openHandle(t, tb, dir)
	wantOK(t, tb.call(h, 1, "settings.set_host", []byte(fmt.Sprintf(`{"root_uri":%q,"token":"t"}`, srv.URL()))))
	wantOK(t, tb.call(h, 1, "settings.set_directory_mapping", []byte(`{"fs_slug":"gba","slug":"gba","enabled":true}`)))
	return h, srv, dir
}

func callAsync(tb *table, h uintptr, id int64, method string, args []byte) <-chan []byte {
	out := make(chan []byte, 1)
	go func() { out <- tb.call(h, id, method, args) }()
	return out
}

func within(t *testing.T, what string, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatalf("%s did not return", what)
	}
}

func receive(t *testing.T, what string, ch <-chan []byte) []byte {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(10 * time.Second):
		t.Fatalf("%s did not return", what)
		return nil
	}
}

func TestTableHandlesStartAtOneAndAreNotReused(t *testing.T) {
	tb := newTestTable()
	first := openHandle(t, tb, t.TempDir())
	second := openHandle(t, tb, t.TempDir())
	if first != 1 || second != 2 {
		t.Fatalf("handles = %d, %d, want 1, 2", first, second)
	}
	tb.close(first)
	third := openHandle(t, tb, t.TempDir())
	if third != 3 {
		t.Fatalf("handle after close = %d, want 3", third)
	}
}

func TestOpenInvalidOptionsIsBadRequest(t *testing.T) {
	tb := newTestTable()
	for _, raw := range [][]byte{nil, []byte("{"), []byte(`{"data_dir": 5}`)} {
		h, failure := tb.open(raw)
		if h != 0 {
			t.Fatalf("open(%q) = %d, want 0", raw, h)
		}
		wantCode(t, failure, api.CodeBadRequest)
	}
}

func TestOpenUnknownOptionIsBadRequest(t *testing.T) {
	tb := newTestTable()
	root := t.TempDir()
	for _, format := range []string{
		`{"data_dir":%q,"firmware":{"id":"testfw"},"enviroment":{"GROUT_ROOT":%q}}`,
		`{"data_dir":%q,"firmware":{"id":"testfw","bogus":1},"env":{"GROUT_ROOT":%q}}`,
	} {
		raw := []byte(fmt.Sprintf(format, t.TempDir(), root))
		h, failure := tb.open(raw)
		if h != 0 {
			t.Fatalf("open(%s) = %d, want 0", raw, h)
		}
		wantCode(t, failure, api.CodeBadRequest)
	}
}

func TestOpenSameDataDirIsLocked(t *testing.T) {
	tb := newTestTable()
	dir := t.TempDir()
	openHandle(t, tb, dir)
	h, failure := tb.open(optionsJSON(t, dir))
	if h != 0 {
		t.Fatalf("second open = %d, want 0", h)
	}
	wantCode(t, failure, api.CodeDataDirLocked)
}

func TestDuplicateInFlightCallIDIsBadRequest(t *testing.T) {
	tb := newTestTable()
	h, srv, _ := connectedHandle(t, tb)
	g := block(t, srv)
	done := callAsync(tb, h, 5, "library.refresh", nil)
	<-g.entered
	wantCode(t, tb.call(h, 5, "status", nil), api.CodeBadRequest)
	g.unblock()
	wantOK(t, receive(t, "refresh", done))
	wantOK(t, tb.call(h, 5, "status", nil))
}

func TestCancelStopsEventsForThatCall(t *testing.T) {
	tb := newTestTable()
	h, srv, _ := connectedHandle(t, tb)
	rec := &recorder{}
	tb.setSink(h, rec.sink)
	g := block(t, srv)
	done := callAsync(tb, h, 9, "library.refresh", nil)
	<-g.entered
	if rec.count(9) == 0 {
		t.Fatal("no events before the blocked page")
	}
	tb.cancel(h, 9)
	after := rec.count(9)
	g.unblock()
	wantCode(t, receive(t, "refresh", done), api.CodeCancelled)
	if got := rec.count(9); got != after {
		t.Fatalf("events after cancel: %d, want %d", got, after)
	}
}

func TestCloseCancelsWaitsAndSilencesEvents(t *testing.T) {
	tb := newTestTable()
	h, srv, dir := connectedHandle(t, tb)
	rec := &recorder{}
	tb.setSink(h, rec.sink)
	g := block(t, srv)
	done := callAsync(tb, h, 3, "library.refresh", nil)
	<-g.entered
	within(t, "close", func() { tb.close(h) })
	if n := inFlight(tb, h); n != 0 {
		t.Fatalf("calls in flight after close: %d", n)
	}
	after := rec.total()
	g.unblock()
	wantCode(t, receive(t, "refresh", done), api.CodeCancelled)
	if got := rec.total(); got != after {
		t.Fatalf("events after close: %d, want %d", got, after)
	}
	wantCode(t, tb.call(h, 4, "status", nil), api.CodeBadRequest)
	reopened, failure := tb.open(optionsJSON(t, dir))
	if reopened == 0 {
		t.Fatalf("reopen after close failed: %s", failure)
	}
	tb.close(reopened)
}

func inFlight(tb *table, h uintptr) int {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	if inst := tb.instances[h]; inst != nil {
		return len(inst.calls)
	}
	return 0
}

type slow struct {
	returned atomic.Bool
}

func (s *slow) Call(ctx context.Context, method string, args []byte, emit func(api.Event)) []byte {
	<-ctx.Done()
	time.Sleep(50 * time.Millisecond)
	s.returned.Store(true)
	raw, _ := json.Marshal(api.Envelope{Error: &api.ErrorBody{Code: api.CodeCancelled, Message: "cancelled"}})
	return raw
}

func (s *slow) Close() error {
	return nil
}

func TestCloseWaitsForInFlightCallThatIsNotDelivering(t *testing.T) {
	tb := newTestTable()
	b := &slow{}
	h := tb.add(b)
	done := callAsync(tb, h, 1, "x", nil)
	for inFlight(tb, h) == 0 {
		runtime.Gosched()
	}
	within(t, "close", func() { tb.close(h) })
	if !b.returned.Load() {
		t.Fatal("close returned before the in-flight call returned")
	}
	if n := inFlight(tb, h); n != 0 {
		t.Fatalf("calls in flight after close: %d", n)
	}
	wantCode(t, receive(t, "call", done), api.CodeCancelled)
}

func TestSetSinkFromAnotherThreadWaitsForDelivery(t *testing.T) {
	tb := newTestTable()
	h := tb.add(&scripted{events: 3})
	rec := &recorder{}
	hd := newHeld()
	tb.setSink(h, hd.sink(rec))
	done := callAsync(tb, h, 1, "x", nil)
	<-hd.entered
	replaced := make(chan struct{})
	go func() {
		tb.setSink(h, nil)
		close(replaced)
	}()
	select {
	case <-replaced:
		t.Fatal("setSink returned while a delivery was running")
	case <-time.After(100 * time.Millisecond):
	}
	close(hd.release)
	select {
	case <-replaced:
	case <-time.After(10 * time.Second):
		t.Fatal("setSink did not return")
	}
	wantOK(t, receive(t, "call", done))
	if got := rec.count(1); got != 1 {
		t.Fatalf("events = %d, want 1", got)
	}
	tb.close(h)
}

func TestSetSinkFromInsideDeliveryOnSameThread(t *testing.T) {
	tb := newTestTable()
	h := tb.add(&scripted{events: 3})
	rec := &recorder{}
	tb.setSink(h, func(id int64, p []byte) {
		rec.sink(id, p)
		tb.setSink(h, nil)
	})
	var out []byte
	within(t, "call", func() { out = tb.call(h, 1, "x", nil) })
	wantOK(t, out)
	if got := rec.count(1); got != 1 {
		t.Fatalf("events = %d, want 1", got)
	}
	tb.close(h)
}

func TestUnknownHandleIsBadRequest(t *testing.T) {
	tb := newTestTable()
	wantCode(t, tb.call(42, 1, "status", nil), api.CodeBadRequest)
	tb.cancel(42, 1)
	tb.setSink(42, nil)
	tb.close(42)
}

func TestEventJSONCarriesCallID(t *testing.T) {
	got := string(eventJSON(7, api.Event{Type: api.EventProgress, Stage: "games", Done: 1, Total: 2}))
	want := `{"call_id":7,"type":"progress","stage":"games","done":1,"total":2}`
	if got != want {
		t.Fatalf("event = %s, want %s", got, want)
	}
}

type scripted struct {
	events int
	closes atomic.Int32
}

func (s *scripted) Call(ctx context.Context, method string, args []byte, emit func(api.Event)) []byte {
	for i := 0; i < s.events; i++ {
		if ctx.Err() != nil {
			raw, _ := json.Marshal(api.Envelope{Error: &api.ErrorBody{Code: api.CodeCancelled, Message: "cancelled"}})
			return raw
		}
		emit(api.Event{Type: api.EventProgress, Done: i, Total: s.events})
	}
	return []byte(`{"ok":true}`)
}

func (s *scripted) Close() error {
	s.closes.Add(1)
	return nil
}

func TestCloseFromInsideDeliveryOnSameThread(t *testing.T) {
	tb := newTestTable()
	b := &scripted{events: 3}
	h := tb.add(b)
	rec := &recorder{}
	tb.setSink(h, func(id int64, p []byte) {
		rec.sink(id, p)
		tb.close(h)
	})
	var out []byte
	within(t, "call", func() { out = tb.call(h, 1, "x", nil) })
	wantCode(t, out, api.CodeCancelled)
	if got := rec.count(1); got != 1 {
		t.Fatalf("events = %d, want 1", got)
	}
	if b.closes.Load() == 0 {
		t.Fatal("backend not closed")
	}
	wantCode(t, tb.call(h, 2, "x", nil), api.CodeBadRequest)
}

func TestCancelFromInsideDeliveryOnSameThread(t *testing.T) {
	tb := newTestTable()
	h := tb.add(&scripted{events: 3})
	rec := &recorder{}
	tb.setSink(h, func(id int64, p []byte) {
		rec.sink(id, p)
		tb.cancel(h, id)
	})
	var out []byte
	within(t, "call", func() { out = tb.call(h, 1, "x", nil) })
	wantCode(t, out, api.CodeCancelled)
	if got := rec.count(1); got != 1 {
		t.Fatalf("events = %d, want 1", got)
	}
	tb.close(h)
}

type held struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func newHeld() *held {
	return &held{entered: make(chan struct{}), release: make(chan struct{})}
}

func (hd *held) sink(rec *recorder) sink {
	return func(id int64, p []byte) {
		rec.sink(id, p)
		first := false
		hd.once.Do(func() { first = true })
		if first {
			close(hd.entered)
			<-hd.release
		}
	}
}

func TestCloseFromAnotherThreadWaitsForDelivery(t *testing.T) {
	tb := newTestTable()
	b := &scripted{events: 3}
	h := tb.add(b)
	rec := &recorder{}
	hd := newHeld()
	tb.setSink(h, hd.sink(rec))
	done := callAsync(tb, h, 1, "x", nil)
	<-hd.entered
	closed := make(chan struct{})
	go func() {
		tb.close(h)
		close(closed)
	}()
	select {
	case <-closed:
		t.Fatal("close returned while a delivery was running")
	case <-time.After(100 * time.Millisecond):
	}
	close(hd.release)
	select {
	case <-closed:
	case <-time.After(10 * time.Second):
		t.Fatal("close did not return")
	}
	wantCode(t, receive(t, "call", done), api.CodeCancelled)
	if got := rec.count(1); got != 1 {
		t.Fatalf("events = %d, want 1", got)
	}
	if b.closes.Load() != 1 {
		t.Fatalf("backend closes = %d, want 1", b.closes.Load())
	}
}

func TestCancelFromAnotherThreadWaitsForDelivery(t *testing.T) {
	tb := newTestTable()
	h := tb.add(&scripted{events: 3})
	rec := &recorder{}
	hd := newHeld()
	tb.setSink(h, hd.sink(rec))
	done := callAsync(tb, h, 1, "x", nil)
	<-hd.entered
	cancelled := make(chan struct{})
	go func() {
		tb.cancel(h, 1)
		close(cancelled)
	}()
	select {
	case <-cancelled:
		t.Fatal("cancel returned while a delivery was running")
	case <-time.After(100 * time.Millisecond):
	}
	close(hd.release)
	select {
	case <-cancelled:
	case <-time.After(10 * time.Second):
		t.Fatal("cancel did not return")
	}
	wantCode(t, receive(t, "call", done), api.CodeCancelled)
	if got := rec.count(1); got != 1 {
		t.Fatalf("events = %d, want 1", got)
	}
	tb.close(h)
}

func TestCrossThreadReentryDoesNotDeadlock(t *testing.T) {
	for _, op := range []string{"close", "cancel"} {
		t.Run(op, func(t *testing.T) {
			tb := newTestTable()
			h := tb.add(&scripted{events: 3})
			var both sync.WaitGroup
			both.Add(2)
			var once [2]sync.Once
			tb.setSink(h, func(id int64, _ []byte) {
				first := false
				once[id-1].Do(func() { first = true })
				if !first {
					return
				}
				both.Done()
				both.Wait()
				if op == "close" {
					tb.close(h)
				} else {
					tb.cancel(h, 3-id)
				}
			})
			one := callAsync(tb, h, 1, "x", nil)
			two := callAsync(tb, h, 2, "x", nil)
			wantCode(t, receive(t, "call 1", one), api.CodeCancelled)
			wantCode(t, receive(t, "call 2", two), api.CodeCancelled)
			tb.close(h)
		})
	}
}

func TestNoSinkDropsEvents(t *testing.T) {
	tb := newTestTable()
	h := tb.add(&scripted{events: 2})
	wantOK(t, tb.call(h, 1, "x", nil))
	rec := &recorder{}
	tb.setSink(h, rec.sink)
	wantOK(t, tb.call(h, 2, "x", nil))
	tb.setSink(h, nil)
	wantOK(t, tb.call(h, 3, "x", nil))
	if rec.count(2) != 2 || rec.total() != 2 {
		t.Fatalf("events = %+v", rec.events)
	}
	tb.close(h)
}
