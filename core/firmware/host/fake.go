package host

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"testing/fstest"
)

type Fake struct {
	Files fstest.MapFS
	Env   map[string]string
	Exe   string
}

func (f *Fake) Getenv(k string) string {
	return f.Env[k]
}

func (f *Fake) key(p string) (string, error) {
	if !path.IsAbs(p) {
		return "", fmt.Errorf("host: path %q is not absolute", p)
	}
	k := strings.TrimPrefix(path.Clean(p), "/")
	if k == "" {
		k = "."
	}
	return k, nil
}

func (f *Fake) ReadFile(p string) ([]byte, error) {
	k, err := f.key(p)
	if err != nil {
		return nil, err
	}
	return f.Files.ReadFile(k)
}

func (f *Fake) Stat(p string) (fs.FileInfo, error) {
	k, err := f.key(p)
	if err != nil {
		return nil, err
	}
	return f.Files.Stat(k)
}

func (f *Fake) ReadDir(p string) ([]fs.DirEntry, error) {
	k, err := f.key(p)
	if err != nil {
		return nil, err
	}
	return f.Files.ReadDir(k)
}

func (f *Fake) Executable() (string, error) {
	if f.Exe == "" {
		return "", errors.New("host: no executable set")
	}
	return f.Exe, nil
}
