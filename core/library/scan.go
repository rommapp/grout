package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path"
	"strings"

	"github.com/rommapp/grout/core/firmware/host"
)

type ScanResult struct {
	Files          int      `json:"files"`
	Matched        int      `json:"matched"`
	Platforms      int      `json:"platforms"`
	SkippedFolders []string `json:"skipped_folders"`
}

type localFile struct {
	path     string
	rel      string
	basename string
	size     int64
	modTime  int64
}

const playableCount = `(SELECT COUNT(*) FROM game_files gf WHERE gf.game_id = g.id AND gf.playable = 1)`

const haveCount = `(SELECT COUNT(*) FROM game_files gf WHERE gf.game_id = g.id AND gf.playable = 1
	AND EXISTS (SELECT 1 FROM local_files lf WHERE lf.game_id = g.id AND lf.basename = gf.file_name COLLATE NOCASE))`

const stateExpr = `CASE g.kind
	WHEN 'multi' THEN CASE WHEN EXISTS (SELECT 1 FROM local_files lf WHERE lf.game_id = g.id AND lf.basename LIKE '%.m3u') THEN 2 ELSE 0 END
	WHEN 'single' THEN CASE WHEN ` + haveCount + ` > 0 THEN 2 ELSE 0 END
	ELSE CASE
		WHEN ` + haveCount + ` = 0 THEN 0
		WHEN ` + haveCount + ` = ` + playableCount + ` THEN 2
		ELSE 1
	END
END`

func (l *Library) Scan(ctx context.Context, progress func(Progress)) (ScanResult, error) {
	res := ScanResult{SkippedFolders: []string{}}
	if l.Firmware == nil {
		return res, errors.New("scan: no firmware resolved")
	}
	all, err := l.load(ctx)
	if err != nil {
		return res, err
	}
	for i, p := range all {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		if progress != nil {
			progress(Progress{Stage: StageScan, Done: i, Total: len(all)})
		}
		if !p.Enabled {
			if _, err := l.Store.DB.ExecContext(ctx, `DELETE FROM local_files WHERE platform_id = ?`, p.ID); err != nil {
				return res, err
			}
			continue
		}
		var files []localFile
		for _, dir := range p.RomDirs {
			found, skipped, err := walkRomDir(l.log(), l.Firmware.Host, dir)
			if err != nil {
				return res, err
			}
			files = append(files, found...)
			res.SkippedFolders = append(res.SkippedFolders, skipped...)
		}
		matched, err := l.replaceFiles(ctx, p.ID, files)
		if err != nil {
			return res, err
		}
		res.Files += len(files)
		res.Matched += matched
		res.Platforms++
	}
	if progress != nil {
		progress(Progress{Stage: StageScan, Done: len(all), Total: len(all)})
	}
	return res, nil
}

func readRomDir(log *slog.Logger, h host.Host, dir string, skipped *[]string) ([]fs.DirEntry, bool) {
	entries, err := h.ReadDir(dir)
	if err == nil {
		return entries, true
	}
	if !errors.Is(err, fs.ErrNotExist) {
		log.Warn("scan: skipping unreadable folder", "path", dir, "error", err)
		*skipped = append(*skipped, dir)
	}
	return nil, false
}

func walkRomDir(log *slog.Logger, h host.Host, dir string) ([]localFile, []string, error) {
	var skipped []string
	entries, ok := readRomDir(log, h, dir, &skipped)
	if !ok {
		return nil, skipped, nil
	}
	var out []localFile
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if !e.IsDir() {
			f, ok, err := fileEntry(dir, e.Name(), e.Name(), e)
			if err != nil {
				return nil, nil, err
			}
			if ok {
				out = append(out, f)
			}
			continue
		}
		sub := path.Join(dir, e.Name())
		children, ok := readRomDir(log, h, sub, &skipped)
		if !ok {
			continue
		}
		for _, c := range children {
			if c.IsDir() || strings.HasPrefix(c.Name(), ".") {
				continue
			}
			f, ok, err := fileEntry(sub, c.Name(), e.Name()+"/"+c.Name(), c)
			if err != nil {
				return nil, nil, err
			}
			if ok {
				out = append(out, f)
			}
		}
	}
	return out, skipped, nil
}

func fileEntry(dir, name, rel string, e fs.DirEntry) (localFile, bool, error) {
	info, err := e.Info()
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return localFile{}, false, nil
		}
		return localFile{}, false, err
	}
	if !info.Mode().IsRegular() {
		return localFile{}, false, nil
	}
	return localFile{
		path:     path.Join(dir, name),
		rel:      rel,
		basename: name,
		size:     info.Size(),
		modTime:  info.ModTime().Unix(),
	}, true, nil
}

func (l *Library) platformIndex(ctx context.Context, tx *sql.Tx, platformID int) (map[string]int, map[string]int, error) {
	m3u := map[string]int{}
	rows, err := tx.QueryContext(ctx, `SELECT id, fs_name_no_ext FROM games WHERE platform_id = ? AND kind = 'multi' ORDER BY id DESC`, platformID)
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			rows.Close()
			return nil, nil, err
		}
		m3u[strings.ToLower(name)] = id
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	names := map[string]int{}
	rows, err = tx.QueryContext(ctx, `SELECT g.id, gf.file_name FROM game_files gf JOIN games g ON g.id = gf.game_id
		WHERE g.platform_id = ? AND gf.playable = 1 ORDER BY g.id DESC`, platformID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, nil, err
		}
		names[strings.ToLower(name)] = id
	}
	return m3u, names, rows.Err()
}

func (l *Library) replaceFiles(ctx context.Context, platformID int, files []localFile) (int, error) {
	tx, err := l.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	m3u, names, err := l.platformIndex(ctx, tx, platformID)
	if err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM local_files WHERE platform_id = ?`, platformID); err != nil {
		return 0, err
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT OR REPLACE INTO local_files (path, platform_id, rel, basename, size_bytes, mod_time, game_id)
		VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	matched := 0
	for _, f := range files {
		lower := strings.ToLower(f.basename)
		id := 0
		if strings.HasSuffix(lower, ".m3u") {
			id = m3u[strings.TrimSuffix(lower, ".m3u")]
		}
		if id == 0 {
			id = names[lower]
		}
		if id != 0 {
			matched++
		}
		if _, err := stmt.ExecContext(ctx, f.path, platformID, f.rel, f.basename, f.size, f.modTime, id); err != nil {
			return 0, fmt.Errorf("recording %s: %w", f.path, err)
		}
	}
	return matched, tx.Commit()
}

func (l *Library) State(ctx context.Context, gameID int) (DownloadState, error) {
	var s int
	err := l.Store.DB.QueryRowContext(ctx, `SELECT `+stateExpr+` FROM games g WHERE g.id = ?`, gameID).Scan(&s)
	if err != nil {
		return NotDownloaded, err
	}
	return DownloadState(s), nil
}
