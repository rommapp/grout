package fileutil

import (
	"archive/zip"
	"crypto/md5"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestComputeMD5(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f.bin")
	if err := os.WriteFile(p, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := ComputeMD5(p)
	if err != nil {
		t.Fatal(err)
	}
	want := "5d41402abc4b2a76b9719d911017c592" // md5("hello")
	if got != want {
		t.Errorf("ComputeMD5 = %s, want %s", got, want)
	}
	// sanity: matches stdlib
	sum := md5.Sum([]byte("hello"))
	if got != hex.EncodeToString(sum[:]) {
		t.Errorf("ComputeMD5 disagrees with stdlib")
	}
}

// helper: reference composite hash from an ordered (name -> content) map,
// computed independently of the zip-reading code path.
func refComposite(t *testing.T, entries map[string]string) string {
	t.Helper()
	names := make([]string, 0, len(entries))
	for n := range entries {
		names = append(names, n)
	}
	sort.Strings(names)
	var lines []string
	for _, n := range names {
		sum := md5.Sum([]byte(entries[n]))
		lines = append(lines, n+":"+hex.EncodeToString(sum[:]))
	}
	combined := strings.Join(lines, "\n")
	final := md5.Sum([]byte(combined))
	return hex.EncodeToString(final[:])
}

func TestComputeCompositeZipHash_Structure(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "a.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	// write in non-sorted order to prove sorting happens
	for _, name := range []string{"b.txt", "a.txt"} {
		fw, _ := w.Create(name)
		fw.Write([]byte(name + "-content"))
	}
	// a directory entry must be ignored by the hash
	w.Create("d/")
	w.Close()
	f.Close()

	got, err := ComputeCompositeZipHash(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	want := refComposite(t, map[string]string{
		"a.txt": "a.txt-content",
		"b.txt": "b.txt-content",
	})
	if got != want {
		t.Errorf("ComputeCompositeZipHash = %s, want %s", got, want)
	}
}

func TestDirHashMatchesZipHash(t *testing.T) {
	root := t.TempDir()
	saveDir := filepath.Join(root, "ULUS10064DATA00")
	if err := os.MkdirAll(filepath.Join(saveDir, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(saveDir, "SETTINGS.BIN"), []byte("settings"), 0644)
	os.WriteFile(filepath.Join(saveDir, "sub", "DATA.DAT"), []byte("payload"), 0644)
	os.WriteFile(filepath.Join(saveDir, ".hidden"), []byte("nope"), 0644) // must be skipped

	// Build a zip the same way addDirToZip does (folder name as top-level prefix).
	zipPath := filepath.Join(root, "out.zip")
	zf, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(zf)
	parent := filepath.Dir(saveDir)
	filepath.Walk(saveDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(parent, path)
		if strings.HasPrefix(filepath.Base(rel), ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			zw.Create(rel + "/")
			return nil
		}
		w, _ := zw.Create(rel)
		b, _ := os.ReadFile(path)
		w.Write(b)
		return nil
	})
	zw.Close()
	zf.Close()

	zipHash, err := ComputeCompositeZipHash(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	dirHash, err := ComputeDirsCompositeHash([]string{saveDir})
	if err != nil {
		t.Fatal(err)
	}
	if zipHash != dirHash {
		t.Errorf("dir hash %s != zip hash %s", dirHash, zipHash)
	}
}
