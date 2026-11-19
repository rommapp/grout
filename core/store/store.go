package store

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"

	_ "modernc.org/sqlite"
)

const (
	FileName      = "library.db"
	SchemaVersion = 1
)

type Table struct {
	Name string
	DDL  []string
}

type Store struct {
	DB *sql.DB
}

func Open(path string, tables []Table) (*Store, error) {
	db, err := openDB(path)
	if err != nil {
		return nil, err
	}
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		db.Close()
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if version != 0 && version != SchemaVersion {
		db.Close()
		for _, suffix := range []string{"", "-wal", "-shm"} {
			if err := os.Remove(path + suffix); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return nil, fmt.Errorf("removing %s: %w", path+suffix, err)
			}
		}
		if db, err = openDB(path); err != nil {
			return nil, err
		}
	}
	if err := prepare(db, tables); err != nil {
		db.Close()
		return nil, fmt.Errorf("preparing %s: %w", path, err)
	}
	return &Store{DB: db}, nil
}

func openDB(path string) (*sql.DB, error) {
	dsn := "file:" + (&url.URL{Path: path}).EscapedPath() + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

func (s *Store) Close() error {
	return s.DB.Close()
}

func prepare(db *sql.DB, tables []Table) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, t := range tables {
		var n int
		if err := tx.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", t.Name).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		for _, stmt := range t.DDL {
			if _, err := tx.Exec(stmt); err != nil {
				return fmt.Errorf("creating %s: %w", t.Name, err)
			}
		}
	}
	if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", SchemaVersion)); err != nil {
		return err
	}
	return tx.Commit()
}
