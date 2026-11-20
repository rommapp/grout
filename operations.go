package grout

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"

	"github.com/rommapp/grout/core/library"
)

type RefreshOptions struct {
	Full bool
	Scan bool
}

type RefreshResult struct {
	Refresh library.RefreshResult
	Scan    *library.ScanResult
}

var errBusy = errors.New("a refresh or scan is already running")

func (g *Grout) beginWrite(ctx context.Context) (context.Context, func(), error) {
	ctx, end, err := g.begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	if !g.writer.TryLock() {
		end()
		return nil, nil, newError(CodeBusy, errBusy)
	}
	return ctx, func() {
		g.writer.Unlock()
		end()
	}, nil
}

func (g *Grout) Refresh(ctx context.Context, o RefreshOptions, progress func(library.Progress)) (*RefreshResult, error) {
	ctx, end, err := g.beginWrite(ctx)
	if err != nil {
		return nil, err
	}
	defer end()
	cfg, src := g.settings()
	_, client, err := connect(ctx, cfg, src)
	if err != nil {
		return nil, err
	}
	res := &RefreshResult{}
	l := g.library(cfg, client)
	report := g.progress(progress)
	res.Refresh, err = l.Refresh(ctx, library.RefreshOptions{Full: o.Full}, report)
	if err := finish(ctx, err); err != nil {
		return res, err
	}
	if o.Scan {
		s, err := l.Scan(ctx, report)
		res.Scan = &s
		if err := finish(ctx, err); err != nil {
			return res, err
		}
	}
	return res, nil
}

func (g *Grout) Scan(ctx context.Context, progress func(library.Progress)) (library.ScanResult, error) {
	ctx, end, err := g.beginWrite(ctx)
	if err != nil {
		return library.ScanResult{}, err
	}
	defer end()
	res, err := g.snapshot().Scan(ctx, g.progress(progress))
	return res, finish(ctx, err)
}

func (g *Grout) Platforms(ctx context.Context, all bool) ([]library.Platform, error) {
	ctx, end, err := g.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer end()
	l := g.snapshot()
	var ps []library.Platform
	if all {
		ps, err = l.AllPlatforms(ctx)
	} else {
		ps, err = l.Platforms(ctx)
	}
	return list(ctx, ps, err)
}

func (g *Grout) Games(ctx context.Context, q library.Query) ([]library.Game, error) {
	ctx, end, err := g.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer end()
	if err := checkQuery(q); err != nil {
		return nil, err
	}
	gs, err := g.snapshot().Games(ctx, q)
	return list(ctx, gs, err)
}

func (g *Grout) FilterValues(ctx context.Context, q library.Query, d library.Dim) ([]string, error) {
	ctx, end, err := g.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer end()
	if err := checkQuery(q); err != nil {
		return nil, err
	}
	if err := checkDim(d); err != nil {
		return nil, err
	}
	vs, err := g.snapshot().FilterValues(ctx, q, d)
	return list(ctx, vs, err)
}

func (g *Grout) Collections(ctx context.Context) ([]library.CollectionRow, error) {
	ctx, end, err := g.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer end()
	rows, err := g.snapshot().Collections(ctx)
	return list(ctx, rows, err)
}

func (g *Grout) CollectionPlatforms(ctx context.Context, k library.CollectionKey) ([]library.Platform, error) {
	ctx, end, err := g.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer end()
	ps, err := g.snapshot().CollectionPlatforms(ctx, k)
	return list(ctx, ps, err)
}

func (g *Grout) GameState(ctx context.Context, gameID int) (library.DownloadState, error) {
	ctx, end, err := g.begin(ctx)
	if err != nil {
		return library.NotDownloaded, err
	}
	defer end()
	s, err := g.snapshot().State(ctx, gameID)
	if errors.Is(err, sql.ErrNoRows) {
		return s, newError(CodeBadRequest, fmt.Errorf("unknown game %d", gameID))
	}
	return s, classify(ctx, err, CodeStorage)
}

func finish(ctx context.Context, err error) error {
	if err == nil && ctx.Err() != nil {
		return newError(CodeCancelled, ctx.Err())
	}
	return classify(ctx, err, CodeInternal)
}

func checkDim(d library.Dim) error {
	if !slices.Contains(library.Dims(), d) {
		return newError(CodeBadRequest, fmt.Errorf("unknown filter dimension %q", d))
	}
	return nil
}

func checkQuery(q library.Query) error {
	for d := range q.Filters {
		if err := checkDim(d); err != nil {
			return err
		}
	}
	return nil
}

func list[T any](ctx context.Context, s []T, err error) ([]T, error) {
	if err != nil {
		return nil, classify(ctx, err, CodeStorage)
	}
	if s == nil {
		return []T{}, nil
	}
	return s, nil
}
