package saves

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// putAway moves a save into the .backup folder beside it, named the way the
// backups made before a download are, and returns where it went.
func putAway(path, fileName string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(filepath.Dir(path), ".backup")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	ext := filepath.Ext(fileName)
	base := strings.TrimSuffix(fileName, ext)
	stamp := info.ModTime().Format("2006-01-02 15-04-05")

	target := filepath.Join(dir, fmt.Sprintf("%s [%s]%s", base, stamp, ext))
	for n := 2; ; n++ {
		if _, err := os.Stat(target); os.IsNotExist(err) {
			break
		}
		target = filepath.Join(dir, fmt.Sprintf("%s [%s] (%d)%s", base, stamp, n, ext))
	}
	if err := os.Rename(path, target); err != nil {
		return "", err
	}
	return target, nil
}

func undoPutAway(original, backup string) {
	if original != "" && backup != "" {
		os.Rename(backup, original)
	}
}
