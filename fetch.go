package grout

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/rommapp/grout/core/config"
	"github.com/rommapp/grout/core/fetch"
	"github.com/rommapp/grout/core/firmware"
	"github.com/rommapp/grout/core/firmware/host"
	"github.com/rommapp/grout/core/gamelist"
	"github.com/rommapp/grout/core/library"
	"github.com/rommapp/grout/core/romm"
)

type FetchGameResult struct {
	fetch.GameResult
	Art      fetch.ArtResult `json:"art"`
	Gamelist gamelist.Result `json:"gamelist"`
}

type FetchArtResult struct {
	fetch.ArtResult
	Gamelist gamelist.Result `json:"gamelist"`
}

var errReadOnly = errors.New("host is read-only")

type fetchSession struct {
	cfg     *config.Config
	api     *romm.Client
	content *romm.Client
	w       host.Writable
	lib     *library.Library
}

func gameKey(id int) string {
	return fmt.Sprintf("game:%d", id)
}

func biosKey(id int) string {
	return fmt.Sprintf("bios:%d", id)
}

func requireToken(cfg *config.Config) error {
	h, err := requireHost(cfg)
	if err != nil {
		return err
	}
	if h.Token == "" {
		return newError(CodeNotConfigured, errNoToken)
	}
	return nil
}

func (g *Grout) prepareFetch(ctx context.Context) (fetchSession, error) {
	cfg, src := g.settings()
	if err := requireToken(cfg); err != nil {
		return fetchSession{}, err
	}
	w, ok := g.fw.Host.(host.Writable)
	if !ok {
		return fetchSession{}, newError(CodeInternal, errReadOnly)
	}
	_, client, err := connect(ctx, cfg, src)
	if err != nil {
		return fetchSession{}, err
	}
	return fetchSession{
		cfg:     cfg,
		api:     client,
		content: client.WithContentTimeout(cfg.DownloadTimeout.Duration()),
		w:       w,
		lib:     g.library(cfg, client),
	}, nil
}

func (g *Grout) FetchGame(ctx context.Context, gameID, fileID int, progress func(library.Progress)) (FetchGameResult, error) {
	ctx, end, err := g.beginFetch(ctx, gameKey(gameID))
	if err != nil {
		return FetchGameResult{}, err
	}
	defer end()
	s, err := g.prepareFetch(ctx)
	if err != nil {
		return FetchGameResult{}, err
	}
	info, err := s.lib.FetchInfo(ctx, gameID)
	if err != nil {
		return FetchGameResult{}, libraryError(ctx, err)
	}
	report := g.progress(progress)
	res, err := fetch.Game(ctx, fetch.GameInput{
		Info:     info,
		FileID:   fileID,
		Unzip:    s.cfg.UnzipDownloads,
		Client:   s.content,
		Firmware: g.fw,
		Host:     s.w,
	}, report)
	if err != nil {
		return FetchGameResult{}, fetchError(ctx, err)
	}
	out := FetchGameResult{GameResult: res}
	err = s.lib.RecordFetch(context.WithoutCancel(ctx), library.Download{
		GameID:     gameID,
		PlatformID: info.Platform.ID,
		FileID:     res.FileID,
		LaunchPath: res.LaunchPath,
		Placed:     res.Placed,
	})
	if err != nil {
		out.Gamelist = emptyGamelist()
		return out, classify(ctx, err, CodeStorage)
	}
	g.fire(ctx, s.lib, firmware.CondGamesChanged)
	folder := !slices.Contains(res.Placed, res.LaunchPath)
	var identified []library.Identity
	if ctx.Err() == nil {
		var idErr error
		identified, idErr = s.lib.Identities(ctx, gameID)
		if idErr != nil && ctx.Err() == nil {
			g.log.Warn("identify failed", "game", gameID, "err", idErr)
		}
	}
	if ctx.Err() != nil {
		out.Art = g.skippedArt(s, info.Platform, res.RomDir)
		out.Gamelist = emptyGamelist()
		return out, nil
	}
	out.Art = g.art(ctx, s, info.Platform, gameID, res.RomDir, artStem(res.LaunchPath, folder), false, report)
	out.Gamelist = g.writeEntry(ctx, s.lib, info.Platform, gameID, res.RomDir, res.LaunchPath, folder)
	g.log.Info("game fetched", "game", gameID, "launch", res.LaunchPath, "art", len(out.Art.Written), "art_failed", len(out.Art.Failed), "gamelist", len(out.Gamelist.Written), "gamelist_failed", len(out.Gamelist.Failed), "identified", len(identified))
	return out, nil
}

func (g *Grout) FetchArt(ctx context.Context, gameID int, progress func(library.Progress)) (FetchArtResult, error) {
	ctx, end, err := g.beginFetch(ctx, gameKey(gameID))
	if err != nil {
		return FetchArtResult{}, err
	}
	defer end()
	s, err := g.prepareFetch(ctx)
	if err != nil {
		return FetchArtResult{}, err
	}
	launch, ok, err := s.lib.LaunchFile(ctx, gameID)
	if err != nil {
		return FetchArtResult{}, libraryError(ctx, err)
	}
	if !ok {
		return FetchArtResult{}, newError(CodeBadRequest, fmt.Errorf("game %d is not on the card", gameID))
	}
	stat, err := s.w.Stat(launch)
	if errors.Is(err, fs.ErrNotExist) {
		return FetchArtResult{}, newError(CodeBadRequest, fmt.Errorf("game %d is not on the card", gameID))
	}
	if err != nil {
		return FetchArtResult{}, classify(ctx, err, CodeStorage)
	}
	info, err := s.lib.FetchInfo(ctx, gameID)
	if err != nil {
		return FetchArtResult{}, libraryError(ctx, err)
	}
	dir, stem := artTarget(info.Platform.RomDirs, launch, stat.IsDir())
	out := FetchArtResult{ArtResult: g.art(ctx, s, info.Platform, gameID, dir, stem, true, g.progress(progress))}
	out.Gamelist = g.writeEntry(ctx, s.lib, info.Platform, gameID, dir, launch, stat.IsDir())
	return out, finish(ctx, nil)
}

