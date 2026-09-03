// Package tables loads the lookup tables that ship inside the binary: which
// folders a firmware accepts for a platform, where its saves go, which BIOS
// files a system needs.
package tables

import (
	"embed"
	"encoding/json"
	"fmt"
)

// Load reads an embedded JSON object into a map.
func Load[K comparable, V any](fs embed.FS, path string) (map[K]V, error) {
	data, err := fs.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	var result map[K]V
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}

	return result, nil
}

// MustLoad is Load, panicking on failure. A table that ships with the binary
// and will not parse is a build problem, not a runtime condition.
func MustLoad[K comparable, V any](fs embed.FS, path string) map[K]V {
	result, err := Load[K, V](fs, path)
	if err != nil {
		panic(err)
	}
	return result
}
