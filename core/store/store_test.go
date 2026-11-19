package store

import (
	"path/filepath"
	"sync"
	"testing"
)

var testTables = []Table{
	{Name: "games", DDL: []string{"CREATE TABLE games (id INTEGER PRIMARY KEY, name TEXT)", "CREATE INDEX idx_games_name ON games(name)"}},
}

func TestOpenCreatesTablesAndVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	s, err := Open(path, testTables)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var v int
	s.DB.QueryRow("PRAGMA user_version").Scan(&v)
	if v != SchemaVersion {
		t.Fatalf("version %d", v)
	}
	if _, err := s.DB.Exec("INSERT INTO games (id, name) VALUES (1, 'a')"); err != nil {
		t.Fatal(err)
	}
}

func TestOpenSetsPragmas(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), FileName), testTables)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var mode string
	var fk int
	s.DB.QueryRow("PRAGMA journal_mode").Scan(&mode)
	s.DB.QueryRow("PRAGMA foreign_keys").Scan(&fk)
	if mode != "wal" || fk != 1 {
		t.Fatalf("journal_mode=%s foreign_keys=%d", mode, fk)
	}
}

func TestReopenKeepsData(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	s, _ := Open(path, testTables)
	s.DB.Exec("INSERT INTO games (id, name) VALUES (1, 'a')")
	s.Close()
	s, _ = Open(path, testTables)
	defer s.Close()
	var n int
	s.DB.QueryRow("SELECT COUNT(*) FROM games").Scan(&n)
	if n != 1 {
		t.Fatalf("rows %d", n)
	}
}

func TestOtherVersionRecreates(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	s, _ := Open(path, testTables)
	s.DB.Exec("INSERT INTO games (id, name) VALUES (1, 'a')")
	s.DB.Exec("PRAGMA user_version = 99")
	s.Close()
	s, err := Open(path, testTables)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var n int
	s.DB.QueryRow("SELECT COUNT(*) FROM games").Scan(&n)
	if n != 0 {
		t.Fatal("old data survived a version change")
	}
	var v int
	s.DB.QueryRow("PRAGMA user_version").Scan(&v)
	if v != SchemaVersion {
		t.Fatalf("version %d", v)
	}
}

func TestMissingRegisteredTableIsCreated(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	s, _ := Open(path, testTables)
	s.Close()
	more := append(testTables, Table{Name: "extra", DDL: []string{"CREATE TABLE extra (id INTEGER)"}})
	s, err := Open(path, more)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.DB.Exec("INSERT INTO extra (id) VALUES (1)"); err != nil {
		t.Fatal(err)
	}
}

func TestBadDDLFailsAndLeavesVersionUnset(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	bad := []Table{{Name: "broken", DDL: []string{"CREATE TABLE broken (", "x"}}}
	if _, err := Open(path, bad); err == nil {
		t.Fatal("expected error")
	}
	s, err := Open(path, testTables)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var v int
	s.DB.QueryRow("PRAGMA user_version").Scan(&v)
	if v != SchemaVersion {
		t.Fatalf("version %d", v)
	}
}

func TestConcurrentWritersDoNotFailBusy(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), FileName), testTables)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var wg sync.WaitGroup
	errs := make(chan error, 400)
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				tx, err := s.DB.Begin()
				if err != nil {
					errs <- err
					continue
				}
				if _, err := tx.Exec("INSERT INTO games (id, name) VALUES (?, 'x')", w*1000+i); err != nil {
					tx.Rollback()
					errs <- err
					continue
				}
				if err := tx.Commit(); err != nil {
					errs <- err
				}
			}
		}(w)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}

func TestVersionChangeWithForeignKeysRecreates(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	tables := []Table{
		{Name: "platforms", DDL: []string{"CREATE TABLE platforms (id INTEGER PRIMARY KEY)"}},
		{Name: "games", DDL: []string{"CREATE TABLE games (id INTEGER PRIMARY KEY, platform_id INTEGER REFERENCES platforms(id))"}},
	}
	s, err := Open(path, tables)
	if err != nil {
		t.Fatal(err)
	}
	s.DB.Exec("INSERT INTO platforms (id) VALUES (1)")
	s.DB.Exec("INSERT INTO games (id, platform_id) VALUES (1, 1)")
	s.DB.Exec("PRAGMA user_version = 99")
	s.Close()
	s, err = Open(path, tables)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var n int
	s.DB.QueryRow("SELECT COUNT(*) FROM games").Scan(&n)
	if n != 0 {
		t.Fatal("old data survived a version change")
	}
}
