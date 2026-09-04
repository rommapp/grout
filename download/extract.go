package download

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/atomic"

	"grout/archive"
	"grout/cfw"
	"grout/files"
	"grout/romm"
)

// IsArchive reports whether a rom arrived as an archive worth unpacking.
func IsArchive(fileName string) bool {
	switch strings.ToLower(filepath.Ext(fileName)) {
	case ".zip", ".7z":
		return true
	}
	return false
}

// ExtractMultiFile unpacks a game that arrived as a single archive holding
// several files, and returns the path the frontend should point at.
//
// The archive lands in a temp directory rather than the rom directory, so a
// failure here leaves nothing half-unpacked where the frontend would find it.
func ExtractMultiFile(game romm.Rom, romDirectory string, progress *atomic.Float64) (string, error) {
	archivePath := MultiFileArchivePath(game)
	extractDir := filepath.Join(romDirectory, game.FsNameNoExt)

	if err := archive.Unzip(archivePath, extractDir, progress); err != nil {
		os.Remove(archivePath)
		return "", fmt.Errorf("unpacking %s: %w", game.Name, err)
	}

	if err := cfw.OrganizeExtractedRom(extractDir, romDirectory, game.FsNameNoExt); err != nil {
		os.Remove(archivePath)
		os.RemoveAll(extractDir)
		return "", fmt.Errorf("arranging %s for this firmware: %w", game.Name, err)
	}

	if err := os.Remove(archivePath); err != nil {
		slog.Default().Warn("Could not remove the downloaded archive", "path", archivePath, "error", err)
	}

	return multiFileGamePath(romDirectory, extractDir, game.FsNameNoExt), nil
}

// ExtractArchive unpacks a rom that arrived as a .zip or .7z straight into its
// rom directory, and returns the path the frontend should point at.
//
// The archive is removed once it is unpacked. A failure leaves it alone, so
// the game is still playable from the archive if the frontend can read one.
func ExtractArchive(archivePath, romDirectory string, progress *atomic.Float64) (string, error) {
	sevenZip := strings.EqualFold(filepath.Ext(archivePath), ".7z")

	var contents []string
	var err error
	if sevenZip {
		contents, err = archive.SevenZipFileNames(archivePath)
	} else {
		contents, err = archive.ZipFileNames(archivePath)
	}
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", filepath.Base(archivePath), err)
	}

	if sevenZip {
		err = archive.Un7zip(archivePath, romDirectory, progress)
	} else {
		err = archive.Unzip(archivePath, romDirectory, progress)
	}
	if err != nil {
		return "", fmt.Errorf("unpacking %s: %w", filepath.Base(archivePath), err)
	}

	if err := os.Remove(archivePath); err != nil {
		slog.Default().Warn("Could not remove the archive after unpacking", "path", archivePath, "error", err)
	}

	return filepath.Join(romDirectory, playablePath(contents)), nil
}

// playablePath picks what to launch out of an archive's contents: the playlist
// if there is one, since it is what ties several discs together, and otherwise
// the first file.
func playablePath(contents []string) string {
	if len(contents) == 0 {
		return ""
	}
	for _, name := range contents {
		if strings.EqualFold(filepath.Ext(name), ".m3u") {
			return name
		}
	}
	return contents[0]
}

// multiFileGamePath finds what to launch after a multi-file game is unpacked.
//
// A firmware that rearranges the files may have lifted a playlist up into the
// rom directory, so that is looked for first. Failing everything the extract
// directory itself is named, which at least points somewhere real.
func multiFileGamePath(romDirectory, extractDir, baseName string) string {
	logger := slog.Default()

	if playlist := filepath.Join(romDirectory, baseName+".m3u"); files.FileExists(playlist) {
		return playlist
	}

	entries, err := os.ReadDir(extractDir)
	if err != nil {
		logger.Debug("Cannot read the extract directory, naming it instead", "path", extractDir, "error", err)
		return extractDir
	}

	firstFile := ""
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Ext(entry.Name()), ".m3u") {
			return filepath.Join(extractDir, entry.Name())
		}
		if firstFile == "" {
			firstFile = filepath.Join(extractDir, entry.Name())
		}
	}

	if firstFile == "" {
		logger.Debug("Nothing was unpacked, naming the directory instead", "path", extractDir)
		return extractDir
	}
	return firstFile
}
