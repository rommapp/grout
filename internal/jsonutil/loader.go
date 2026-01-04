package jsonutil

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// LoadJSONMap loads a JSON file from the embedded FS or an override file.
// overridePrefix is prepended to the path when checking for override files.
func LoadJSONMap[K comparable, V any](fs embed.FS, path string, overridePrefix string) (map[K]V, error) {
	var data []byte
	var err error

	// Check for override file in current working directory
	overridePath := filepath.Join("overrides", overridePrefix, path)
	if fileData, readErr := os.ReadFile(overridePath); readErr == nil {
		data = fileData
	} else {
		// Fall back to embedded file
		data, err = fs.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", path, err)
		}
	}

	var result map[K]V
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}

	return result, nil
}

// MustLoadJSONMap loads a JSON file and panics on error.
func MustLoadJSONMap[K comparable, V any](fs embed.FS, path string, overridePrefix string) map[K]V {
	result, err := LoadJSONMap[K, V](fs, path, overridePrefix)
	if err != nil {
		panic(err)
	}
	return result
}
