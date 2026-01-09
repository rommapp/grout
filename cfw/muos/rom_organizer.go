package muos

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

// OrganizeMultiFileRom handles multi-file ROM extraction and M3U file organization for muOS.
// It renames the extract directory to have an underscore prefix and updates M3U paths accordingly.
func OrganizeMultiFileRom(extractDir, romDirectory, gameName string) error {
	logger := gaba.GetLogger()

	var m3uFile string
	entries, err := os.ReadDir(extractDir)
	if err != nil {
		return fmt.Errorf("failed to read extracted directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".m3u" {
			m3uFile = filepath.Join(extractDir, entry.Name())
			break
		}
	}

	if m3uFile == "" {
		underscoreDir := filepath.Join(romDirectory, "_"+gameName)
		if err := os.Rename(extractDir, underscoreDir); err != nil {
			return fmt.Errorf("failed to rename directory to %s: %w", underscoreDir, err)
		}
		logger.Debug("No .m3u file found, renamed directory", "from", extractDir, "to", underscoreDir)
		return nil
	}

	m3uContent, err := os.ReadFile(m3uFile)
	if err != nil {
		return fmt.Errorf("failed to read .m3u file: %w", err)
	}

	lines := strings.Split(string(m3uContent), "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines[i] = filepath.Join("_"+gameName, line)
	}
	updatedM3U := strings.Join(lines, "\n")

	m3uDestPath := filepath.Join(romDirectory, gameName+".m3u")
	if err := os.WriteFile(m3uDestPath, []byte(updatedM3U), 0644); err != nil {
		return fmt.Errorf("failed to write updated .m3u file: %w", err)
	}
	logger.Debug("Moved and updated .m3u file", "from", m3uFile, "to", m3uDestPath)

	if err := os.Remove(m3uFile); err != nil {
		logger.Warn("Failed to remove original .m3u file", "path", m3uFile, "error", err)
	}

	underscoreDir := filepath.Join(romDirectory, "_"+gameName)
	if err := os.Rename(extractDir, underscoreDir); err != nil {
		return fmt.Errorf("failed to rename directory to %s: %w", underscoreDir, err)
	}
	logger.Debug("Renamed directory for muOS", "from", extractDir, "to", underscoreDir)

	return nil
}
