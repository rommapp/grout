package library

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenCreatesLibraryTables(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if _, err := os.Stat(filepath.Join(dir, ".cache", "library.db")); err != nil {
		t.Fatal(err)
	}

	want := []string{"platforms", "games", "game_files", "game_meta", "collections", "collection_games", "cursors", "platform_sync", "local_files", "identities", "downloads", "exit_conditions"}
	for _, name := range want {
		var n int
		if err := s.DB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", name).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Errorf("table %s missing", name)
		}
	}

	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := s.DB.Exec(q, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec("INSERT INTO platforms (id, slug, fs_slug, name) VALUES (1, 'nes', 'nes', 'NES')")
	exec("INSERT INTO games (id, platform_id, name, kind) VALUES (10, 1, 'Game', 'single')")
	exec("INSERT INTO game_files (game_id, file_id, file_name) VALUES (10, 100, 'a.nes')")
	exec("INSERT INTO game_meta (game_id, dim, value) VALUES (10, 'genre', 'rpg')")
	exec("DELETE FROM games WHERE id = 10")
	for _, table := range []string{"game_files", "game_meta"} {
		var n int
		if err := s.DB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Errorf("%s rows remain after games delete: %d", table, n)
		}
	}

	exec("INSERT INTO collections (kind, remote_id, name) VALUES ('regular', 1, 'C')")
	exec("INSERT INTO collection_games (kind, remote_id, game_id) VALUES ('regular', 1, 999)")
	exec("DELETE FROM collections WHERE kind = 'regular' AND remote_id = 1")
	var n int
	if err := s.DB.QueryRow("SELECT COUNT(*) FROM collection_games").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("collection_games rows remain: %d", n)
	}
}
