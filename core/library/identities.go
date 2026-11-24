package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/rommapp/grout/core/firmware/host"
	"github.com/rommapp/grout/core/ident"
)

type Identity struct {
	Path string
	ident.Result
}

func (l *Library) Identities(ctx context.Context, gameID int) ([]Identity, error) {
	if l.Ident == nil {
		return nil, nil
	}
	var slug string
	err := l.Store.DB.QueryRowContext(ctx, `SELECT p.slug FROM games g LEFT JOIN platforms p ON p.id = g.platform_id WHERE g.id = ?`, gameID).Scan(&slug)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("game %d: %w", gameID, ErrUnknownGame)
	}
	if err != nil {
		return nil, err
	}
	sigilSlug, ok := ident.Platform(slug)
	if !ok {
		return []Identity{}, nil
	}
	paths, err := l.identityPaths(ctx, gameID)
	if err != nil {
		return nil, err
	}
	version := l.Ident.Version()
	out := []Identity{}
	for _, p := range paths {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		info, err := l.Firmware.Host.Stat(p)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			out = append(out, Identity{Path: p, Result: ident.Result{Status: ident.StatusIO}})
			continue
		}
		size, mod := info.Size(), info.ModTime().Unix()
		cached, hit, err := l.cachedIdentity(ctx, p, size, mod, version)
		if err != nil {
			return out, err
		}
		if hit {
			out = append(out, Identity{Path: p, Result: cached})
			continue
		}
		real := p
		if w, ok := l.Firmware.Host.(host.Writable); ok {
			if real, err = w.Real(p); err != nil {
				out = append(out, Identity{Path: p, Result: ident.Result{Status: ident.StatusIO}})
				continue
			}
		}
		res := l.Ident.Extract(real, sigilSlug)
		if err := l.storeIdentity(ctx, p, size, mod, version, res); err != nil {
			return out, err
		}
		out = append(out, Identity{Path: p, Result: res})
	}
	return out, nil
}

func (l *Library) identityPaths(ctx context.Context, gameID int) ([]string, error) {
	rows, err := l.Store.DB.QueryContext(ctx, `SELECT path FROM local_files WHERE game_id = ? ORDER BY path`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		if strings.HasSuffix(strings.ToLower(p), ".m3u") {
			continue
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}

func (l *Library) cachedIdentity(ctx context.Context, p string, size, mod int64, version string) (ident.Result, bool, error) {
	var r ident.Result
	var status, source, usage, stored string
	var experimental int
	err := l.Store.DB.QueryRowContext(ctx, `SELECT sigil_version, status, title_id, save_id, raw_serial, platform, source, usage,
		features, experimental, n64_header, n64_md5, n64_md5_n64
		FROM identities WHERE path = ? AND size_bytes = ? AND mod_time = ?`, p, size, mod).
		Scan(&stored, &status, &r.TitleID, &r.SaveID, &r.RawSerial, &r.Platform, &source, &usage,
			&r.Features, &experimental, &r.N64Header, &r.N64MD5, &r.N64MD5N64)
	if errors.Is(err, sql.ErrNoRows) {
		return ident.Result{}, false, nil
	}
	if err != nil {
		return ident.Result{}, false, err
	}
	r.Status = ident.Status(status)
	r.Source = ident.Source(source)
	r.Usage = ident.Usage(usage)
	r.Experimental = experimental != 0
	if stored != version && (r.Status != ident.StatusOK || r.Experimental) {
		return ident.Result{}, false, nil
	}
	return r, true, nil
}

func (l *Library) storeIdentity(ctx context.Context, p string, size, mod int64, version string, r ident.Result) error {
	experimental := 0
	if r.Experimental {
		experimental = 1
	}
	_, err := l.Store.DB.ExecContext(context.WithoutCancel(ctx), `INSERT INTO identities
		(path, size_bytes, mod_time, sigil_version, status, title_id, save_id, raw_serial, platform, source, usage,
		features, experimental, n64_header, n64_md5, n64_md5_n64, identified_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
		size_bytes = excluded.size_bytes, mod_time = excluded.mod_time, sigil_version = excluded.sigil_version,
		status = excluded.status, title_id = excluded.title_id, save_id = excluded.save_id,
		raw_serial = excluded.raw_serial, platform = excluded.platform, source = excluded.source,
		usage = excluded.usage, features = excluded.features, experimental = excluded.experimental,
		n64_header = excluded.n64_header, n64_md5 = excluded.n64_md5, n64_md5_n64 = excluded.n64_md5_n64,
		identified_at = excluded.identified_at`,
		p, size, mod, version, string(r.Status), r.TitleID, r.SaveID, r.RawSerial, r.Platform,
		string(r.Source), string(r.Usage), r.Features, experimental, r.N64Header, r.N64MD5, r.N64MD5N64,
		time.Now().UTC().Format(time.RFC3339))
	return err
}
