package gamelist

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sync"

	"github.com/rommapp/grout/core/fetch"
	"github.com/rommapp/grout/core/firmware"
	"github.com/rommapp/grout/core/firmware/host"
	"github.com/rommapp/grout/core/gamelist/emulationstation"
	"github.com/rommapp/grout/core/gamelist/entry"
	"github.com/rommapp/grout/core/gamelist/miyoo"
	"github.com/rommapp/grout/core/gamelist/muostext"
)

var ErrNoTextDir = errors.New("firmware has no text folder for this platform")

type Failure struct {
	GameID  int    `json:"game_id,omitempty"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message"`
}

type Result struct {
	Written []string  `json:"written"`
	Failed  []Failure `json:"failed"`
}

func newResult() Result {
	return Result{Written: []string{}, Failed: []Failure{}}
}

type Writer struct {
	fw    *firmware.Firmware
	w     host.Writable
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func NewWriter(fw *firmware.Firmware, w host.Writable) *Writer {
	return &Writer{fw: fw, w: w, locks: map[string]*sync.Mutex{}}
}

func (w *Writer) lock(file string) *sync.Mutex {
	w.mu.Lock()
	defer w.mu.Unlock()
	l, ok := w.locks[file]
	if !ok {
		l = &sync.Mutex{}
		w.locks[file] = l
	}
	return l
}

type target struct {
	file    string
	gameIDs []int
	entries []entry.Entry
}

func (w *Writer) Write(ctx context.Context, games []Game, omitRegion bool) Result {
	res := newResult()
	format := w.fw.GamelistFormat()
	if format == firmware.FormatNone {
		return res
	}
	var order []string
	targets := map[string]*target{}
	for _, g := range games {
		e, err := Build(w.fw, w.w, g, omitRegion)
		if err != nil {
			res.Failed = append(res.Failed, Failure{GameID: g.Meta.GameID, Path: g.LaunchPath, Message: err.Error()})
			continue
		}
		file, err := w.targetFile(format, g)
		if err != nil {
			res.Failed = append(res.Failed, Failure{GameID: g.Meta.GameID, Path: g.LaunchPath, Message: err.Error()})
			continue
		}
		t, ok := targets[file]
		if !ok {
			t = &target{file: file}
			targets[file] = t
			order = append(order, file)
		}
		t.gameIDs = append(t.gameIDs, g.Meta.GameID)
		t.entries = append(t.entries, e)
	}
	for _, file := range order {
		t := targets[file]
		written, err := w.update(ctx, file, func(existing []byte) ([]byte, bool, error) {
			return render(format, existing, t.entries)
		})
		if err != nil {
			for _, id := range t.gameIDs {
				res.Failed = append(res.Failed, Failure{GameID: id, Path: file, Message: err.Error()})
			}
			continue
		}
		if written {
			res.Written = append(res.Written, file)
		}
	}
	return res
}

func (w *Writer) targetFile(format firmware.GamelistFormat, g Game) (string, error) {
	switch format {
	case firmware.FormatEmulationStation:
		return path.Join(g.RomDir, emulationstation.FileName), nil
	case firmware.FormatMiyoo:
		return path.Join(g.RomDir, miyoo.FileName), nil
	case firmware.FormatMuOSText:
		dir, ok := w.fw.GamelistTextDir(g.Platform.Slug, g.RomDir, g.Platform.Name)
		if !ok {
			return "", ErrNoTextDir
		}
		return path.Join(dir, stem(g.LaunchPath)+".txt"), nil
	}
	return "", fmt.Errorf("unknown gamelist format %q", format)
}

func render(format firmware.GamelistFormat, existing []byte, entries []entry.Entry) ([]byte, bool, error) {
	switch format {
	case firmware.FormatEmulationStation:
		return emulationstation.Merge(existing, entries)
	case firmware.FormatMiyoo:
		return miyoo.Merge(existing, entries)
	case firmware.FormatMuOSText:
		out := muostext.Render(entries[len(entries)-1])
		return out, !bytes.Equal(out, existing), nil
	}
	return nil, false, fmt.Errorf("unknown gamelist format %q", format)
}

func (w *Writer) update(ctx context.Context, file string, merge func([]byte) ([]byte, bool, error)) (bool, error) {
	l := w.lock(path.Clean(file))
	l.Lock()
	defer l.Unlock()
	if err := ctx.Err(); err != nil {
		return false, err
	}
	existing, err := w.w.ReadFile(file)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, fmt.Errorf("reading %s: %w", file, err)
	}
	out, changed, err := merge(existing)
	if err != nil {
		return false, fmt.Errorf("%s: %w", file, err)
	}
	if !changed {
		return false, nil
	}
	f, staged, err := fetch.StageFile(w.w, file)
	if err != nil {
		return false, fmt.Errorf("writing %s: %w", file, err)
	}
	_, err = f.Write(out)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = fetch.Commit(w.w, staged, file)
	}
	if err != nil {
		fetch.Discard(w.w, staged)
		return false, fmt.Errorf("writing %s: %w", file, err)
	}
	return true, nil
}
