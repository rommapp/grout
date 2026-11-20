package grout

import (
	"context"
	"io/fs"
	"sync"
	"testing"
	"time"

	"github.com/rommapp/grout/core/config"
	"github.com/rommapp/grout/core/firmware/host"
	"github.com/rommapp/grout/core/library"
	"github.com/rommapp/grout/test/fakeromm"
)

const prompt = 5 * time.Second

type blockingHost struct {
	host.Host
	mu      sync.Mutex
	armed   bool
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (h *blockingHost) arm() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.armed = true
}

func (h *blockingHost) ReadDir(p string) ([]fs.DirEntry, error) {
	h.mu.Lock()
	armed := h.armed
	h.mu.Unlock()
	if armed {
		h.once.Do(func() { close(h.entered) })
		<-h.release
	}
	return h.Host.ReadDir(p)
}

func closeAsync(g *Grout) <-chan error {
	out := make(chan error, 1)
	go func() { out <- g.Close() }()
	return out
}

func within[T any](t *testing.T, ch <-chan T, what string) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(prompt):
		t.Fatalf("%s did not return promptly", what)
		var zero T
		return zero
	}
}

func TestCloseCancelsInFlightRefresh(t *testing.T) {
	srv := gbaServer(t)
	g := connected(t, srv)
	gt := block(t, srv)
	done := refreshAsync(g, context.Background())
	<-gt.entered

	closed := closeAsync(g)
	if err := within(t, closed, "Close"); err != nil {
		t.Fatal(err)
	}
	out := within(t, done, "Refresh")
	wantCode(t, out.err, CodeCancelled)
}

