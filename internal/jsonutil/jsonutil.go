package jsonutil

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func LoadJSONMap[K comparable, V any](fs embed.FS, path string, overridePrefix string) (map[K]V, error) {
	var data []byte
	var err error

	overridePath := filepath.Join("overrides", overridePrefix, path)
	if fileData, readErr := os.ReadFile(overridePath); readErr == nil {
		data = fileData
	} else {
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

func MustLoadJSONMap[K comparable, V any](fs embed.FS, path string, overridePrefix string) map[K]V {
	result, err := LoadJSONMap[K, V](fs, path, overridePrefix)
	if err != nil {
		panic(err)
	}
	return result
}
