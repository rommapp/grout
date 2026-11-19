package host

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type osHost struct {
	getenv func(string) string
	root   string
	roots  []string
}

func OS(getenv func(string) string) Host {
	h := &osHost{getenv: getenv, root: getenv("GROUT_ROOT")}
	if h.root != "" {
		if abs, err := filepath.Abs(h.root); err == nil {
			h.root = abs
		}
		h.root = filepath.Clean(h.root)
		h.roots = []string{h.root}
		if resolved, err := filepath.EvalSymlinks(h.root); err == nil && resolved != h.root {
			h.roots = append(h.roots, resolved)
		}
	}
	return h
}

func (h *osHost) Getenv(k string) string {
	return h.getenv(k)
}

func (h *osHost) resolve(p string) (string, error) {
	if h.root == "" {
		return p, nil
	}
	if !path.IsAbs(p) {
		return "", fmt.Errorf("host: path %q is not absolute", p)
	}
	return filepath.Join(h.root, filepath.FromSlash(path.Clean(p))), nil
}

func (h *osHost) ReadFile(p string) ([]byte, error) {
	r, err := h.resolve(p)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(r)
}

func (h *osHost) Stat(p string) (fs.FileInfo, error) {
	r, err := h.resolve(p)
	if err != nil {
		return nil, err
	}
	return os.Stat(r)
}

func (h *osHost) ReadDir(p string) ([]fs.DirEntry, error) {
	r, err := h.resolve(p)
	if err != nil {
		return nil, err
	}
	return os.ReadDir(r)
}

func (h *osHost) Executable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if h.root == "" {
		return exe, nil
	}
	return stripRoot(exe, h.roots), nil
}

func stripRoot(exe string, roots []string) string {
	for _, root := range roots {
		prefix := root + string(filepath.Separator)
		if strings.HasPrefix(exe, prefix) {
			return "/" + filepath.ToSlash(strings.TrimPrefix(exe, prefix))
		}
	}
	return exe
}
