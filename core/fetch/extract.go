package fetch

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/bodgit/sevenzip"

	"github.com/rommapp/grout/core/firmware/host"
)

var ErrUnsafeArchive = errors.New("archive is unsafe or corrupt")

type Limits struct {
	MaxBytes   int64
	MaxEntries int
}

func DefaultLimits() Limits {
	return Limits{MaxBytes: 64 << 30, MaxEntries: 100000}
}

const ctxCheckBytes = 1 << 20

var (
	zipMagic      = []byte("PK\x03\x04")
	zipEmptyMagic = []byte("PK\x05\x06")
	sevenZipMagic = []byte("7z\xBC\xAF\x27\x1C")
)

type entry struct {
	name  string
	mode  fs.FileMode
	drain bool
	open  func() (io.ReadCloser, error)
}

type nodeKind int

const (
	nodeDir nodeKind = iota + 1
	nodeFile
)

func Extract(ctx context.Context, w host.Writable, archive, destDir string, lim Limits) ([]string, error) {
	real, err := w.Real(archive)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(real)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	entries, err := listEntries(f, st.Size(), lim.MaxEntries)
	if err != nil {
		return nil, err
	}
	x := &extractor{ctx: ctx, w: w, destDir: destDir, lim: lim, buf: make([]byte, 32<<10), seen: map[string]nodeKind{}}
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := x.entry(e); err != nil {
			return nil, err
		}
	}
	return x.files, nil
}

func tooMany(n, max int) error {
	if n > max {
		return fmt.Errorf("%w: more than %d entries", ErrUnsafeArchive, max)
	}
	return nil
}

func listEntries(f *os.File, size int64, maxEntries int) ([]entry, error) {
	head := make([]byte, len(sevenZipMagic))
	n, err := f.ReadAt(head, 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	head = head[:n]
	switch {
	case bytes.HasPrefix(head, zipMagic), bytes.HasPrefix(head, zipEmptyMagic):
		r, err := zip.NewReader(f, size)
		if err != nil && !(errors.Is(err, zip.ErrInsecurePath) && r != nil) {
			return nil, fmt.Errorf("%w: %w", ErrUnsafeArchive, err)
		}
		if err := tooMany(len(r.File), maxEntries); err != nil {
			return nil, err
		}
		out := make([]entry, 0, len(r.File))
		for _, zf := range r.File {
			mode := zf.Mode()
			if strings.HasSuffix(zf.Name, "/") {
				mode = mode&^fs.ModeType | fs.ModeDir
			}
			out = append(out, entry{name: zf.Name, mode: mode, open: zf.Open})
		}
		return out, nil
	case bytes.HasPrefix(head, sevenZipMagic):
		r, err := sevenzip.NewReader(f, size)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrUnsafeArchive, err)
		}
		if err := tooMany(len(r.File), maxEntries); err != nil {
			return nil, err
		}
		out := make([]entry, 0, len(r.File))
		for _, sf := range r.File {
			out = append(out, entry{name: sf.Name, mode: sf.FileInfo().Mode(), drain: true, open: sf.Open})
		}
		return out, nil
	}
	return nil, fmt.Errorf("%w: unrecognized format", ErrUnsafeArchive)
}

type extractor struct {
	ctx     context.Context
	w       host.Writable
	destDir string
	lim     Limits
	written int64
	files   []string
	buf     []byte
	seen    map[string]nodeKind
}

func safeName(raw string) (string, error) {
	if strings.ContainsRune(raw, 0) {
		return "", fmt.Errorf("%w: NUL in entry %q", ErrUnsafeArchive, raw)
	}
	n := strings.ReplaceAll(raw, "\\", "/")
	if strings.HasPrefix(n, "/") {
		return "", fmt.Errorf("%w: absolute entry %q", ErrUnsafeArchive, raw)
	}
	if len(n) >= 2 && n[1] == ':' && ((n[0] >= 'a' && n[0] <= 'z') || (n[0] >= 'A' && n[0] <= 'Z')) {
		return "", fmt.Errorf("%w: drive entry %q", ErrUnsafeArchive, raw)
	}
	c := path.Clean(n)
	if c == ".." || strings.HasPrefix(c, "../") {
		return "", fmt.Errorf("%w: traversal entry %q", ErrUnsafeArchive, raw)
	}
	return c, nil
}

func (x *extractor) entry(e entry) error {
	name, err := safeName(e.name)
	if err != nil {
		return err
	}
	if e.mode&fs.ModeType&^fs.ModeDir != 0 {
		if e.drain {
			return x.drain(e, name)
		}
		return nil
	}
	if name == "." {
		return nil
	}
	if err := x.claim(name, e.mode.IsDir()); err != nil {
		return err
	}
	real, err := x.w.Real(path.Join(x.destDir, name))
	if err != nil {
		return err
	}
	if e.mode.IsDir() {
		return os.MkdirAll(real, 0755)
	}
	if err := os.MkdirAll(filepath.Dir(real), 0755); err != nil {
		return err
	}
	src, err := e.open()
	if err != nil {
		return fmt.Errorf("%w: %s: %w", ErrUnsafeArchive, name, err)
	}
	defer src.Close()
	dst, err := os.OpenFile(real, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	if err := x.copy(dst, src, name); err != nil {
		dst.Close()
		return err
	}
	if err := dst.Close(); err != nil {
		return err
	}
	x.files = append(x.files, name)
	return nil
}

func (x *extractor) claim(name string, dir bool) error {
	key := strings.ToLower(name)
	parts := strings.Split(key, "/")
	for i := 1; i < len(parts); i++ {
		p := strings.Join(parts[:i], "/")
		if x.seen[p] == nodeFile {
			return fmt.Errorf("%w: %q is under a file", ErrUnsafeArchive, name)
		}
		x.seen[p] = nodeDir
	}
	switch prev := x.seen[key]; {
	case prev == nodeFile:
		return fmt.Errorf("%w: duplicate entry %q", ErrUnsafeArchive, name)
	case prev == nodeDir && !dir:
		return fmt.Errorf("%w: file %q conflicts with a directory", ErrUnsafeArchive, name)
	}
	if dir {
		x.seen[key] = nodeDir
	} else {
		x.seen[key] = nodeFile
	}
	return nil
}

func (x *extractor) drain(e entry, name string) error {
	src, err := e.open()
	if err != nil {
		return fmt.Errorf("%w: %s: %w", ErrUnsafeArchive, name, err)
	}
	defer src.Close()
	return x.copy(io.Discard, src, name)
}

func (x *extractor) copy(dst io.Writer, src io.Reader, name string) error {
	var since int64
	for {
		n, rerr := src.Read(x.buf)
		if n > 0 {
			if x.written+int64(n) > x.lim.MaxBytes {
				return fmt.Errorf("%w: more than %d bytes", ErrUnsafeArchive, x.lim.MaxBytes)
			}
			if _, err := dst.Write(x.buf[:n]); err != nil {
				return err
			}
			x.written += int64(n)
			since += int64(n)
			if since >= ctxCheckBytes {
				since = 0
				if err := x.ctx.Err(); err != nil {
					return err
				}
			}
		}
		if rerr == io.EOF {
			return nil
		}
		if rerr != nil {
			return fmt.Errorf("%w: %s: %w", ErrUnsafeArchive, name, rerr)
		}
	}
}
