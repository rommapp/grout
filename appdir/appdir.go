// Package appdir decides where grout keeps its own files. Each location can be
// moved with an environment variable, for installs whose working directory is
// not the right place for them; unset, they are where grout always kept them.
package appdir

import (
	"os"
	"path/filepath"
)

// DataDir holds the settings files: config.json, save_slots.json and
// input_mapping.json. GROUT_DATA_DIR overrides the working directory.
func DataDir() string {
	if dir := os.Getenv("GROUT_DATA_DIR"); dir != "" {
		return dir
	}
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

// CacheDir holds the SQLite database and the artwork cache. GROUT_CACHE_DIR
// overrides {DataDir}/.cache.
func CacheDir() string {
	if dir := os.Getenv("GROUT_CACHE_DIR"); dir != "" {
		return dir
	}
	return filepath.Join(DataDir(), ".cache")
}

// TmpDir stages downloads before they are moved into place. GROUT_TMP_DIR
// overrides {DataDir}/.tmp.
func TmpDir() string {
	if dir := os.Getenv("GROUT_TMP_DIR"); dir != "" {
		return dir
	}
	return filepath.Join(DataDir(), ".tmp")
}

// SystemTmpDir holds short-lived scratch files: update archives and save zips.
// GROUT_TMP_DIR overrides the system temp directory.
func SystemTmpDir() string {
	if dir := os.Getenv("GROUT_TMP_DIR"); dir != "" {
		return dir
	}
	return os.TempDir()
}

// UpdateStagingDir holds a downloaded update until the launch script applies
// it. GROUT_UPDATE_DIR overrides {installRoot}/.update.
func UpdateStagingDir(installRoot string) string {
	if dir := os.Getenv("GROUT_UPDATE_DIR"); dir != "" {
		return dir
	}
	return filepath.Join(installRoot, ".update")
}
