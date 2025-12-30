package utils

import (
	"archive/zip"
	"bufio"
	"fmt"
	"grout/constants"
	"io"
	"os"
	"path/filepath"
	"strings"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"go.uber.org/atomic"
)

type progressWriter struct {
	writer         io.Writer
	totalBytes     uint64
	extractedBytes *uint64
	progress       *atomic.Float64
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n, err := pw.writer.Write(p)
	if n > 0 && pw.progress != nil && pw.totalBytes > 0 {
		*pw.extractedBytes += uint64(n)
		pw.progress.Store(float64(*pw.extractedBytes) / float64(pw.totalBytes))
	}
	return n, err
}

func TempDir() string {
	wd, err := os.Getwd()
	if err != nil {
		return os.TempDir()
	}

	return filepath.Join(wd, ".tmp")
}

func copyFile(src, dest string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	destinationFile, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destinationFile.Close()

	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	err = destinationFile.Sync()
	if err != nil {
		return fmt.Errorf("failed to sync destination file: %w", err)
	}

	return nil
}

func DeleteFile(path string) bool {
	logger := gaba.GetLogger()

	err := os.Remove(path)
	if err != nil {
		logger.Error("Issue removing file",
			"path", path,
			"error", err)
		return false
	}
	return true
}

func Unzip(zipPath string, destDir string, progress *atomic.Float64) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip file: %w", err)
	}
	defer reader.Close()

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	buffer := make([]byte, constants.DefaultBufferSize)

	createdDirs := make(map[string]bool)
	createdDirs[destDir] = true

	var totalBytes uint64
	for _, file := range reader.File {
		if !file.FileInfo().IsDir() {
			totalBytes += file.UncompressedSize64
		}
	}

	var extractedBytes uint64

	for _, file := range reader.File {
		filePath := filepath.Join(destDir, file.Name)

		if file.FileInfo().IsDir() {
			if !createdDirs[filePath] {
				if err := os.MkdirAll(filePath, file.Mode()); err != nil {
					return fmt.Errorf("failed to create directory %s: %w", filePath, err)
				}
				createdDirs[filePath] = true
			}
			continue
		}

		parentDir := filepath.Dir(filePath)
		if !createdDirs[parentDir] {
			if err := os.MkdirAll(parentDir, 0755); err != nil {
				return fmt.Errorf("failed to create parent directory for %s: %w", filePath, err)
			}
			createdDirs[parentDir] = true
		}

		if err := extractFile(file, filePath, buffer, totalBytes, &extractedBytes, progress); err != nil {
			return fmt.Errorf("failed to extract file %s: %w", file.Name, err)
		}
	}

	return nil
}

func extractFile(file *zip.File, destPath string, buffer []byte, totalBytes uint64, extractedBytes *uint64, progress *atomic.Float64) error {
	srcFile, err := file.Open()
	if err != nil {
		return err
	}
	defer srcFile.Close()

	destFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
	if err != nil {
		return err
	}
	defer destFile.Close()

	bufWriter := bufio.NewWriterSize(destFile, constants.SmallBufferSize)
	defer bufWriter.Flush()

	progressW := &progressWriter{
		writer:         bufWriter,
		totalBytes:     totalBytes,
		extractedBytes: extractedBytes,
		progress:       progress,
	}

	_, err = io.CopyBuffer(progressW, srcFile, buffer)
	if err != nil {
		return err
	}

	return bufWriter.Flush()
}

func OrganizeMultiFileRomForMuOS(extractDir, romDirectory, gameName string) error {
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

// FilterVisibleFiles returns non-directory entries that don't start with "."
func FilterVisibleFiles(entries []os.DirEntry) []os.DirEntry {
	result := make([]os.DirEntry, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			result = append(result, entry)
		}
	}
	return result
}

// FilterVisibleDirectories returns directory entries that don't start with "."
func FilterVisibleDirectories(entries []os.DirEntry) []os.DirEntry {
	result := make([]os.DirEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			result = append(result, entry)
		}
	}
	return result
}
