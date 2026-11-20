package library

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/rommapp/grout/core/romm"
)

const purgeBatch = 400

func fold(s string) string {
	var b strings.Builder
	space := true
	for _, r := range norm.NFD.String(strings.ToLower(s)) {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			space = false
			continue
		}
		if !space {
			b.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSuffix(b.String(), " ")
}

func stamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func savePlatforms(tx *sql.Tx, ps []romm.Platform) error {
	stmt, err := tx.Prepare(`INSERT INTO platforms (id, slug, fs_slug, name, custom_name, rom_count, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET slug = excluded.slug, fs_slug = excluded.fs_slug, name = excluded.name,
			custom_name = excluded.custom_name, rom_count = excluded.rom_count, updated_at = excluded.updated_at`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, p := range ps {
		if _, err := stmt.Exec(p.ID, p.Slug, p.FSSlug, p.Name, p.CustomName, p.RomCount, stamp(p.UpdatedAt)); err != nil {
			return fmt.Errorf("saving platform %d: %w", p.ID, err)
		}
	}
	return nil
}

func gameKind(r romm.Rom) string {
	switch {
	case r.HasMultipleFiles:
		return "multi"
	case r.HasNestedSingleFile:
		return "nested"
	}
	return "single"
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

type metaDim struct {
	dim    string
	values []string
}

func gameMeta(r romm.Rom) []metaDim {
	m := r.Metadatum
	return []metaDim{
		{"genre", m.Genres},
		{"franchise", m.Franchises},
		{"company", m.Companies},
		{"game_mode", m.GameModes},
		{"age_rating", m.AgeRatings},
		{"region", r.Regions},
		{"language", r.Languages},
		{"tag", r.Tags},
	}
}

func saveGames(tx *sql.Tx, rs []romm.Rom) error {
	upsert, err := tx.Prepare(`INSERT INTO games (id, platform_id, name, sort_key, search_name, summary, fs_name, fs_name_no_ext,
			fs_extension, fs_size_bytes, kind, has_manual, missing_from_fs, player_count, first_release_date, average_rating,
			cover_small, cover_large, cover_url, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET platform_id = excluded.platform_id, name = excluded.name, sort_key = excluded.sort_key,
			search_name = excluded.search_name, summary = excluded.summary, fs_name = excluded.fs_name,
			fs_name_no_ext = excluded.fs_name_no_ext, fs_extension = excluded.fs_extension,
			fs_size_bytes = excluded.fs_size_bytes, kind = excluded.kind, has_manual = excluded.has_manual,
			missing_from_fs = excluded.missing_from_fs, player_count = excluded.player_count,
			first_release_date = excluded.first_release_date, average_rating = excluded.average_rating,
			cover_small = excluded.cover_small, cover_large = excluded.cover_large, cover_url = excluded.cover_url,
			updated_at = excluded.updated_at`)
	if err != nil {
		return err
	}
	defer upsert.Close()
	delFiles, err := tx.Prepare(`DELETE FROM game_files WHERE game_id = ?`)
	if err != nil {
		return err
	}
	defer delFiles.Close()
	delMeta, err := tx.Prepare(`DELETE FROM game_meta WHERE game_id = ?`)
	if err != nil {
		return err
	}
	defer delMeta.Close()
	insFile, err := tx.Prepare(`INSERT OR REPLACE INTO game_files (game_id, file_id, file_name, file_path, size_bytes, category, crc, md5, sha1, playable)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer insFile.Close()
	insMeta, err := tx.Prepare(`INSERT OR IGNORE INTO game_meta (game_id, dim, value) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer insMeta.Close()

	for _, r := range rs {
		display := r.DisplayName()
		sortKey := fold(r.NameSortKey)
		if sortKey == "" {
			sortKey = fold(display)
		}
		search := fold(display) + " " + fold(r.FSNameNoExt)
		var releaseDate int64
		if r.Metadatum.FirstReleaseDate != nil {
			releaseDate = *r.Metadatum.FirstReleaseDate
		}
		var rating float64
		if r.Metadatum.AverageRating != nil {
			rating = *r.Metadatum.AverageRating
		}
		if _, err := upsert.Exec(r.ID, r.PlatformID, display, sortKey, search, r.Summary, r.FSName, r.FSNameNoExt,
			r.FSExtension, r.FSSizeBytes, gameKind(r), boolInt(r.HasManual), boolInt(r.MissingFromFS),
			r.Metadatum.PlayerCount, releaseDate, rating, r.PathCoverSmall, r.PathCoverLarge, r.URLCover,
			stamp(r.UpdatedAt)); err != nil {
			return fmt.Errorf("saving game %d: %w", r.ID, err)
		}
		if _, err := delFiles.Exec(r.ID); err != nil {
			return err
		}
		if _, err := delMeta.Exec(r.ID); err != nil {
			return err
		}
		playable := map[int]bool{}
		for _, f := range r.PlayableFiles() {
			playable[f.ID] = true
		}
		for _, f := range r.Files {
			if _, err := insFile.Exec(r.ID, f.ID, f.FileName, f.FilePath, f.FileSizeBytes, f.Category,
				f.CRCHash, f.MD5Hash, f.SHA1Hash, boolInt(playable[f.ID])); err != nil {
				return fmt.Errorf("saving file %d of game %d: %w", f.ID, r.ID, err)
			}
		}
		for _, d := range gameMeta(r) {
			for _, v := range d.values {
				if v == "" {
					continue
				}
				if _, err := insMeta.Exec(r.ID, d.dim, v); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func saveCollection(tx *sql.Tx, c romm.Collection) error {
	if _, err := tx.Exec(`INSERT INTO collections (kind, remote_id, name, updated_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(kind, remote_id) DO UPDATE SET name = excluded.name, updated_at = excluded.updated_at`,
		string(c.Kind), c.RemoteID, c.Name, stamp(c.UpdatedAt)); err != nil {
		return fmt.Errorf("saving collection %s/%s: %w", c.Kind, c.RemoteID, err)
	}
	if _, err := tx.Exec(`DELETE FROM collection_games WHERE kind = ? AND remote_id = ?`, string(c.Kind), c.RemoteID); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO collection_games (kind, remote_id, game_id) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, id := range c.RomIDs {
		if _, err := stmt.Exec(string(c.Kind), c.RemoteID, id); err != nil {
			return err
		}
	}
	return nil
}

func fillKeep(tx *sql.Tx, keep []string) error {
	if _, err := tx.Exec(`DROP TABLE IF EXISTS temp.purge_keep`); err != nil {
		return err
	}
	if _, err := tx.Exec(`CREATE TEMP TABLE purge_keep (v)`); err != nil {
		return err
	}
	for start := 0; start < len(keep); start += purgeBatch {
		end := min(start+purgeBatch, len(keep))
		batch := keep[start:end]
		args := make([]any, len(batch))
		for i, v := range batch {
			args[i] = v
		}
		q := `INSERT INTO purge_keep (v) VALUES ` + strings.TrimSuffix(strings.Repeat("(?),", len(batch)), ",")
		if _, err := tx.Exec(q, args...); err != nil {
			return err
		}
	}
	return nil
}

func dropKeep(tx *sql.Tx) {
	_, _ = tx.Exec(`DROP TABLE IF EXISTS temp.purge_keep`)
}

func purgeExcept(tx *sql.Tx, table, column string, keep []string) (int, error) {
	if len(keep) == 0 {
		res, err := tx.Exec(`DELETE FROM ` + table)
		if err != nil {
			return 0, err
		}
		n, err := res.RowsAffected()
		return int(n), err
	}
	if err := fillKeep(tx, keep); err != nil {
		return 0, err
	}
	defer dropKeep(tx)
	res, err := tx.Exec(`DELETE FROM ` + table + ` WHERE ` + column + ` NOT IN (SELECT v FROM purge_keep)`)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}

func purgeCollectionsExcept(tx *sql.Tx, kind romm.CollectionKind, keep []string) (int, error) {
	if len(keep) == 0 {
		res, err := tx.Exec(`DELETE FROM collections WHERE kind = ?`, string(kind))
		if err != nil {
			return 0, err
		}
		n, err := res.RowsAffected()
		return int(n), err
	}
	if err := fillKeep(tx, keep); err != nil {
		return 0, err
	}
	defer dropKeep(tx)
	res, err := tx.Exec(`DELETE FROM collections WHERE kind = ? AND remote_id NOT IN (SELECT v FROM purge_keep)`, string(kind))
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}
