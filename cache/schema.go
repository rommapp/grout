package cache

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"grout/romm"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

const schemaVersion = 13

// nowUTC returns the current UTC time formatted as RFC3339 for consistent datetime storage
func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// execer is the subset of *sql.DB / *sql.Tx used by the small schema helpers, so they can
// run inside createTables' transaction or directly against the DB during a migration.
type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

// createGameBasenamesTable creates the table that indexes every on-disk basename a game can
// occupy (one row per file), keyed for fast (platform_fs_slug, basename) -> game_id lookup.
// Used both by createTables and by the v13 migration backfill (issue #242).
func createGameBasenamesTable(db execer) error {
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS game_basenames (
			game_id INTEGER NOT NULL,
			platform_fs_slug TEXT NOT NULL,
			basename TEXT NOT NULL,
			PRIMARY KEY (game_id, basename)
		)
	`); err != nil {
		return err
	}
	_, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_game_basenames_lookup ON game_basenames(platform_fs_slug, basename)`)
	return err
}

// lookupTables lists all metadata lookup tables for use in migrations and cleanup
var lookupTables = []string{
	"genres",
	"franchises",
	"companies",
	"game_modes",
	"age_ratings",
	"regions",
	"languages",
	"tags",
}

// junctionTables lists all game metadata junction tables for use in migrations and cleanup
var junctionTables = []string{
	"game_genres",
	"game_franchises",
	"game_companies",
	"game_game_modes",
	"game_age_ratings",
	"game_regions",
	"game_languages",
	"game_tags",
}

// migrateIfNeeded checks the current schema version and runs migrations if required.
// Since this is a cache database, migration simply drops and recreates affected tables.
func migrateIfNeeded(db *sql.DB) error {
	logger := gaba.GetLogger()

	var versionStr string
	err := db.QueryRow(`SELECT value FROM cache_metadata WHERE key = 'schema_version'`).Scan(&versionStr)
	if err != nil {
		// Table doesn't exist yet or no version — fresh database, nothing to migrate
		return nil
	}

	currentVersion, err := strconv.Atoi(versionStr)
	if err != nil {
		return nil
	}

	if currentVersion >= schemaVersion {
		return nil
	}

	logger.Info("Migrating cache schema", "from", currentVersion, "to", schemaVersion)

	if currentVersion < 7 {
		if err := migrateToV7(db); err != nil {
			return fmt.Errorf("migration to v7 failed: %w", err)
		}
	}

	// v10 removes save_mappings, filename_mappings, and failed_lookups tables (save sync ripped out)
	if currentVersion < 10 {
		if _, err := db.Exec("DROP TABLE IF EXISTS save_mappings"); err != nil {
			return fmt.Errorf("migration to v10: drop save_mappings: %w", err)
		}
		if _, err := db.Exec("DROP TABLE IF EXISTS filename_mappings"); err != nil {
			return fmt.Errorf("migration to v10: drop filename_mappings: %w", err)
		}
		if _, err := db.Exec("DROP TABLE IF EXISTS failed_lookups"); err != nil {
			return fmt.Errorf("migration to v10: drop failed_lookups: %w", err)
		}
	}

	// v11 removes bios_availability table (now derived from platform firmware_count)
	if currentVersion < 11 {
		if _, err := db.Exec("DROP TABLE IF EXISTS bios_availability"); err != nil {
			return fmt.Errorf("migration to v11: drop bios_availability: %w", err)
		}
	}

	// v12 keys local ROM/save matching on the on-disk file basename
	// (expected_basename) instead of fs_name_no_ext, so nested-single-file ROMs
	// resolve correctly (issue #242). Drop games + game-keyed tables so the next
	// sync repopulates the new column.
	if currentVersion < 12 {
		if err := dropGamesForRepopulate(db); err != nil {
			return fmt.Errorf("migration to v12 failed: %w", err)
		}
	}

	// v13 adds game_basenames so a local ROM/save resolves by ANY of a multi-file game's
	// versions, not just Files[0] (issue #242). Backfill from the already-cached data_json:
	// no library re-download, no full re-cache (unlike a games drop). For a v11-or-older DB
	// the v12 step above already dropped games, so there's nothing to backfill and the next
	// sync repopulates both tables.
	if currentVersion < 13 {
		if err := backfillGameBasenames(db); err != nil {
			return fmt.Errorf("migration to v13 failed: %w", err)
		}
	}

	return nil
}

