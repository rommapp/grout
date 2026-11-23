package fetch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rommapp/grout/core/firmware"
	"github.com/rommapp/grout/core/firmware/host"
	"github.com/rommapp/grout/core/library"
	"github.com/rommapp/grout/core/romm"
)

type Progress = library.Progress

type GameInput struct {
	Info     library.FetchInfo
	FileID   int
	Unzip    bool
	Client   *romm.Client
	Firmware *firmware.Firmware
	Host     host.Writable
	Limits   Limits
	Clock    func() time.Time
}

type GameResult struct {
	GameID     int      `json:"game_id"`
	FileID     int      `json:"file_id,omitempty"`
	RomDir     string   `json:"rom_dir"`
	LaunchPath string   `json:"launch_path"`
	Placed     []string `json:"placed"`
}

var (
	ErrNoRomDir     = errors.New("platform has no ROM folder on this firmware")
	ErrBadFile      = errors.New("file is not one of this game's playable files")
	ErrFileForMulti = errors.New("a multi-file game downloads whole")
	ErrNoFiles      = errors.New("game has no files; refresh the library")
)

func PickRomDir(h host.Host, dirs []string) (string, bool) {
	if len(dirs) == 0 {
		return "", false
	}
	for _, d := range dirs {
		if info, err := h.Stat(d); err == nil && info.IsDir() {
			return d, true
		}
	}
	return dirs[0], true
}

type gameFetch struct {
	in     GameInput
	romDir string
	limits Limits
	t      *transfer
	staged []string
}

func Game(ctx context.Context, in GameInput, progress func(Progress)) (GameResult, error) {
	if progress == nil {
		progress = func(Progress) {}
	}
	clock := in.Clock
	if clock == nil {
		clock = time.Now
	}
	limits := in.Limits
	if limits == (Limits{}) {
		limits = DefaultLimits()
	}
	g := &gameFetch{in: in, limits: limits, t: &transfer{client: in.Client, w: in.Host, clock: clock, progress: progress, stage: library.StageDownload, count: 1}}
	var res GameResult
	var err error
	if in.Info.Kind == library.KindMulti {
		res, err = g.multi(ctx)
	} else {
		res, err = g.single(ctx)
	}
	for _, s := range g.staged {
		Discard(in.Host, s)
	}
	if err != nil {
		return GameResult{}, err
	}
	sort.Strings(res.Placed)
	return res, nil
}

func safeElem(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, "/\\") && path.Base(name) == name
}

func (g *gameFetch) pickRomDir() error {
	dir, ok := PickRomDir(g.in.Host, g.in.Info.Platform.RomDirs)
	if !ok {
		return fmt.Errorf("%s: %w", g.in.Info.Platform.Slug, ErrNoRomDir)
	}
	g.romDir = dir
	return nil
}

func (g *gameFetch) pickFile() (library.GameFile, error) {
	files := g.in.Info.Files
	if len(files) == 0 {
		return library.GameFile{}, fmt.Errorf("game %d: %w", g.in.Info.GameID, ErrNoFiles)
	}
	file := files[0]
	if g.in.FileID != 0 {
		found := false
		for _, f := range files {
			if f.ID == g.in.FileID {
				file, found = f, true
				break
			}
		}
		if !found {
			return library.GameFile{}, fmt.Errorf("file %d of game %d: %w", g.in.FileID, g.in.Info.GameID, ErrBadFile)
		}
	}
	if !safeElem(file.FileName) {
		return library.GameFile{}, fmt.Errorf("file name %q: %w", file.FileName, ErrBadFile)
	}
	return file, nil
}

func (g *gameFetch) single(ctx context.Context) (GameResult, error) {
	file, err := g.pickFile()
	if err != nil {
		return GameResult{}, err
	}
	if err := g.pickRomDir(); err != nil {
		return GameResult{}, err
	}
	res := GameResult{GameID: g.in.Info.GameID, FileID: file.ID, RomDir: g.romDir}
	dest := path.Join(g.romDir, file.FileName)
	query := url.Values{"purpose": {"download"}, "file_ids": {strconv.Itoa(file.ID)}}
	archive, err := g.t.download(ctx, romm.RomContentPath(g.in.Info.GameID, file.FileName), query, dest, file.Size)
	if err != nil {
		return GameResult{}, err
	}
	g.staged = append(g.staged, archive)
	if g.in.Unzip {
		isArchive, err := g.isArchive(archive)
		if err != nil {
			return GameResult{}, err
		}
		if isArchive {
			return g.singleExtract(ctx, res, archive)
		}
	}
	if err := ctx.Err(); err != nil {
		return GameResult{}, err
	}
	if err := Commit(g.in.Host, archive, dest); err != nil {
		return GameResult{}, err
	}
	res.LaunchPath = dest
	res.Placed = []string{dest}
	return res, nil
}

func (g *gameFetch) singleExtract(ctx context.Context, res GameResult, archive string) (GameResult, error) {
	dir, files, err := g.extract(ctx, archive)
	if err != nil {
		return GameResult{}, err
	}
	if len(files) == 0 {
		return GameResult{}, fmt.Errorf("%w: archive holds no files", ErrUnsafeArchive)
	}
	launch := files[0]
	for _, f := range files {
		if !strings.Contains(f, "/") && strings.EqualFold(path.Ext(f), ".m3u") {
			launch = f
			break
		}
	}
	if err := ctx.Err(); err != nil {
		return GameResult{}, err
	}
	placed, err := MoveMerge(g.in.Host, dir, g.romDir)
	if err != nil {
		return GameResult{}, err
	}
	res.LaunchPath = path.Join(g.romDir, launch)
	res.Placed = placed
	return res, nil
}

