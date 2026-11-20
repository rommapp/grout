package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"grout/core/romm"
)

const (
	pageSize      = 250
	cursorOverlap = 5 * time.Second
)

type RefreshResult struct {
	Platforms        int      `json:"platforms"`
	Games            int      `json:"games"`
	Collections      int      `json:"collections"`
	Deleted          int      `json:"deleted"`
	FailedPlatforms  []string `json:"failed_platforms"`
	CollectionErrors []string `json:"collection_errors"`
	PurgeError       string   `json:"purge_error"`
}

type RefreshOptions struct {
	Full bool
}

type refreshRun struct {
	l        *Library
	ctx      context.Context
	opts     RefreshOptions
	progress func(Progress)
	res      RefreshResult
}

func (l *Library) Refresh(ctx context.Context, opts RefreshOptions, progress func(Progress)) (RefreshResult, error) {
	if progress == nil {
		progress = func(Progress) {}
	}
	r := &refreshRun{l: l, ctx: ctx, opts: opts, progress: progress, res: RefreshResult{FailedPlatforms: []string{}, CollectionErrors: []string{}}}
	platforms, err := r.platforms()
	if err != nil {
		return r.res, err
	}
	if err := r.games(platforms); err != nil {
		return r.res, err
	}
	if err := r.purgeGames(); err != nil {
		return r.res, err
	}
	if err := r.collections(); err != nil {
		return r.res, err
	}
	return r.res, nil
}

