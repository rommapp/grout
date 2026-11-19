package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

const FileName = "settings.json"

type Source int

const (
	SourceSettings Source = iota
	SourceImportedV2
	SourceNew
)

func (s Source) String() string {
	switch s {
	case SourceSettings:
		return "settings"
	case SourceImportedV2:
		return "imported_v2"
	default:
		return "new"
	}
}

func Load(dir string) (*Config, Source, error) {
	path := filepath.Join(dir, FileName)
	data, err := os.ReadFile(path)
	if err == nil {
		var c Config
		if err := json.Unmarshal(data, &c); err != nil {
			return nil, SourceSettings, fmt.Errorf("parsing %s: %w", path, err)
		}
		applyDefaults(&c)
		return &c, SourceSettings, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, SourceSettings, fmt.Errorf("reading %s: %w", path, err)
	}
	imported, found, err := importV2(dir)
	if err != nil {
		return nil, SourceImportedV2, err
	}
	if found {
		if err := Save(dir, imported); err != nil {
			return nil, SourceImportedV2, err
		}
		return imported, SourceImportedV2, nil
	}
	c := &Config{}
	applyDefaults(c)
	return c, SourceNew, nil
}

func Save(dir string, c *Config) error {
	applyDefaults(c)
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding settings: %w", err)
	}
	path := filepath.Join(dir, FileName)
	if err := writeFileAtomic(path, data, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, perm); err != nil {
		return err
	}
	return os.Rename(name, path)
}
