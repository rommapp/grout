package grout

import (
	"context"
	"sync"
	"testing"

	"github.com/rommapp/grout/core/config"
	"github.com/rommapp/grout/core/library"
	"github.com/rommapp/grout/test/fakeromm"
)

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

type refreshOutcome struct {
	res *RefreshResult
	err error
}

func refreshAsync(g *Grout, ctx context.Context) <-chan refreshOutcome {
	out := make(chan refreshOutcome, 1)
	go func() {
		res, err := g.Refresh(ctx, RefreshOptions{}, nil)
		out <- refreshOutcome{res, err}
	}()
	return out
}

func TestRefreshWithScan(t *testing.T) {
	srv := gbaServer(t)
	g := connected(t, srv)
	ctx := context.Background()
	res, err := g.Refresh(ctx, RefreshOptions{Scan: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Scan == nil {
		t.Fatal("scan result missing")
	}
	if res.Refresh.Games != 2 {
		t.Fatalf("refresh = %+v", res.Refresh)
	}
	res, err = g.Refresh(ctx, RefreshOptions{Scan: false}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Scan != nil {
		t.Fatalf("scan = %+v", res.Scan)
	}
}

func TestWritersAreExclusiveReadsAreNot(t *testing.T) {
	srv := gbaServer(t)
	g := connected(t, srv)
	ctx := context.Background()
	gt := block(t, srv)
	done := refreshAsync(g, ctx)
	<-gt.entered

	_, err := g.Refresh(ctx, RefreshOptions{}, nil)
	wantCode(t, err, CodeBusy)
	_, err = g.Scan(ctx, nil)
	wantCode(t, err, CodeBusy)
	if _, err := g.Games(ctx, library.Query{}); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Platforms(ctx, true); err != nil {
		t.Fatal(err)
	}

	gt.unblock()
	if out := <-done; out.err != nil {
		t.Fatal(out.err)
	}
}

func TestRefreshCancelled(t *testing.T) {
	srv := gbaServer(t)
	g := connected(t, srv)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv.SetBeforePage(func(int, int) { cancel() })
	_, err := g.Refresh(ctx, RefreshOptions{}, nil)
	wantCode(t, err, CodeCancelled)
}

func TestSetHostDuringRefreshKeepsSnapshot(t *testing.T) {
	first := gbaServer(t)
	second := gbaServer(t)
	g := connected(t, first)
	ctx := context.Background()
	gt := block(t, first)
	done := refreshAsync(g, ctx)
	<-gt.entered

	if err := g.SetHost(config.Host{RootURI: second.URL(), Token: "t"}); err != nil {
		t.Fatal(err)
	}
	gt.unblock()
	out := <-done
	if out.err != nil {
		t.Fatal(out.err)
	}
	if out.res.Refresh.Games != 2 {
		t.Fatalf("refresh = %+v", out.res.Refresh)
	}
	if calls := second.Calls(); len(calls) != 0 {
		t.Fatalf("second server saw %+v", calls)
	}
	if got := g.Settings().Hosts[0].RootURI; got != second.URL() {
		t.Fatalf("host = %q", got)
	}
	games, err := g.Games(ctx, library.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(games) != 2 {
		t.Fatalf("games = %+v", games)
	}
}
