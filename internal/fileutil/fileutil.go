package fileutil

import (
	"archive/zip"
	"bufio"
	"crypto/sha1"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bodgit/sevenzip"
	"go.uber.org/atomic"
)

const (
	DefaultBufferSize = 128 * 1024 // 128KB
	SmallBufferSize   = 32 * 1024  // 32KB
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

func CopyFile(src, dest string) error {
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

func DeleteFile(path string) error {
	return os.Remove(path)
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

	buffer := make([]byte, DefaultBufferSize)

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

func Un7zip(archivePath string, destDir string, progress *atomic.Float64) error {
	reader, err := sevenzip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open 7z file: %w", err)
	}
	defer reader.Close()

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	buffer := make([]byte, DefaultBufferSize)

	createdDirs := make(map[string]bool)
	createdDirs[destDir] = true

	var totalBytes uint64
	for _, file := range reader.File {
		if !file.FileInfo().IsDir() {
			totalBytes += file.UncompressedSize
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

		if err := extract7zFile(file, filePath, buffer, totalBytes, &extractedBytes, progress); err != nil {
			return fmt.Errorf("failed to extract file %s: %w", file.Name, err)
		}
	}

	return nil
}

func ZipFileNames(zipPath string) ([]string, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open zip file: %w", err)
	}
	defer reader.Close()

	var names []string
	for _, file := range reader.File {
		if !file.FileInfo().IsDir() {
			names = append(names, file.Name)
		}
	}
	return names, nil
}

func SevenZipFileNames(archivePath string) ([]string, error) {
	reader, err := sevenzip.OpenReader(archivePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open 7z file: %w", err)
	}
	defer reader.Close()

	var names []string
	for _, file := range reader.File {
		if !file.FileInfo().IsDir() {
			names = append(names, file.Name)
		}
	}
	return names, nil
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func extract7zFile(file *sevenzip.File, destPath string, buffer []byte, totalBytes uint64, extractedBytes *uint64, progress *atomic.Float64) error {
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

	bufWriter := bufio.NewWriterSize(destFile, SmallBufferSize)
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

	bufWriter := bufio.NewWriterSize(destFile, SmallBufferSize)
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

func FilterVisibleFiles(entries []os.DirEntry) []os.DirEntry {
	result := make([]os.DirEntry, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			result = append(result, entry)
		}
	}
	return result
}

func FilterHiddenDirectories(entries []os.DirEntry) []os.DirEntry {
	result := make([]os.DirEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			result = append(result, entry)
		}
	}
	return result
}

// ComputeCRC32 computes the CRC32 hash of a file and returns it as an uppercase hex string
func ComputeCRC32(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hash := crc32.NewIEEE()
	buffer := make([]byte, DefaultBufferSize)

	if _, err := io.CopyBuffer(hash, file, buffer); err != nil {
		return "", fmt.Errorf("failed to compute hash: %w", err)
	}

	return fmt.Sprintf("%08X", hash.Sum32()), nil
}

// ComputeSHA1 computes the SHA1 hash of a file and returns it as a lowercase hex string
func ComputeSHA1(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hash := sha1.New()
	buffer := make([]byte, DefaultBufferSize)

	if _, err := io.CopyBuffer(hash, file, buffer); err != nil {
		return "", fmt.Errorf("failed to compute hash: %w", err)
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