func (g *Grout) skippedArt(s fetchSession, p library.Platform, romDir string) fetch.ArtResult {
	return fetch.Skipped(fetch.ArtInput{Platform: p, RomDir: romDir, Settings: *s.cfg, Firmware: g.fw})
}

func (g *Grout) art(ctx context.Context, s fetchSession, p library.Platform, gameID int, romDir, stem string, onlyMissing bool, report func(library.Progress)) fetch.ArtResult {
	if len(fetch.Wanted(*s.cfg)) == 0 {
		return g.skippedArt(s, p, romDir)
	}
	detail, err := s.api.RomDetail(ctx, gameID)
	if err != nil {
		g.log.Warn("art details unavailable", "game", gameID, "err", err)
		return g.skippedArt(s, p, romDir)
	}
	return fetch.Art(ctx, fetch.ArtInput{
		Detail:      detail,
		Platform:    p,
		RomDir:      romDir,
		Stem:        stem,
		Settings:    *s.cfg,
		MaxSize:     g.artMax,
		OnlyMissing: onlyMissing,
		Client:      s.content,
		Firmware:    g.fw,
		Host:        s.w,
	}, report)
}

func artTarget(romDirs []string, launch string, folder bool) (string, string) {
	dir, ok := library.RomDirOf(romDirs, launch)
	if !ok {
		dir = path.Dir(launch)
	}
	return dir, artStem(launch, folder)
}

func artStem(launch string, folder bool) string {
	base := path.Base(launch)
	if folder {
		return base
	}
	return strings.TrimSuffix(base, path.Ext(base))
}

func (g *Grout) FetchBios(ctx context.Context, platformID int, progress func(library.Progress)) ([]fetch.BiosFile, error) {
	ctx, end, err := g.beginFetch(ctx, biosKey(platformID))
	if err != nil {
		return nil, err
	}
	defer end()
	s, err := g.prepareFetch(ctx)
	if err != nil {
		return nil, err
	}
	p, err := platformByID(ctx, s.lib, platformID)
	if err != nil {
		return nil, err
	}
	entries, err := s.api.PlatformFirmware(ctx, platformID)
	if err != nil {
		return nil, classify(ctx, err, CodeServerUnreachable)
	}
	files, err := fetch.Bios(ctx, fetch.BiosInput{
		PlatformSlug: p.Slug,
		Entries:      entries,
		Client:       s.content,
		Firmware:     g.fw,
		Host:         s.w,
	}, g.progress(progress))
	if err != nil {
		return files, fetchError(ctx, err)
	}
	return files, nil
}

func (g *Grout) LibraryBios(ctx context.Context, platformID int) ([]fetch.BiosFile, error) {
	ctx, end, err := g.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer end()
	cfg, src := g.settings()
	if err := requireToken(cfg); err != nil {
		return nil, err
	}
	_, client, err := connect(ctx, cfg, src)
	if err != nil {
		return nil, err
	}
	p, err := platformByID(ctx, g.library(cfg, client), platformID)
	if err != nil {
		return nil, err
	}
	entries, err := client.PlatformFirmware(ctx, platformID)
	if err != nil {
		return nil, classify(ctx, err, CodeServerUnreachable)
	}
	return fetch.BiosStatus(g.fw.Host, g.fw, p.Slug, entries), nil
}

func (g *Grout) GameFiles(ctx context.Context, gameID int) ([]library.GameFile, error) {
	ctx, end, err := g.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer end()
	files, err := g.snapshot().GameFiles(ctx, gameID)
	if err != nil {
		return nil, libraryError(ctx, err)
	}
	return list(ctx, files, nil)
}

func platformByID(ctx context.Context, l *library.Library, id int) (library.Platform, error) {
	all, err := l.AllPlatforms(ctx)
	if err != nil {
		return library.Platform{}, classify(ctx, err, CodeStorage)
	}
	for _, p := range all {
		if p.ID == id {
			return p, nil
		}
	}
	return library.Platform{}, newError(CodeBadRequest, fmt.Errorf("unknown platform %d", id))
}

func libraryError(ctx context.Context, err error) error {
	if errors.Is(err, library.ErrUnknownGame) {
		return newError(CodeBadRequest, err)
	}
	return classify(ctx, err, CodeStorage)
}

func fetchError(ctx context.Context, err error) error {
	var e *Error
	switch {
	case err == nil:
		return nil
	case errors.As(err, &e):
		return err
	case ctx.Err() != nil, errors.Is(err, context.Canceled):
		return newError(CodeCancelled, err)
	case errors.Is(err, fetch.ErrNoRomDir), errors.Is(err, fetch.ErrBadFile),
		errors.Is(err, fetch.ErrFileForMulti), errors.Is(err, fetch.ErrNoFiles):
		return newError(CodeBadRequest, err)
	case errors.Is(err, fetch.ErrUnsafeArchive):
		return newError(CodeBadArchive, err)
	case romm.IsNotFound(err):
		return newError(CodeMissingOnServer, err)
	case errors.Is(err, io.ErrUnexpectedEOF):
		return newError(CodeServerUnreachable, err)
	}
	return classify(ctx, err, CodeInternal)
}
