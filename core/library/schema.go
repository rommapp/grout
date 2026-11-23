package library

import "github.com/rommapp/grout/core/store"

func Tables() []store.Table {
	return []store.Table{
		{Name: "platforms", DDL: []string{
			`CREATE TABLE platforms (
				id INTEGER PRIMARY KEY,
				slug TEXT NOT NULL DEFAULT '',
				fs_slug TEXT NOT NULL DEFAULT '',
				name TEXT NOT NULL DEFAULT '',
				custom_name TEXT NOT NULL DEFAULT '',
				rom_count INTEGER NOT NULL DEFAULT 0,
				updated_at TEXT NOT NULL DEFAULT ''
			)`,
			`CREATE INDEX idx_platforms_fs_slug ON platforms(fs_slug COLLATE NOCASE)`,
		}},
		{Name: "games", DDL: []string{
			`CREATE TABLE games (
				id INTEGER PRIMARY KEY,
				platform_id INTEGER NOT NULL,
				name TEXT NOT NULL DEFAULT '',
				sort_key TEXT NOT NULL DEFAULT '',
				search_name TEXT NOT NULL DEFAULT '',
				summary TEXT NOT NULL DEFAULT '',
				fs_name TEXT NOT NULL DEFAULT '',
				fs_name_no_ext TEXT NOT NULL DEFAULT '',
				fs_extension TEXT NOT NULL DEFAULT '',
				fs_size_bytes INTEGER NOT NULL DEFAULT 0,
				kind TEXT NOT NULL DEFAULT 'single' CHECK (kind IN ('single', 'nested', 'multi')),
				has_manual INTEGER NOT NULL DEFAULT 0,
				missing_from_fs INTEGER NOT NULL DEFAULT 0,
				player_count TEXT NOT NULL DEFAULT '',
				first_release_date INTEGER NOT NULL DEFAULT 0,
				average_rating REAL NOT NULL DEFAULT 0,
				cover_small TEXT NOT NULL DEFAULT '',
				cover_large TEXT NOT NULL DEFAULT '',
				cover_url TEXT NOT NULL DEFAULT '',
				updated_at TEXT NOT NULL DEFAULT ''
			)`,
			`CREATE INDEX idx_games_platform_sort ON games(platform_id, sort_key)`,
		}},
		{Name: "game_files", DDL: []string{
			`CREATE TABLE game_files (
				game_id INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
				file_id INTEGER NOT NULL,
				file_name TEXT NOT NULL DEFAULT '',
				file_path TEXT NOT NULL DEFAULT '',
				size_bytes INTEGER NOT NULL DEFAULT 0,
				category TEXT NOT NULL DEFAULT '',
				crc TEXT NOT NULL DEFAULT '',
				md5 TEXT NOT NULL DEFAULT '',
				sha1 TEXT NOT NULL DEFAULT '',
				playable INTEGER NOT NULL DEFAULT 0,
				PRIMARY KEY (game_id, file_id)
			)`,
			`CREATE INDEX idx_game_files_name ON game_files(file_name COLLATE NOCASE)`,
		}},
		{Name: "game_meta", DDL: []string{
			`CREATE TABLE game_meta (
				game_id INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
				dim TEXT NOT NULL,
				value TEXT NOT NULL,
				PRIMARY KEY (game_id, dim, value)
			)`,
			`CREATE INDEX idx_game_meta_dim_value ON game_meta(dim, value)`,
		}},
		{Name: "collections", DDL: []string{
			`CREATE TABLE collections (
				kind TEXT NOT NULL,
				remote_id TEXT NOT NULL,
				name TEXT NOT NULL DEFAULT '',
				updated_at TEXT NOT NULL DEFAULT '',
				PRIMARY KEY (kind, remote_id)
			)`,
		}},
		{Name: "collection_games", DDL: []string{
			`CREATE TABLE collection_games (
				kind TEXT NOT NULL,
				remote_id TEXT NOT NULL,
				game_id INTEGER NOT NULL,
				PRIMARY KEY (kind, remote_id, game_id),
				FOREIGN KEY (kind, remote_id) REFERENCES collections(kind, remote_id) ON DELETE CASCADE
			)`,
			`CREATE INDEX idx_collection_games_game ON collection_games(game_id)`,
		}},
		{Name: "cursors", DDL: []string{
			`CREATE TABLE cursors (
				stream TEXT PRIMARY KEY,
				value TEXT NOT NULL DEFAULT ''
			)`,
		}},
		{Name: "platform_sync", DDL: []string{
			`CREATE TABLE platform_sync (
				platform_id INTEGER PRIMARY KEY REFERENCES platforms(id) ON DELETE CASCADE,
				cursor TEXT NOT NULL DEFAULT '',
				status TEXT NOT NULL DEFAULT 'ok' CHECK (status IN ('ok', 'failed')),
				attempted_at TEXT NOT NULL DEFAULT ''
			)`,
		}},
		{Name: "local_files", DDL: []string{
			`CREATE TABLE local_files (
				path TEXT PRIMARY KEY,
				platform_id INTEGER NOT NULL,
				rel TEXT NOT NULL DEFAULT '',
				basename TEXT NOT NULL DEFAULT '',
				size_bytes INTEGER NOT NULL DEFAULT 0,
				mod_time INTEGER NOT NULL DEFAULT 0,
				game_id INTEGER NOT NULL DEFAULT 0
			)`,
			`CREATE INDEX idx_local_files_platform_basename ON local_files(platform_id, basename COLLATE NOCASE)`,
			`CREATE INDEX idx_local_files_game ON local_files(game_id)`,
		}},
		{Name: "downloads", DDL: []string{
			`CREATE TABLE downloads (
				game_id INTEGER PRIMARY KEY,
				platform_id INTEGER NOT NULL,
				file_id INTEGER NOT NULL DEFAULT 0,
				launch_path TEXT NOT NULL,
				fetched_at TEXT NOT NULL
			)`,
		}},
		{Name: "exit_conditions", DDL: []string{
			`CREATE TABLE exit_conditions (
				seq INTEGER PRIMARY KEY AUTOINCREMENT,
				condition TEXT NOT NULL,
				fired_at TEXT NOT NULL
			)`,
		}},
	}
}
