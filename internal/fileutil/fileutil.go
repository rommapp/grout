package fileutil

import (
	"archive/zip"
	"bufio"
	"crypto/md5"
	"crypto/sha1"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

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

// ComputeMD5 returns the lowercase hex MD5 of a file's bytes.
// Matches the server's plain-file content hash (md5, hex).
func ComputeMD5(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hash := md5.New()
	buf := make([]byte, DefaultBufferSize)
	if _, err := io.CopyBuffer(hash, file, buf); err != nil {
		return "", fmt.Errorf("failed to compute hash: %w", err)
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// compositeFromPairs joins "name:hash" lines (sorted by name) and MD5s the result.
func compositeFromPairs(pairs map[string]string) string {
	names := make([]string, 0, len(pairs))
	for n := range pairs {
		names = append(names, n)
	}
	sort.Strings(names)
	lines := make([]string, 0, len(names))
	for _, n := range names {
		lines = append(lines, n+":"+pairs[n])
	}
	sum := md5.Sum([]byte(strings.Join(lines, "\n")))
	return fmt.Sprintf("%x", sum)
}

// ComputeCompositeZipHash hashes a zip the way the RomM server does: md5 of each
// non-directory entry, then md5 of the sorted "name:filehash" lines joined by "\n".
func ComputeCompositeZipHash(zipPath string) (string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf("failed to open zip: %w", err)
	}
	defer r.Close()

	pairs := make(map[string]string, len(r.File))
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("failed to open zip entry %s: %w", f.Name, err)
		}
		h := md5.New()
		if _, err := io.Copy(h, rc); err != nil {
			rc.Close()
			return "", fmt.Errorf("failed to hash zip entry %s: %w", f.Name, err)
		}
		rc.Close()
		pairs[f.Name] = fmt.Sprintf("%x", h.Sum(nil))
	}
	return compositeFromPairs(pairs), nil
}

// DirHashStat is the composite content hash plus aggregate metadata for one or more
// save directories, gathered in a single walk.
type DirHashStat struct {
	Hash   string    // server-compatible composite hash
	Newest time.Time // newest file mtime (second-truncated)
	Size   int64     // total bytes across hashed files
}

// ComputeDirsCompositeHashStat walks the save directories ONCE, computing the
// server-compatible composite hash AND the newest mtime / total size, mirroring how
// addDirToZip names entries (relative to each directory's parent) so the hash equals
// what the server computes for the uploaded zip. Dot-prefixed files/dirs are skipped
// (consistently for hash, size, and mtime). Each file is closed immediately after
// hashing so a large save can't exhaust file descriptors.
func ComputeDirsCompositeHashStat(dirPaths []string) (DirHashStat, error) {
	pairs := make(map[string]string)
	var stat DirHashStat
	for _, dirPath := range dirPaths {
		parent := filepath.Dir(dirPath)
		walkErr := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			rel, relErr := filepath.Rel(parent, path)
			if relErr != nil {
				return relErr
			}
			if strings.HasPrefix(filepath.Base(rel), ".") {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if info.IsDir() {
				return nil // directory entries are excluded from the hash
			}
			f, openErr := os.Open(path)
			if openErr != nil {
				return openErr
			}
			h := md5.New()
			_, copyErr := io.Copy(h, f)
			f.Close()
			if copyErr != nil {
				return copyErr
			}
			pairs[rel] = fmt.Sprintf("%x", h.Sum(nil))
			stat.Size += info.Size()
			if mt := info.ModTime(); mt.After(stat.Newest) {
				stat.Newest = mt
			}
			return nil
		})
		if walkErr != nil {
			return DirHashStat{}, fmt.Errorf("failed to walk %s: %w", dirPath, walkErr)
		}
	}
	stat.Hash = compositeFromPairs(pairs)
	stat.Newest = stat.Newest.Truncate(time.Second)
	return stat, nil
}

// ComputeDirsCompositeHash returns just the composite hash for the given save
// directories (see ComputeDirsCompositeHashStat).
func ComputeDirsCompositeHash(dirPaths []string) (string, error) {
	stat, err := ComputeDirsCompositeHashStat(dirPaths)
	return stat.Hash, err
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
