// Package hashing computes the file digests save sync uses to tell whether
// local and remote content differ.
package hashing

import (
	"archive/zip"
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
)

const readBufferSize = 128 * 1024

// DirHashStat is the composite content hash plus aggregate metadata for one or more
// save directories, gathered in a single walk.
type DirHashStat struct {
	Hash   string    // server-compatible composite hash
	Newest time.Time // newest file mtime (second-truncated)
	Size   int64     // total bytes across hashed files
}

// ComputeCRC32 computes the CRC32 hash of a file and returns it as an uppercase hex string
func ComputeCRC32(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hash := crc32.NewIEEE()
	buffer := make([]byte, readBufferSize)

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
	buf := make([]byte, readBufferSize)
	if _, err := io.CopyBuffer(hash, file, buf); err != nil {
		return "", fmt.Errorf("failed to compute hash: %w", err)
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// ComputeSHA1 computes the SHA1 hash of a file and returns it as a lowercase hex string
func ComputeSHA1(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hash := sha1.New()
	buffer := make([]byte, readBufferSize)

	if _, err := io.CopyBuffer(hash, file, buffer); err != nil {
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
