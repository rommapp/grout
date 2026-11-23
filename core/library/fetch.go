package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"
)

type GameKind string

const (
	KindSingle GameKind = "single"
	KindNested GameKind = "nested"
	KindMulti  GameKind = "multi"
)

func GameKinds() []GameKind {
	return []GameKind{KindSingle, KindNested, KindMulti}
}

var ErrUnknownGame = errors.New("unknown game")

type GameFile struct {
	ID       int    `json:"id"`
	FileName string `json:"file_name"`
	Size     int64  `json:"size_bytes"`
	Category string `json:"category"`
}

type FetchInfo struct {
	GameID      int
	Name        string
	Kind        GameKind
	FSName      string
	FSNameNoExt string
	Platform    Platform
	Files       []GameFile
}

type Download struct {
	GameID     int
	PlatformID int
	FileID     int
	LaunchPath string
	Placed     []string
}

func (l *Library) FetchInfo(ctx context.Context, gameID int) (FetchInfo, error) {
	info := FetchInfo{GameID: gameID}
	var platformID int
	var kind string
	err := l.Store.DB.QueryRowContext(ctx, `SELECT platform_id, name, kind, fs_name, fs_name_no_ext FROM games WHERE id = ?`, gameID).
		Scan(&platformID, &info.Name, &kind, &info.FSName, &info.FSNameNoExt)
	if errors.Is(err, sql.ErrNoRows) {
		return FetchInfo{}, fmt.Errorf("game %d: %w", gameID, ErrUnknownGame)
	}
	if err != nil {
		return FetchInfo{}, err
	}
	info.Kind = GameKind(kind)
	files, err := l.playableFiles(ctx, gameID)
	if err != nil {
		return FetchInfo{}, err
	}
	info.Files = files
	all, err := l.AllPlatforms(ctx)
	if err != nil {
		return FetchInfo{}, err
	}
	info.Platform = Platform{ID: platformID}
	for _, p := range all {
		if p.ID == platformID {
			info.Platform = p
			break
		}
	}
	return info, nil
}

func (l *Library) GameFiles(ctx context.Context, gameID int) ([]GameFile, error) {
	if _, err := l.gameKind(ctx, gameID); err != nil {
		return nil, err
	}
	return l.playableFiles(ctx, gameID)
}

func (l *Library) gameKind(ctx context.Context, gameID int) (GameKind, error) {
	var kind string
	err := l.Store.DB.QueryRowContext(ctx, `SELECT kind FROM games WHERE id = ?`, gameID).Scan(&kind)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("game %d: %w", gameID, ErrUnknownGame)
	}
	return GameKind(kind), err
}