func (r *refreshRun) inTx(fn func(tx *sql.Tx) error) error {
	tx, err := r.l.Store.DB.Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (r *refreshRun) platforms() ([]romm.Platform, error) {
	r.progress(Progress{Stage: StagePlatforms})
	ps, err := r.l.Client.Platforms(r.ctx)
	if err != nil {
		if ctxErr := r.ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, fmt.Errorf("fetching platforms: %w", err)
	}
	keep := make([]string, len(ps))
	for i, p := range ps {
		keep[i] = strconv.Itoa(p.ID)
	}
	var deleted int
	err = r.inTx(func(tx *sql.Tx) error {
		if err := savePlatforms(tx, ps); err != nil {
			return err
		}
		n, err := purgeExcept(tx, "platforms", "id", keep)
		deleted = n
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("saving platforms: %w", err)
	}
	r.res.Platforms = len(ps)
	r.res.Deleted += deleted
	r.progress(Progress{Stage: StagePlatforms, Done: len(ps), Total: len(ps)})
	return ps, nil
}

func (r *refreshRun) enabled(ps []romm.Platform) []romm.Platform {
	var out []romm.Platform
	for _, p := range ps {
		if _, ok := r.l.Config.DirectoryMappings[p.FSSlug]; ok {
			out = append(out, p)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func parseCursor(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func sendCursor(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Add(-cursorOverlap).UTC().Format(time.RFC3339Nano)
}

func later(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

func (r *refreshRun) platformCursor(id int) (time.Time, error) {
	if r.opts.Full {
		return time.Time{}, nil
	}
	var v string
	err := r.l.Store.DB.QueryRow(`SELECT cursor FROM platform_sync WHERE platform_id = ?`, id).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	return parseCursor(v), nil
}

func (r *refreshRun) games(ps []romm.Platform) error {
	enabled := r.enabled(ps)
	total := 0
	for _, p := range enabled {
		total += p.RomCount
	}
	done := 0
	report := func(n int) {
		done += n
		r.progress(Progress{Stage: StageGames, Done: min(done, total), Total: total})
	}
	r.progress(Progress{Stage: StageGames, Total: total})
	for _, p := range enabled {
		if err := r.ctx.Err(); err != nil {
			return err
		}
		if err := r.platformGames(p, report); err != nil {
			if ctxErr := r.ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			r.res.FailedPlatforms = append(r.res.FailedPlatforms, p.FSSlug)
			if err := r.markPlatform(p.ID, "", "failed"); err != nil {
				return fmt.Errorf("marking platform %s failed: %w", p.FSSlug, err)
			}
		}
	}
	r.progress(Progress{Stage: StageGames, Done: total, Total: total})
	return nil
}

func (r *refreshRun) platformGames(p romm.Platform, report func(int)) error {
	stored, err := r.platformCursor(p.ID)
	if err != nil {
		return err
	}
	after := sendCursor(stored)
	saved := map[int]bool{}
	var newest, passStart time.Time
	firstPage := true
	expected := p.RomCount
	pass := func(withTotal bool) (int, error) {
		n := 0
		for offset := 0; ; offset += pageSize {
			q := romm.RomsQuery{PlatformID: p.ID, UpdatedAfter: after, Limit: pageSize, Offset: offset, WithTotal: withTotal && offset == 0}
			page, err := r.l.Client.Roms(r.ctx, q)
			if err != nil {
				return n, err
			}
			if firstPage {
				passStart, firstPage = page.ServerTime, false
			}
			if q.WithTotal && (page.Total > 0 || len(page.Items) == 0) {
				expected = page.Total
			}
			if len(page.Items) == 0 {
				return n, nil
			}
			if err := r.inTx(func(tx *sql.Tx) error { return saveGames(tx, page.Items) }); err != nil {
				return n, err
			}
			fresh := 0
			for _, g := range page.Items {
				if !saved[g.ID] {
					saved[g.ID] = true
					fresh++
				}
				newest = later(newest, g.UpdatedAt)
			}
			n += len(page.Items)
			report(fresh)
			if len(page.Items) < pageSize {
				return n, nil
			}
		}
	}
	n, err := pass(after == "")
	if err != nil {
		return err
	}
	if after == "" && n < expected {
		if _, err := pass(false); err != nil {
			return err
		}
	}
	r.res.Games += len(saved)
	cursor := ""
	if !newest.IsZero() {
		if !passStart.IsZero() && newest.After(passStart) {
			newest = passStart
		}
		cursor = later(stored, newest).UTC().Format(time.RFC3339Nano)
	}
	return r.markPlatform(p.ID, cursor, "ok")
}

func (r *refreshRun) markPlatform(id int, cursor, status string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	return r.inTx(func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO platform_sync (platform_id, cursor, status, attempted_at) VALUES (?, ?, ?, ?)
			ON CONFLICT(platform_id) DO UPDATE SET
				cursor = CASE WHEN excluded.cursor = '' THEN platform_sync.cursor ELSE excluded.cursor END,
				status = excluded.status, attempted_at = excluded.attempted_at`,
			id, cursor, status, now)
		return err
	})
}

func (r *refreshRun) purgeGames() error {
	r.progress(Progress{Stage: StagePurge})
	ids, err := r.l.Client.RomIDs(r.ctx)
	if err != nil {
		if ctxErr := r.ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		r.res.PurgeError = err.Error()
		r.progress(Progress{Stage: StagePurge, Done: 1, Total: 1})
		return nil
	}
	keep := make([]string, len(ids))
	for i, id := range ids {
		keep[i] = strconv.Itoa(id)
	}
	var deleted int
	err = r.inTx(func(tx *sql.Tx) error {
		n, err := purgeExcept(tx, "games", "id", keep)
		deleted = n
		return err
	})
	if err != nil {
		return fmt.Errorf("purging games: %w", err)
	}
	r.res.Deleted += deleted
	r.progress(Progress{Stage: StagePurge, Done: 1, Total: 1})
	return nil
}

func (r *refreshRun) kindEnabled(kind romm.CollectionKind) bool {
	switch kind {
	case romm.CollectionRegular:
		return r.l.Config.ShowRegularCollections
	case romm.CollectionSmart:
		return r.l.Config.ShowSmartCollections
	case romm.CollectionVirtual:
		return r.l.Config.ShowVirtualCollections
	}
	return false
}

func collectionStream(kind romm.CollectionKind) string {
	return "collections:" + string(kind)
}

func (r *refreshRun) streamCursor(stream string) (time.Time, error) {
	var v string
	err := r.l.Store.DB.QueryRow(`SELECT value FROM cursors WHERE stream = ?`, stream).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	return parseCursor(v), nil
}

func (r *refreshRun) collections() error {
	kinds := []romm.CollectionKind{romm.CollectionRegular, romm.CollectionSmart, romm.CollectionVirtual}
	r.progress(Progress{Stage: StageCollections, Total: len(kinds)})
	for i, kind := range kinds {
		if err := r.ctx.Err(); err != nil {
			return err
		}
		var err error
		if r.kindEnabled(kind) {
			err = r.collectionKind(kind)
		} else {
			err = r.dropKind(kind)
		}
		if err != nil {
			if ctxErr := r.ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			r.res.CollectionErrors = append(r.res.CollectionErrors, fmt.Sprintf("%s: %v", kind, err))
		}
		r.progress(Progress{Stage: StageCollections, Done: i + 1, Total: len(kinds)})
	}
	return nil
}

func (r *refreshRun) collectionKind(kind romm.CollectionKind) error {
	stream := collectionStream(kind)
	var stored time.Time
	if kind != romm.CollectionVirtual && !r.opts.Full {
		var err error
		if stored, err = r.streamCursor(stream); err != nil {
			return err
		}
	}
	cols, err := r.l.Client.Collections(r.ctx, kind, sendCursor(stored))
	if err != nil {
		return err
	}
	ids, err := r.l.Client.CollectionIDs(r.ctx, kind)
	if err != nil {
		return err
	}
	var newest time.Time
	for _, c := range cols {
		newest = later(newest, c.UpdatedAt)
	}
	var deleted int
	err = r.inTx(func(tx *sql.Tx) error {
		for _, c := range cols {
			if err := saveCollection(tx, c); err != nil {
				return err
			}
		}
		n, err := purgeCollectionsExcept(tx, kind, ids)
		if err != nil {
			return err
		}
		deleted = n
		if kind == romm.CollectionVirtual || newest.IsZero() {
			return nil
		}
		_, err = tx.Exec(`INSERT INTO cursors (stream, value) VALUES (?, ?)
			ON CONFLICT(stream) DO UPDATE SET value = excluded.value`,
			stream, later(stored, newest).UTC().Format(time.RFC3339Nano))
		return err
	})
	if err != nil {
		return err
	}
	r.res.Collections += len(cols)
	r.res.Deleted += deleted
	return nil
}

func (r *refreshRun) dropKind(kind romm.CollectionKind) error {
	var deleted int
	err := r.inTx(func(tx *sql.Tx) error {
		res, err := tx.Exec(`DELETE FROM collections WHERE kind = ?`, string(kind))
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		deleted = int(n)
		_, err = tx.Exec(`DELETE FROM cursors WHERE stream = ?`, collectionStream(kind))
		return err
	})
	if err != nil {
		return err
	}
	r.res.Deleted += deleted
	return nil
}
