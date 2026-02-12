package cache

import (
	"database/sql"
	"fmt"
	"strconv"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

const schemaVersion = 7

// nowUTC returns the current UTC time formatted as RFC3339 for consistent datetime storage
func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339)
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

	return nil
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
			has_bios INTEGER DEFAULT 0,
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

	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS bios_availability (
			platform_id INTEGER PRIMARY KEY,
			has_bios INTEGER NOT NULL DEFAULT 0,
			checked_at TEXT NOT NULL
		)
	`)
	if err != nil {
		return err
	}

	// filename_mappings stores user's local filenames that differ from RomM's fs_name
	// This enables matching orphan ROMs by hash and remembering the association
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS filename_mappings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			platform_fs_slug TEXT NOT NULL,
			local_filename_no_ext TEXT NOT NULL,
			rom_id INTEGER NOT NULL,
			rom_name TEXT NOT NULL,
			matched_at TEXT NOT NULL,
			UNIQUE(platform_fs_slug, local_filename_no_ext)
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_filename_mappings_lookup ON filename_mappings(platform_fs_slug, local_filename_no_ext)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS failed_lookups (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			platform_fs_slug TEXT NOT NULL,
			local_filename_no_ext TEXT NOT NULL,
			last_attempt TEXT NOT NULL,
			UNIQUE(platform_fs_slug, local_filename_no_ext)
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_failed_lookups ON failed_lookups(platform_fs_slug, local_filename_no_ext)`)
	if err != nil {
		return err
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
		INSERT OR REPLACE INTO cache_metadata (key, value, updated_at)
		VALUES ('schema_version', ?, ?)
	`, schemaVersion, nowUTC())
	if err != nil {
		return err
	}

	return tx.Commit()
}
