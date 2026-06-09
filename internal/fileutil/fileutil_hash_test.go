package fileutil

import (
	"crypto/md5"
	"encoding/hex"
	"os"
	"path/filepath"
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
