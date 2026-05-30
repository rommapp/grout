package appdir

import (
	"os"
	"path/filepath"
)

// DataDir returns the base directory for config files (config.json, save_slots.json)
// Override with GROUT_DATA_DIR env var; falls back to the process working directory
func DataDir() string {
	if d := os.Getenv("GROUT_DATA_DIR"); d != "" {
		return d
	}
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

// CacheDir returns the cache directory (SQLite DB, artwork)
// Override with GROUT_CACHE_DIR env var; falls back to {DataDir}/.cache
func CacheDir() string {
	if d := os.Getenv("GROUT_CACHE_DIR"); d != "" {
		return d
	}
	dataDir := filepath.Join(DataDir(), ".cache")
	if dataDir == "." {
		return os.TempDir()
	}

	return dataDir
}

// TmpDir returns the directory used for temporary files (zip archives, downloads)
// Override with GROUT_TMP_DIR env var; falls back to {DataDir}/.tmp
func TmpDir() string {
	if d := os.Getenv("GROUT_TMP_DIR"); d != "" {
		return d
	}
	return filepath.Join(DataDir(), ".tmp")
}

// BackupDir returns the directory used to store save backups
// Override with GROUT_BACKUP_DIR env var; falls back to a .backup/ sibling of saveFilePath
func BackupDir(saveFilePath string) string {
	if d := os.Getenv("GROUT_BACKUP_DIR"); d != "" {
		return d
	}
	return filepath.Join(filepath.Dir(saveFilePath), ".backup")
}

// UpdateStagingDir returns the staging directory for pending updates
// Override with GROUT_UPDATE_DIR env var; falls back to {installRoot}/.update
func UpdateStagingDir(installRoot string) string {
	if d := os.Getenv("GROUT_UPDATE_DIR"); d != "" {
		return d
	}
	return filepath.Join(installRoot, ".update")
}
