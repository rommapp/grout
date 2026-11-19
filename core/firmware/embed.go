package firmware

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"testing/fstest"
)

const placeholderID = "testfw"

//go:embed */firmware.toml */input/*.json all:*/mock */mock.env
var embedded embed.FS

func IDs() []string {
	entries, err := embedded.ReadDir(".")
	if err != nil {
		return nil
	}
	var ids []string
	for _, e := range entries {
		if !e.IsDir() || e.Name() == placeholderID {
			continue
		}
		if _, err := embedded.ReadFile(path.Join(e.Name(), "firmware.toml")); err == nil {
			ids = append(ids, e.Name())
		}
	}
	sort.Strings(ids)
	return ids
}

func Load(id string) (*Definition, error) {
	data, err := embedded.ReadFile(path.Join(id, "firmware.toml"))
	if err != nil {
		return nil, fmt.Errorf("firmware %q: %w", id, err)
	}
	d, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("firmware %q: %w", id, err)
	}
	if d.ID != id {
		return nil, fmt.Errorf("firmware %q: id %q does not match directory", id, d.ID)
	}
	return d, nil
}

func LoadAll() ([]*Definition, error) {
	var out []*Definition
	for _, id := range IDs() {
		d, err := Load(id)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

func InputFile(id, name string) ([]byte, error) {
	return embedded.ReadFile(path.Join(id, "input", name))
}

func MockFS(id string) (fs.FS, error) {
	dir := path.Join(id, "mock")
	if _, err := fs.Stat(embedded, dir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fstest.MapFS{}, nil
		}
		return nil, fmt.Errorf("firmware %q: %w", id, err)
	}
	return fs.Sub(embedded, dir)
}

func MockEnv(id string) (map[string]string, error) {
	env := map[string]string{}
	data, err := embedded.ReadFile(path.Join(id, "mock.env"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return env, nil
		}
		return nil, fmt.Errorf("firmware %q: %w", id, err)
	}
	for i, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(k) == "" {
			return nil, fmt.Errorf("firmware %q: mock.env line %d: %q is not KEY=VALUE", id, i+1, line)
		}
		env[strings.TrimSpace(k)] = v
	}
	return env, nil
}
