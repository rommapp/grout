package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
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

const stateExpr = `CASE WHEN EXISTS (SELECT 1 FROM downloads d WHERE d.game_id = g.id) THEN 2 ELSE CASE g.kind
	WHEN 'multi' THEN CASE WHEN EXISTS (SELECT 1 FROM local_files lf WHERE lf.game_id = g.id) THEN 2 ELSE 0 END
	WHEN 'single' THEN CASE WHEN ` + haveCount + ` > 0 THEN 2 ELSE 0 END
	ELSE CASE
		WHEN ` + haveCount + ` = 0 THEN 0
		WHEN ` + haveCount + ` = ` + playableCount + ` THEN 2
		ELSE 1
	END
END END`

const stagingPrefix = ".grout-"

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
			if w, ok := l.Firmware.Host.(host.Writable); ok {
				sweepStaging(l.log(), w, dir)
			}
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
		if err := l.pruneDownloads(ctx, p.ID); err != nil {
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

func sweepStaging(log *slog.Logger, w host.Writable, dir string) {
	real, err := w.Real(dir)
	if err != nil {
		return
	}
	entries, err := os.ReadDir(real)
	if err != nil {
		return
	}
	remove := func(p string) {
		if err := os.RemoveAll(p); err != nil {
			log.Warn("scan: removing staging leftover", "path", p, "error", err)
		}
	}
	for _, e := range entries {
		full := filepath.Join(real, e.Name())
		if strings.HasPrefix(e.Name(), stagingPrefix) {
			remove(full)
			continue
		}
		if !e.IsDir() {
			continue
		}
		subs, err := os.ReadDir(full)
		if err != nil {
			continue
		}
		for _, sub := range subs {
			if strings.HasPrefix(sub.Name(), stagingPrefix) {
				remove(filepath.Join(full, sub.Name()))
			}
		}
	}
}

func (l *Library) pruneDownloads(ctx context.Context, platformID int) error {
	rows, err := l.Store.DB.QueryContext(ctx, `SELECT game_id, launch_path FROM downloads WHERE platform_id = ?`, platformID)
	if err != nil {
		return err
	}
	type entry struct {
		id     int
		launch string
	}
	var all []entry
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.id, &e.launch); err != nil {
			rows.Close()
			return err
		}
		all = append(all, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, e := range all {
		if _, err := l.Firmware.Host.Stat(e.launch); !errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if _, err := l.Store.DB.ExecContext(ctx, `DELETE FROM downloads WHERE game_id = ?`, e.id); err != nil {
			return err
		}
	}
	return nil
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

type matchIndex struct {
	m3u     map[string]int
	folders map[string]int
	names   map[string]int
}

func (l *Library) platformIndex(ctx context.Context, tx *sql.Tx, platformID int) (matchIndex, error) {
	idx := matchIndex{m3u: map[string]int{}, folders: map[string]int{}, names: map[string]int{}}
	rows, err := tx.QueryContext(ctx, `SELECT id, fs_name, fs_name_no_ext FROM games WHERE platform_id = ? AND kind = 'multi' ORDER BY id DESC`, platformID)
	if err != nil {
		return idx, err
	}
	for rows.Next() {
		var id int
		var fsName, name string
		if err := rows.Scan(&id, &fsName, &name); err != nil {
			rows.Close()
			return idx, err
		}
		idx.m3u[strings.ToLower(name)] = id
		if fsName == "" {
			fsName = name
		}
		if fsName != "" {
			idx.folders[strings.ToLower(l.Firmware.MultiLayout(fsName, nil).Folder)] = id
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return idx, err
	}
	rows, err = tx.QueryContext(ctx, `SELECT g.id, gf.file_name FROM game_files gf JOIN games g ON g.id = gf.game_id
		WHERE g.platform_id = ? AND g.kind != 'multi' AND gf.playable = 1 ORDER BY g.id DESC`, platformID)
	if err != nil {
		return idx, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return idx, err
		}
		idx.names[strings.ToLower(name)] = id
	}
	return idx, rows.Err()
}

func (idx matchIndex) match(f localFile) int {
	lower := strings.ToLower(f.basename)
	if strings.HasSuffix(lower, ".m3u") {
		if id := idx.m3u[strings.TrimSuffix(lower, ".m3u")]; id != 0 {
			return id
		}
	}
	if folder, _, ok := strings.Cut(f.rel, "/"); ok {
		if id := idx.folders[strings.ToLower(folder)]; id != 0 {
			return id
		}
	}
	return idx.names[lower]
}

func (l *Library) replaceFiles(ctx context.Context, platformID int, files []localFile) (int, error) {
	tx, err := l.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	idx, err := l.platformIndex(ctx, tx, platformID)
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
		id := idx.match(f)
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