// backfillGameBasenames (re)builds the game_basenames index from each game's cached
// data_json. It runs no network calls, so existing caches gain multi-file matching on
// upgrade without re-downloading the library. Idempotent: clears then repopulates.
func backfillGameBasenames(db *sql.DB) error {
	if err := createGameBasenamesTable(db); err != nil {
		return err
	}

	rows, err := db.Query(`SELECT id, platform_fs_slug, data_json FROM games`)
	if err != nil {
		return err
	}
	type gameRow struct {
		id       int
		fsSlug   string
		dataJSON string
	}
	var games []gameRow
	for rows.Next() {
		var g gameRow
		if err := rows.Scan(&g.id, &g.fsSlug, &g.dataJSON); err != nil {
			rows.Close()
			return err
		}
		games = append(games, g)
	}
	rows.Close()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM game_basenames`); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO game_basenames (game_id, platform_fs_slug, basename) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, g := range games {
		var rom romm.Rom
		if err := json.Unmarshal([]byte(g.dataJSON), &rom); err != nil {
			// Skip unparseable rows; a later library refresh repopulates them.
			continue
		}
		fsSlug := g.fsSlug
		if fsSlug == "" {
			fsSlug = rom.PlatformFSSlug
		}
		for _, base := range rom.LocalBasenames() {
			if _, err := stmt.Exec(g.id, fsSlug, base); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

// dropGamesForRepopulate drops the games table and its game-keyed junction and
// collection-mapping tables so createTables can recreate them; the next sync refills.
func dropGamesForRepopulate(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	tablesToDrop := []string{"games", "game_collections"}
	tablesToDrop = append(tablesToDrop, junctionTables...)
	for _, table := range tablesToDrop {
		if _, err := tx.Exec("DROP TABLE IF EXISTS " + table); err != nil {
			return fmt.Errorf("failed to drop table %s: %w", table, err)
		}
	}
	return tx.Commit()
}

// migrateToV7 drops games and all related tables so they get
// recreated with the normalized schema by createTables. The next sync refills everything.
func migrateToV7(db *sql.DB) error {
	logger := gaba.GetLogger()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	tablesToDrop := []string{"games", "game_collections"}
	tablesToDrop = append(tablesToDrop, junctionTables...)
	tablesToDrop = append(tablesToDrop, lookupTables...)
	for _, table := range tablesToDrop {
		if _, err := tx.Exec("DROP TABLE IF EXISTS " + table); err != nil {
			return fmt.Errorf("failed to drop table %s: %w", table, err)
		}
	}

	logger.Info("Dropped games, junction, and lookup tables for v7 migration")
	return tx.Commit()
}

func createTables(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS cache_metadata (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS platforms (
			id INTEGER PRIMARY KEY,
			slug TEXT NOT NULL,
			fs_slug TEXT NOT NULL,
			name TEXT NOT NULL,
			api_name TEXT DEFAULT '',
			custom_name TEXT DEFAULT '',
			rom_count INTEGER DEFAULT 0,
			data_json TEXT NOT NULL,
			updated_at TEXT,
			cached_at TEXT NOT NULL
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_platforms_fs_slug ON platforms(fs_slug)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS collections (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			romm_id INTEGER,
			virtual_id TEXT,
			type TEXT NOT NULL,
			name TEXT NOT NULL,
			rom_count INTEGER DEFAULT 0,
			data_json TEXT NOT NULL,
			updated_at TEXT,
			cached_at TEXT NOT NULL,
			UNIQUE(romm_id, type),
			UNIQUE(virtual_id)
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_collections_type ON collections(type)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS games (
			id INTEGER PRIMARY KEY,
			platform_id INTEGER NOT NULL,
			platform_fs_slug TEXT NOT NULL,
			name TEXT NOT NULL,
			fs_name TEXT DEFAULT '',
			fs_name_no_ext TEXT DEFAULT '',
			expected_basename TEXT DEFAULT '',
			crc_hash TEXT DEFAULT '',
			md5_hash TEXT DEFAULT '',
			sha1_hash TEXT DEFAULT '',
			player_count INTEGER DEFAULT 1,
			first_release_date INTEGER DEFAULT 0,
			average_rating REAL DEFAULT 0,
			fs_size_bytes INTEGER DEFAULT 0,
			is_identified INTEGER DEFAULT 0,
			is_unidentified INTEGER DEFAULT 0,
			missing_from_fs INTEGER DEFAULT 0,
			has_manual INTEGER DEFAULT 0,
			has_multiple_files INTEGER DEFAULT 0,
			data_json TEXT NOT NULL,
			updated_at TEXT,
			cached_at TEXT NOT NULL
		)
	`)
	if err != nil {
		return err
	}

	// Existing games indexes
	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_games_platform_id ON games(platform_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_games_platform_fs_slug ON games(platform_fs_slug)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_games_fs_lookup ON games(platform_fs_slug, fs_name_no_ext)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_games_expected_basename ON games(platform_fs_slug, expected_basename)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_games_md5 ON games(md5_hash) WHERE md5_hash != ''`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_games_sha1 ON games(sha1_hash) WHERE sha1_hash != ''`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_games_crc ON games(crc_hash) WHERE crc_hash != ''`)
	if err != nil {
		return err
	}

	// New scalar column indexes
	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_games_release_date ON games(first_release_date) WHERE first_release_date > 0`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_games_rating ON games(average_rating) WHERE average_rating > 0`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_games_missing ON games(missing_from_fs) WHERE missing_from_fs = 1`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_games_platform_rating ON games(platform_id, average_rating)`)
	if err != nil {
		return err
	}

	// game_basenames indexes EVERY on-disk basename a game can occupy (one row per file),
	// so a local ROM/save resolves by any of a multi-file game's versions (issue #242).
	if err := createGameBasenamesTable(tx); err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_games_platform_release ON games(platform_id, first_release_date)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS game_collections (
			game_id INTEGER NOT NULL,
			collection_id INTEGER NOT NULL,
			PRIMARY KEY (game_id, collection_id)
		)
	`)
	if err != nil {
		return err
	}

	// Normalized lookup tables for metadata values
	lookupTableDefs := []struct{ table, column string }{
		{"genres", "name"},
		{"franchises", "name"},
		{"companies", "name"},
		{"game_modes", "name"},
		{"age_ratings", "name"},
		{"regions", "name"},
		{"languages", "name"},
		{"tags", "name"},
	}
	for _, def := range lookupTableDefs {
		_, err = tx.Exec(`
			CREATE TABLE IF NOT EXISTS ` + def.table + ` (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				` + def.column + ` TEXT NOT NULL UNIQUE
			)
		`)
		if err != nil {
			return err
		}
	}

	// Junction tables with integer foreign keys
	junctionTableDefs := []struct{ table, fkColumn, lookupTable string }{
		{"game_genres", "genre_id", "genres"},
		{"game_franchises", "franchise_id", "franchises"},
		{"game_companies", "company_id", "companies"},
		{"game_game_modes", "game_mode_id", "game_modes"},
		{"game_age_ratings", "age_rating_id", "age_ratings"},
		{"game_regions", "region_id", "regions"},
		{"game_languages", "language_id", "languages"},
		{"game_tags", "tag_id", "tags"},
	}
	for _, def := range junctionTableDefs {
		_, err = tx.Exec(`
			CREATE TABLE IF NOT EXISTS ` + def.table + ` (
				game_id INTEGER NOT NULL,
				` + def.fkColumn + ` INTEGER NOT NULL,
				PRIMARY KEY (game_id, ` + def.fkColumn + `)
			)
		`)
		if err != nil {
			return err
		}

		_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_` + def.table + `_` + def.fkColumn + ` ON ` + def.table + `(` + def.fkColumn + `)`)
		if err != nil {
			return err
		}
	}

	// Track per-platform game sync status
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS platform_sync_status (
			platform_id INTEGER PRIMARY KEY,
			last_successful_sync TEXT,
			last_attempt TEXT,
			games_synced INTEGER DEFAULT 0,
			status TEXT DEFAULT 'pending'
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS save_sync_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			rom_id INTEGER NOT NULL,
			rom_name TEXT NOT NULL,
			action TEXT NOT NULL,
			device_id TEXT NOT NULL,
			save_id INTEGER,
			file_name TEXT,
			synced_at TEXT NOT NULL
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_save_sync_history_rom_id ON save_sync_history(rom_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_save_sync_history_device_id ON save_sync_history(device_id)`)
	if err != nil {
		return err
	}

	// Current per-save sync state (one row per device+rom+file), upserted after each
	// successful upload/download. Distinct from the append-only save_sync_history log.
	// Gives downloaded saves a stable slot identity so they aren't re-uploaded to a
	// different slot on the next sync.
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS save_sync_state (
			device_id TEXT NOT NULL,
			rom_id INTEGER NOT NULL,
			file_name TEXT NOT NULL,
			slot TEXT NOT NULL,
			save_id INTEGER,
			content_hash TEXT,
			synced_at TEXT NOT NULL,
			PRIMARY KEY (device_id, rom_id, file_name)
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		INSERT OR REPLACE INTO cache_metadata (key, value, updated_at)
		VALUES ('schema_version', ?, ?)
	`, schemaVersion, nowUTC())
	if err != nil {
		return err
	}

	return tx.Commit()
}
