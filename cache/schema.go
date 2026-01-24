package cache

import (
	"database/sql"
	"time"
)

const schemaVersion = 5

// nowUTC returns the current UTC time formatted as RFC3339 for consistent datetime storage
func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339)
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
			data_json TEXT NOT NULL,
			updated_at TEXT,
			cached_at TEXT NOT NULL
		)
	`)
	if err != nil {
		return err
	}

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
