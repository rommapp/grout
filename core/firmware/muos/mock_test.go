package muos

import (
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"grout/core/firmware/host"
)

func mockHost(t *testing.T) *host.Fake {
	t.Helper()
	files := fstest.MapFS{}
	err := fs.WalkDir(os.DirFS("mock"), ".", func(p string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return err
		}
		data, err := os.ReadFile("mock/" + p)
		if err != nil {
			return err
		}
		files[p] = &fstest.MapFile{Data: data}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return &host.Fake{Files: files}
}

func emptyHost() *host.Fake {
	return &host.Fake{Files: fstest.MapFS{}}
}

func put(h *host.Fake, p, content string) {
	h.Files[strings.TrimPrefix(p, "/")] = &fstest.MapFile{Data: []byte(content)}
}

func mkdir(h *host.Fake, p string) {
	h.Files[strings.TrimPrefix(p, "/")] = &fstest.MapFile{Mode: fs.ModeDir | 0o755}
}

func remove(h *host.Fake, p string) {
	delete(h.Files, strings.TrimPrefix(p, "/"))
}