func (l *Library) playableFiles(ctx context.Context, gameID int) ([]GameFile, error) {
	rows, err := l.Store.DB.QueryContext(ctx, `SELECT file_id, file_name, size_bytes, category FROM game_files
		WHERE game_id = ? AND playable = 1 ORDER BY file_name, file_id`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GameFile
	for rows.Next() {
		var f GameFile
		if err := rows.Scan(&f.ID, &f.FileName, &f.Size, &f.Category); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func romRel(dirs []string, p string) (string, bool) {
	_, rel, ok := romSplit(dirs, p)
	return rel, ok
}

func RomDirOf(dirs []string, p string) (string, bool) {
	dir, _, ok := romSplit(dirs, p)
	return dir, ok
}

func romSplit(dirs []string, p string) (string, string, bool) {
	for _, dir := range dirs {
		dir = path.Clean(dir)
		if !strings.HasPrefix(p, dir+"/") {
			continue
		}
		rel := p[len(dir)+1:]
		parts := strings.Split(rel, "/")
		if len(parts) > 2 {
			continue
		}
		hidden := false
		for _, part := range parts {
			if part == "" || strings.HasPrefix(part, ".") {
				hidden = true
			}
		}
		if hidden {
			continue
		}
		return dir, rel, true
	}
	return "", "", false
}

func (l *Library) placedFiles(ctx context.Context, d Download) ([]localFile, error) {
	if l.Firmware == nil || l.Firmware.Host == nil || len(d.Placed) == 0 {
		return nil, nil
	}
	all, err := l.load(ctx)
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, p := range all {
		if p.ID == d.PlatformID {
			dirs = p.RomDirs
		}
	}
	var out []localFile
	for _, placed := range d.Placed {
		p := path.Clean(placed)
		rel, ok := romRel(dirs, p)
		if !ok {
			continue
		}
		info, err := l.Firmware.Host.Stat(p)
		if err != nil {
			return nil, fmt.Errorf("recording %s: %w", p, err)
		}
		if !info.Mode().IsRegular() {
			continue
		}
		out = append(out, localFile{
			path:     p,
			rel:      rel,
			basename: path.Base(p),
			size:     info.Size(),
			modTime:  info.ModTime().Unix(),
		})
	}
	return out, nil
}

func (l *Library) RecordFetch(ctx context.Context, d Download) error {
	files, err := l.placedFiles(ctx, d)
	if err != nil {
		return err
	}
	tx, err := l.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO downloads (game_id, platform_id, file_id, launch_path, fetched_at) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(game_id) DO UPDATE SET platform_id = excluded.platform_id, file_id = excluded.file_id,
		launch_path = excluded.launch_path, fetched_at = excluded.fetched_at`,
		d.GameID, d.PlatformID, d.FileID, d.LaunchPath, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("recording download of game %d: %w", d.GameID, err)
	}
	for _, f := range files {
		if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO local_files (path, platform_id, rel, basename, size_bytes, mod_time, game_id)
			VALUES (?, ?, ?, ?, ?, ?, ?)`, f.path, d.PlatformID, f.rel, f.basename, f.size, f.modTime, d.GameID); err != nil {
			return fmt.Errorf("recording %s: %w", f.path, err)
		}
	}
	return tx.Commit()
}

func (l *Library) Downloaded(ctx context.Context, gameID int) (Download, bool, error) {
	d := Download{GameID: gameID}
	err := l.Store.DB.QueryRowContext(ctx, `SELECT platform_id, file_id, launch_path FROM downloads WHERE game_id = ?`, gameID).
		Scan(&d.PlatformID, &d.FileID, &d.LaunchPath)
	if errors.Is(err, sql.ErrNoRows) {
		return Download{}, false, nil
	}
	if err != nil {
		return Download{}, false, err
	}
	rows, err := l.Store.DB.QueryContext(ctx, `SELECT path FROM local_files WHERE game_id = ? ORDER BY path`, gameID)
	if err != nil {
		return Download{}, false, err
	}
	defer rows.Close()
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return Download{}, false, err
		}
		d.Placed = append(d.Placed, p)
	}
	if err := rows.Err(); err != nil {
		return Download{}, false, err
	}
	return d, true, nil
}

func (l *Library) LaunchFile(ctx context.Context, gameID int) (string, bool, error) {
	kind, err := l.gameKind(ctx, gameID)
	if err != nil {
		return "", false, err
	}
	var launch string
	err = l.Store.DB.QueryRowContext(ctx, `SELECT launch_path FROM downloads WHERE game_id = ?`, gameID).Scan(&launch)
	if err == nil {
		return launch, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", false, err
	}
	if kind == KindMulti {
		err := l.Store.DB.QueryRowContext(ctx, `SELECT path FROM local_files WHERE game_id = ? AND basename LIKE '%.m3u' ORDER BY path LIMIT 1`, gameID).Scan(&launch)
		if err == nil {
			return launch, true, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return "", false, err
		}
		var rel string
		err = l.Store.DB.QueryRowContext(ctx, `SELECT path, rel FROM local_files WHERE game_id = ?
			ORDER BY instr(rel, '/') = 0, rel, path LIMIT 1`, gameID).Scan(&launch, &rel)
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		if err != nil {
			return "", false, err
		}
		if strings.Contains(rel, "/") {
			return path.Dir(launch), true, nil
		}
		return launch, true, nil
	}
	err = l.Store.DB.QueryRowContext(ctx, `SELECT path FROM local_files WHERE game_id = ? ORDER BY basename COLLATE NOCASE, path LIMIT 1`, gameID).Scan(&launch)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return launch, true, nil
}