func TestCallsDuringCloseFailFast(t *testing.T) {
	srv := gbaServer(t)
	bh := &blockingHost{entered: make(chan struct{}), release: make(chan struct{})}
	o := testOptions(t, t.TempDir())
	bh.Host = host.OS(func(k string) string { return o.Env[k] })
	o.Host = bh
	g := open(t, o)
	t.Cleanup(func() {
		select {
		case <-bh.release:
		default:
			close(bh.release)
		}
	})
	if err := g.SetHost(config.Host{RootURI: srv.URL(), Token: "t"}); err != nil {
		t.Fatal(err)
	}
	if err := g.SetDirectoryMapping("gba", &config.DirectoryMapping{RomMSlug: "gba"}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := g.Refresh(ctx, RefreshOptions{}, nil); err != nil {
		t.Fatal(err)
	}

	bh.arm()
	scanned := make(chan error, 1)
	go func() {
		_, err := g.Scan(ctx, nil)
		scanned <- err
	}()
	<-bh.entered
	closed := closeAsync(g)
	deadline := time.Now().Add(prompt)
	for !g.closing.Load() {
		if time.Now().After(deadline) {
			t.Fatal("Close never started")
		}
		time.Sleep(time.Millisecond)
	}

	games := make(chan error, 1)
	go func() {
		_, err := g.Games(ctx, library.Query{})
		games <- err
	}()
	wantCode(t, within(t, games, "Games"), CodeBadRequest)
	select {
	case err := <-closed:
		t.Fatalf("Close returned %v while a scan was still running", err)
	default:
	}

	close(bh.release)
	within(t, scanned, "Scan")
	if err := within(t, closed, "Close"); err != nil {
		t.Fatal(err)
	}
}

func TestProgressCallbackMayCallGrout(t *testing.T) {
	t.Run("read", func(t *testing.T) {
		srv := gbaServer(t)
		g := connected(t, srv)
		ctx := context.Background()
		var readErr error
		var reads int
		progress := func(library.Progress) {
			_, err := g.Games(ctx, library.Query{})
			reads++
			if err != nil && readErr == nil {
				readErr = err
			}
		}
		done := make(chan error, 1)
		go func() {
			_, err := g.Refresh(ctx, RefreshOptions{}, progress)
			done <- err
		}()
		if err := within(t, done, "Refresh"); err != nil {
			t.Fatal(err)
		}
		if reads == 0 || readErr != nil {
			t.Fatalf("reads = %d, err = %v", reads, readErr)
		}
	})

	t.Run("close", func(t *testing.T) {
		srv := gbaServer(t)
		g := connected(t, srv)
		ctx := context.Background()
		var once sync.Once
		progress := func(library.Progress) {
			once.Do(func() { g.Close() })
		}
		done := make(chan error, 1)
		go func() {
			_, err := g.Refresh(ctx, RefreshOptions{}, progress)
			done <- err
		}()
		wantCode(t, within(t, done, "Refresh"), CodeCancelled)
		_, err := g.Games(ctx, library.Query{})
		wantCode(t, err, CodeBadRequest)
	})
}

func connectedWith(t *testing.T, srv *fakeromm.Server, o Options) *Grout {
	t.Helper()
	g := open(t, o)
	if err := g.SetHost(config.Host{RootURI: srv.URL(), Token: "t"}); err != nil {
		t.Fatal(err)
	}
	if err := g.SetDirectoryMapping("gba", &config.DirectoryMapping{RomMSlug: "gba"}); err != nil {
		t.Fatal(err)
	}
	return g
}

func TestCloseDuringCallbackFromOtherGoroutineDefersRelease(t *testing.T) {
	srv := gbaServer(t)
	o := testOptions(t, t.TempDir())
	g := connectedWith(t, srv, o)
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	progress := func(library.Progress) {
		once.Do(func() {
			close(entered)
			<-release
		})
	}
	done := make(chan error, 1)
	go func() {
		_, err := g.Refresh(context.Background(), RefreshOptions{}, progress)
		done <- err
	}()
	<-entered

	if err := within(t, closeAsync(g), "Close"); err != nil {
		t.Fatal(err)
	}
	_, err := Open(context.Background(), o)
	wantCode(t, err, CodeDataDirLocked)

	close(release)
	wantCode(t, within(t, done, "Refresh"), CodeCancelled)
	again, err := Open(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	again.Close()
}

func TestConcurrentCloseWaitsForRelease(t *testing.T) {
	srv := gbaServer(t)
	bh := &blockingHost{entered: make(chan struct{}), release: make(chan struct{})}
	o := testOptions(t, t.TempDir())
	bh.Host = host.OS(func(k string) string { return o.Env[k] })
	o.Host = bh
	g := connectedWith(t, srv, o)
	var unblock sync.Once
	t.Cleanup(func() { unblock.Do(func() { close(bh.release) }) })
	ctx := context.Background()
	if _, err := g.Refresh(ctx, RefreshOptions{}, nil); err != nil {
		t.Fatal(err)
	}

	bh.arm()
	done := refreshScanAsync(g, ctx)
	<-bh.entered
	first := closeAsync(g)
	second := closeAsync(g)
	deadline := time.Now().Add(prompt)
	for !g.closing.Load() {
		if time.Now().After(deadline) {
			t.Fatal("Close never started")
		}
		time.Sleep(time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	select {
	case err := <-first:
		t.Fatalf("a Close returned %v before the refresh ended", err)
	case err := <-second:
		t.Fatalf("a Close returned %v before the refresh ended", err)
	default:
	}

	unblock.Do(func() { close(bh.release) })
	wantCode(t, within(t, done, "Refresh").err, CodeCancelled)
	a := within(t, first, "first Close")
	b := within(t, second, "second Close")
	if a != nil || b != nil {
		t.Fatalf("close errors = %v, %v", a, b)
	}
	again, err := Open(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	again.Close()
}

func refreshScanAsync(g *Grout, ctx context.Context) <-chan refreshOutcome {
	out := make(chan refreshOutcome, 1)
	go func() {
		res, err := g.Refresh(ctx, RefreshOptions{Scan: true}, nil)
		out <- refreshOutcome{res, err}
	}()
	return out
}
