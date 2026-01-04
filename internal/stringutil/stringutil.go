package stringutil

import (
	"fmt"
	"path/filepath"
	"strings"
)

// StripExtension removes the file extension from a filename.
func StripExtension(filename string) string {
	return strings.TrimSuffix(filename, filepath.Ext(filename))
}

// FormatBytes formats a byte count as a human-readable string.
func FormatBytes(bytes int64) string {
	const unit int64 = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := unit, 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
