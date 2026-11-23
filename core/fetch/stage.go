package fetch

import (
	"crypto/rand"
	"encoding/hex"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rommapp/grout/core/firmware/host"
)

const StagePrefix = ".grout-"

func stageName() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return StagePrefix + hex.EncodeToString(b[:]), nil
}

func StageFile(w host.Writable, dest string) (*os.File, string, error) {
	dir := path.Dir(dest)
	realDir, err := w.Real(dir)
	if err != nil {
		return nil, "", err
	}
	if err := os.MkdirAll(realDir, 0755); err != nil {
		return nil, "", err
	}
	name, err := stageName()
	if err != nil {
		return nil, "", err
	}
	staged := path.Join(dir, name+"-"+path.Base(dest))
	real, err := w.Real(staged)
	if err != nil {
		return nil, "", err
	}
	f, err := os.OpenFile(real, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return nil, "", err
	}
	return f, staged, nil
}

func StageDir(w host.Writable, destDir string) (string, error) {
	realDir, err := w.Real(destDir)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(realDir, 0755); err != nil {
		return "", err
	}
	name, err := stageName()
	if err != nil {
		return "", err
	}
	staged := path.Join(destDir, name)
	real, err := w.Real(staged)
	if err != nil {
		return "", err
	}
	if err := os.Mkdir(real, 0755); err != nil {
		return "", err
	}
	return staged, nil
}

func Commit(w host.Writable, staged, dest string) error {
	src, err := w.Real(staged)
	if err != nil {
		return err
	}
	dst, err := w.Real(dest)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	return os.Rename(src, dst)
}

func Discard(w host.Writable, staged string) {
	real, err := w.Real(staged)
	if err != nil {
		return
	}
	os.RemoveAll(real)
}

func MoveMerge(w host.Writable, srcDir, destDir string) ([]string, error) {
	realSrc, err := w.Real(srcDir)
	if err != nil {
		return nil, err
	}
	var placed []string
	err = filepath.WalkDir(realSrc, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(realSrc, p)
		if err != nil {
			return err
		}
		card := path.Join(destDir, filepath.ToSlash(rel))
		dst, err := w.Real(card)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return err
		}
		if err := os.Rename(p, dst); err != nil {
			return err
		}
		placed = append(placed, card)
		return nil
	})
	sort.Strings(placed)
	if err != nil {
		return placed, err
	}
	if err := os.RemoveAll(realSrc); err != nil {
		return placed, err
	}
	return placed, nil
}

func SweepStaging(w host.Writable, dir string) (int, error) {
	real, err := w.Real(dir)
	if err != nil {
		return 0, err
	}
	entries, err := os.ReadDir(real)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	removed := 0
	for _, e := range entries {
		full := filepath.Join(real, e.Name())
		if strings.HasPrefix(e.Name(), StagePrefix) {
			if err := os.RemoveAll(full); err != nil {
				return removed, err
			}
			removed++
			continue
		}
		if !e.IsDir() {
			continue
		}
		subs, err := os.ReadDir(full)
		if err != nil {
			continue
		}
		for _, s := range subs {
			if !strings.HasPrefix(s.Name(), StagePrefix) {
				continue
			}
			if err := os.RemoveAll(filepath.Join(full, s.Name())); err != nil {
				return removed, err
			}
			removed++
		}
	}
	return removed, nil
}