func (g *gameFetch) multi(ctx context.Context) (GameResult, error) {
	if g.in.FileID != 0 {
		return GameResult{}, fmt.Errorf("game %d: %w", g.in.Info.GameID, ErrFileForMulti)
	}
	fsName := g.in.Info.FSName
	if !safeElem(fsName) {
		return GameResult{}, fmt.Errorf("folder name %q: %w", fsName, ErrBadFile)
	}
	if err := g.pickRomDir(); err != nil {
		return GameResult{}, err
	}
	res := GameResult{GameID: g.in.Info.GameID, RomDir: g.romDir}
	query := url.Values{"purpose": {"download"}}
	archive, err := g.t.download(ctx, romm.RomContentPath(g.in.Info.GameID, fsName), query, path.Join(g.romDir, fsName+".zip"), 0)
	if err != nil {
		return GameResult{}, err
	}
	g.staged = append(g.staged, archive)
	dir, files, err := g.extract(ctx, archive)
	if err != nil {
		return GameResult{}, err
	}
	if len(files) == 0 {
		return GameResult{}, fmt.Errorf("%w: archive holds no files", ErrUnsafeArchive)
	}
	m3uName := ""
	for _, f := range files {
		if strings.EqualFold(f, fsName+".m3u") {
			m3uName = f
			break
		}
	}
	var m3u []byte
	if m3uName != "" {
		m3u, err = g.in.Host.ReadFile(path.Join(dir, m3uName))
		if err != nil {
			return GameResult{}, err
		}
	}
	layout := firmware.MultiLayout{Folder: fsName, Playlist: m3u}
	if g.in.Firmware != nil {
		layout = g.in.Firmware.MultiLayout(fsName, m3u)
	}
	if !safeElem(layout.Folder) {
		return GameResult{}, fmt.Errorf("folder name %q: %w", layout.Folder, ErrBadFile)
	}
	folder := path.Join(g.romDir, layout.Folder)
	res.LaunchPath = folder
	besideDest := path.Join(g.romDir, fsName+".m3u")
	besideStaged := ""
	switch {
	case layout.Beside:
		if m3uName != "" {
			if err := g.remove(path.Join(dir, m3uName)); err != nil {
				return GameResult{}, err
			}
		}
		if layout.Playlist != nil {
			besideStaged, err = g.stagePlaylist(besideDest, layout.Playlist)
			if err != nil {
				return GameResult{}, err
			}
			res.LaunchPath = besideDest
		}
	case layout.Playlist != nil:
		if m3uName == "" {
			m3uName = fsName + ".m3u"
		}
		if !bytes.Equal(layout.Playlist, m3u) {
			if err := g.overwrite(path.Join(dir, m3uName), layout.Playlist); err != nil {
				return GameResult{}, err
			}
		}
		res.LaunchPath = path.Join(folder, m3uName)
	}
	if err := ctx.Err(); err != nil {
		return GameResult{}, err
	}
	placed, err := MoveMerge(g.in.Host, dir, folder)
	if err != nil {
		return GameResult{}, err
	}
	if besideStaged != "" {
		if err := Commit(g.in.Host, besideStaged, besideDest); err != nil {
			return GameResult{}, err
		}
		placed = append(placed, besideDest)
	}
	res.Placed = placed
	return res, nil
}

func (g *gameFetch) extract(ctx context.Context, archive string) (string, []string, error) {
	dir, err := StageDir(g.in.Host, g.romDir)
	if err != nil {
		return "", nil, err
	}
	g.staged = append(g.staged, dir)
	g.t.progress(Progress{Stage: library.StageExtract, Done: 0, Total: 1})
	files, err := Extract(ctx, g.in.Host, archive, dir, g.limits)
	if err != nil {
		return "", nil, err
	}
	g.t.progress(Progress{Stage: library.StageExtract, Done: 1, Total: 1})
	return dir, files, nil
}

func (g *gameFetch) isArchive(staged string) (bool, error) {
	real, err := g.in.Host.Real(staged)
	if err != nil {
		return false, err
	}
	f, err := os.Open(real)
	if err != nil {
		return false, err
	}
	defer f.Close()
	head := make([]byte, len(sevenZipMagic))
	n, err := io.ReadFull(f, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return false, err
	}
	head = head[:n]
	return bytes.HasPrefix(head, zipMagic) || bytes.HasPrefix(head, zipEmptyMagic) || bytes.HasPrefix(head, sevenZipMagic), nil
}

func (g *gameFetch) stagePlaylist(dest string, data []byte) (string, error) {
	f, staged, err := StageFile(g.in.Host, dest)
	if err != nil {
		return "", err
	}
	g.staged = append(g.staged, staged)
	if _, err := f.Write(data); err != nil {
		f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return staged, nil
}

func (g *gameFetch) remove(p string) error {
	real, err := g.in.Host.Real(p)
	if err != nil {
		return err
	}
	return os.Remove(real)
}

func (g *gameFetch) overwrite(p string, data []byte) error {
	real, err := g.in.Host.Real(p)
	if err != nil {
		return err
	}
	return os.WriteFile(real, data, 0644)
}
