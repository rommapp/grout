package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"time"

	"github.com/rommapp/grout"
	"github.com/rommapp/grout/core/config"
	"github.com/rommapp/grout/core/fetch"
	"github.com/rommapp/grout/core/library"
)

type handler func(ctx context.Context, s *Server, args []byte, emit func(Event)) (any, error)

type method struct {
	name string
	fn   handler
}

var methods = []method{
	{"status", status},
	{"settings.get", settingsGet},
	{"settings.set_host", settingsSetHost},
	{"settings.set_directory_mapping", settingsSetDirectoryMapping},
	{"library.refresh", libraryRefresh},
	{"library.scan", libraryScan},
	{"library.platforms", libraryPlatforms},
	{"library.games", libraryGames},
	{"library.filter_values", libraryFilterValues},
	{"library.dims", libraryDims},
	{"library.collections", libraryCollections},
	{"library.collection_platforms", libraryCollectionPlatforms},
	{"library.game_state", libraryGameState},
	{"auth.pair_start", authPairStart},
	{"auth.pair_wait", authPairWait},
	{"auth.exchange_code", authExchangeCode},
	{"auth.logout", authLogout},
	{"device.register", deviceRegister},
	{"settings.update", settingsUpdate},
	{"fetch.game", fetchGame},
	{"fetch.art", fetchArt},
	{"fetch.bios", fetchBios},
	{"library.bios", libraryBios},
	{"library.game_files", libraryGameFiles},
}

func Methods() []string {
	out := make([]string, 0, len(methods))
	for _, m := range methods {
		out = append(out, m.name)
	}
	return out
}

func (s *Server) lookup(name string) handler {
	for _, m := range methods {
		if m.name == name {
			return m.fn
		}
	}
	return s.extra[name]
}

func (s *Server) Call(ctx context.Context, method string, args []byte, emit func(Event)) (out []byte) {
	defer func() {
		if r := recover(); r != nil {
			out = encode(Envelope{Error: &ErrorBody{Code: CodeInternal, Message: fmt.Sprintf("internal error in %s: %v", method, r)}})
		}
	}()
	fn := s.lookup(method)
	if fn == nil {
		return failure(nil, badRequest(fmt.Errorf("unknown method %q", method)))
	}
	if ctx == nil {
		ctx = context.Background()
	}
	res, err := fn(ctx, s, args, emit)
	if err != nil {
		return failure(res, err)
	}
	return success(res)
}

func decodeArgs[T any](args []byte) (T, error) {
	return decodeStrict[T](args, "arguments", "argument object")
}

func DecodeOpenOptions(raw []byte) (OpenOptions, error) {
	return decodeStrict[OpenOptions](raw, "options", "options object")
}

func decodeStrict[T any](raw []byte, what, object string) (T, error) {
	var v T
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return v, nil
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		return v, badRequest(fmt.Errorf("invalid %s: %w", what, err))
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return v, badRequest(fmt.Errorf("invalid %s: trailing data after the %s", what, object))
	}
	return v, nil
}

func noArgs(args []byte) error {
	_, err := decodeArgs[struct{}](args)
	return err
}

func progress(emit func(Event)) func(library.Progress) {
	if emit == nil {
		return nil
	}
	return func(p library.Progress) {
		e := Event{Type: EventProgress, Stage: p.Stage, Done: p.Done, Total: p.Total}
		if carriesBytes(p) {
			done, total := p.BytesDone, p.BytesTotal
			e.BytesDone, e.BytesTotal = &done, &total
		}
		emit(e)
	}
}

func carriesBytes(p library.Progress) bool {
	switch p.Stage {
	case library.StageDownload:
		return true
	case library.StageBios:
		return p.BytesDone != 0 || p.BytesTotal != 0
	}
	return false
}

func positiveID(name string, id int) error {
	if id <= 0 {
		return badRequest(fmt.Errorf("%s must be a positive integer", name))
	}
	return nil
}

func status(ctx context.Context, s *Server, args []byte, _ func(Event)) (any, error) {
	if err := noArgs(args); err != nil {
		return nil, err
	}
	st, err := s.g.Status(ctx)
	if err != nil {
		return nil, err
	}
	return toStatus(st), nil
}

func settingsGet(_ context.Context, s *Server, args []byte, _ func(Event)) (any, error) {
	if err := noArgs(args); err != nil {
		return nil, err
	}
	return toSettings(s.g.Settings()), nil
}

func settingsSetHost(_ context.Context, s *Server, args []byte, _ func(Event)) (any, error) {
	a, err := decodeArgs[HostArgs](args)
	if err != nil {
		return nil, err
	}
	if err := s.g.SetHost(a.host()); err != nil {
		return nil, err
	}
	return toSettings(s.g.Settings()), nil
}

func settingsSetDirectoryMapping(_ context.Context, s *Server, args []byte, _ func(Event)) (any, error) {
	a, err := decodeArgs[DirectoryMappingArgs](args)
	if err != nil {
		return nil, err
	}
	var m *config.DirectoryMapping
	if a.Enabled {
		m = &config.DirectoryMapping{RomMSlug: a.Slug, RelativePath: a.RelativePath}
	}
	if err := s.g.SetDirectoryMapping(a.FSSlug, m); err != nil {
		return nil, err
	}
	return toSettings(s.g.Settings()), nil
}

func libraryRefresh(ctx context.Context, s *Server, args []byte, emit func(Event)) (any, error) {
	a, err := decodeArgs[RefreshArgs](args)
	if err != nil {
		return nil, err
	}
	res, err := s.g.Refresh(ctx, grout.RefreshOptions{Full: a.Full, Scan: a.Scan}, progress(emit))
	if res == nil {
		return nil, err
	}
	return toRefresh(*res), err
}

func libraryScan(ctx context.Context, s *Server, args []byte, emit func(Event)) (any, error) {
	if err := noArgs(args); err != nil {
		return nil, err
	}
	res, err := s.g.Scan(ctx, progress(emit))
	if err != nil {
		return nil, err
	}
	return toScan(res), nil
}

func libraryPlatforms(ctx context.Context, s *Server, args []byte, _ func(Event)) (any, error) {
	a, err := decodeArgs[PlatformsArgs](args)
	if err != nil {
		return nil, err
	}
	ps, err := s.g.Platforms(ctx, a.All)
	if err != nil {
		return nil, err
	}
	return toPlatforms(ps), nil
}

func libraryGames(ctx context.Context, s *Server, args []byte, _ func(Event)) (any, error) {
	a, err := decodeArgs[GamesArgs](args)
	if err != nil {
		return nil, err
	}
	if a.Offset < 0 || a.Limit < 0 {
		return nil, badRequest(errors.New("offset and limit must not be negative"))
	}
	q, err := a.GamesQuery.query()
	if err != nil {
		return nil, err
	}
	gs, err := s.g.Games(ctx, q)
	if err != nil {
		return nil, err
	}
	total := len(gs)
	start := min(a.Offset, total)
	end := total
	if a.Limit > 0 {
		end = min(start+a.Limit, total)
	}
	return GamesResult{Total: total, Games: toGames(gs[start:end])}, nil
}

func libraryFilterValues(ctx context.Context, s *Server, args []byte, _ func(Event)) (any, error) {
	a, err := decodeArgs[FilterValuesArgs](args)
	if err != nil {
		return nil, err
	}
	q, err := a.Query.query()
	if err != nil {
		return nil, err
	}
	vs, err := s.g.FilterValues(ctx, q, library.Dim(a.Dim))
	if err != nil {
		return nil, err
	}
	return nonNil(vs), nil
}

func libraryDims(_ context.Context, _ *Server, args []byte, _ func(Event)) (any, error) {
	if err := noArgs(args); err != nil {
		return nil, err
	}
	dims := library.Dims()
	out := make([]string, 0, len(dims))
	for _, d := range dims {
		out = append(out, string(d))
	}
	return out, nil
}

func libraryCollections(ctx context.Context, s *Server, args []byte, _ func(Event)) (any, error) {
	if err := noArgs(args); err != nil {
		return nil, err
	}
	rows, err := s.g.Collections(ctx)
	if err != nil {
		return nil, err
	}
	return toCollections(rows), nil
}

func libraryCollectionPlatforms(ctx context.Context, s *Server, args []byte, _ func(Event)) (any, error) {
	a, err := decodeArgs[CollectionKey](args)
	if err != nil {
		return nil, err
	}
	k, err := a.key()
	if err != nil {
		return nil, err
	}
	ps, err := s.g.CollectionPlatforms(ctx, k)
	if err != nil {
		return nil, err
	}
	return toPlatforms(ps), nil
}

func libraryGameState(ctx context.Context, s *Server, args []byte, _ func(Event)) (any, error) {
	a, err := decodeArgs[GameStateArgs](args)
	if err != nil {
		return nil, err
	}
	st, err := s.g.GameState(ctx, a.GameID)
	if err != nil {
		return nil, err
	}
	return GameStateResult{State: toGameState(st)}, nil
}

func fetchGame(ctx context.Context, s *Server, args []byte, emit func(Event)) (any, error) {
	a, err := decodeArgs[FetchGameArgs](args)
	if err != nil {
		return nil, err
	}
	if err := positiveID("game_id", a.GameID); err != nil {
		return nil, err
	}
	if a.FileID < 0 {
		return nil, badRequest(errors.New("file_id must not be negative"))
	}
	res, err := s.g.FetchGame(ctx, a.GameID, a.FileID, progress(emit))
	if err != nil {
		return nil, err
	}
	return normalizeFetchGame(res), nil
}

func normalizeFetchGame(r grout.FetchGameResult) grout.FetchGameResult {
	r.Placed = nonNil(r.Placed)
	r.Art.Written = nonNil(r.Art.Written)
	r.Art.Failed = nonNil(r.Art.Failed)
	return r
}

func fetchArt(ctx context.Context, s *Server, args []byte, emit func(Event)) (any, error) {
	a, err := decodeArgs[GameIDArgs](args)
	if err != nil {
		return nil, err
	}
	if err := positiveID("game_id", a.GameID); err != nil {
		return nil, err
	}
	res, err := s.g.FetchArt(ctx, a.GameID, progress(emit))
	if err != nil {
		return nil, err
	}
	res.Written = nonNil(res.Written)
	res.Failed = nonNil(res.Failed)
	return res, nil
}

func fetchBios(ctx context.Context, s *Server, args []byte, emit func(Event)) (any, error) {
	a, err := decodeArgs[PlatformIDArgs](args)
	if err != nil {
		return nil, err
	}
	if err := positiveID("platform_id", a.PlatformID); err != nil {
		return nil, err
	}
	files, err := s.g.FetchBios(ctx, a.PlatformID, progress(emit))
	if err != nil {
		return nil, err
	}
	return FetchBiosResult{Files: nonNilBios(files)}, nil
}

func libraryBios(ctx context.Context, s *Server, args []byte, _ func(Event)) (any, error) {
	a, err := decodeArgs[PlatformIDArgs](args)
	if err != nil {
		return nil, err
	}
	if err := positiveID("platform_id", a.PlatformID); err != nil {
		return nil, err
	}
	files, err := s.g.LibraryBios(ctx, a.PlatformID)
	if err != nil {
		return nil, err
	}
	return nonNilBios(files), nil
}

func nonNilBios(files []fetch.BiosFile) []fetch.BiosFile {
	files = nonNil(files)
	for i := range files {
		files[i].Paths = nonNil(files[i].Paths)
	}
	return files
}

func libraryGameFiles(ctx context.Context, s *Server, args []byte, _ func(Event)) (any, error) {
	a, err := decodeArgs[GameIDArgs](args)
	if err != nil {
		return nil, err
	}
	if err := positiveID("game_id", a.GameID); err != nil {
		return nil, err
	}
	files, err := s.g.GameFiles(ctx, a.GameID)
	if err != nil {
		return nil, err
	}
	return nonNil(files), nil
}

func authPairStart(ctx context.Context, s *Server, args []byte, _ func(Event)) (any, error) {
	a, err := decodeArgs[PairStartArgs](args)
	if err != nil {
		return nil, err
	}
	p, err := s.g.PairStart(ctx, a.DeviceName)
	if err != nil {
		return nil, err
	}
	return toPairStart(p), nil
}

func authPairWait(ctx context.Context, s *Server, args []byte, _ func(Event)) (any, error) {
	if err := noArgs(args); err != nil {
		return nil, err
	}
	r, err := s.g.PairWait(ctx)
	if err != nil {
		return nil, err
	}
	return toPairWait(r), nil
}

func authExchangeCode(ctx context.Context, s *Server, args []byte, _ func(Event)) (any, error) {
	a, err := decodeArgs[ExchangeCodeArgs](args)
	if err != nil {
		return nil, err
	}
	r, err := s.g.ExchangeCode(ctx, a.Code)
	if err != nil {
		return nil, err
	}
	return toExchange(r), nil
}

func authLogout(_ context.Context, s *Server, args []byte, _ func(Event)) (any, error) {
	if err := noArgs(args); err != nil {
		return nil, err
	}
	if err := s.g.Logout(); err != nil {
		return nil, err
	}
	return toSettings(s.g.Settings()), nil
}

func deviceRegister(ctx context.Context, s *Server, args []byte, _ func(Event)) (any, error) {
	a, err := decodeArgs[DeviceRegisterArgs](args)
	if err != nil {
		return nil, err
	}
	d, err := s.g.RegisterDevice(ctx, a.Name)
	if err != nil {
		return nil, err
	}
	return DeviceRegisterResult{DeviceID: d.DeviceID, Name: d.Name}, nil
}

const maxWireSeconds = math.MaxInt64 / int64(time.Second)

func settingField[T any](fields map[string]json.RawMessage, name string, set func(T) error) error {
	raw, ok := fields[name]
	if !ok {
		return nil
	}
	delete(fields, name)
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return badRequest(fmt.Errorf("%s must not be null", name))
	}
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		return badRequest(fmt.Errorf("invalid %s: %w", name, err))
	}
	return set(v)
}

func timeoutField(fields map[string]json.RawMessage, name string, dst **time.Duration) error {
	return settingField(fields, name, func(v int64) error {
		if v > maxWireSeconds || v < -maxWireSeconds {
			return badRequest(fmt.Errorf("%s is out of range", name))
		}
		d := time.Duration(v) * time.Second
		*dst = &d
		return nil
	})
}

func decodeAdditionalDownloads(fields map[string]json.RawMessage) (*grout.AdditionalDownloadsPatch, error) {
	a := &grout.AdditionalDownloadsPatch{}
	steps := []func() error{
		func() error {
			return settingField(fields, "marquee", func(v config.ArtKind) error { a.Marquee = &v; return nil })
		},
		func() error { return settingField(fields, "video", func(v bool) error { a.Video = &v; return nil }) },
		func() error {
			return settingField(fields, "thumbnail", func(v config.ArtKind) error { a.Thumbnail = &v; return nil })
		},
		func() error { return settingField(fields, "bezel", func(v bool) error { a.Bezel = &v; return nil }) },
		func() error { return settingField(fields, "manual", func(v bool) error { a.Manual = &v; return nil }) },
		func() error {
			return settingField(fields, "boxback", func(v bool) error { a.BoxBack = &v; return nil })
		},
		func() error { return settingField(fields, "fanart", func(v bool) error { a.Fanart = &v; return nil }) },
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return nil, err
		}
	}
	for name := range fields {
		return nil, badRequest(fmt.Errorf("invalid additional_downloads: unknown field %q", name))
	}
	return a, nil
}

func decodeSettingsPatch(args []byte) (grout.SettingsPatch, error) {
	var p grout.SettingsPatch
	fields := map[string]json.RawMessage{}
	if trimmed := bytes.TrimSpace(args); len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("null")) {
		dec := json.NewDecoder(bytes.NewReader(trimmed))
		if err := dec.Decode(&fields); err != nil {
			return p, badRequest(fmt.Errorf("invalid arguments: %w", err))
		}
		if _, err := dec.Token(); !errors.Is(err, io.EOF) {
			return p, badRequest(errors.New("invalid arguments: trailing data after the argument object"))
		}
	}
	steps := []func() error{
		func() error {
			return settingField(fields, "show_regular_collections", func(v bool) error { p.ShowRegularCollections = &v; return nil })
		},
		func() error {
			return settingField(fields, "show_smart_collections", func(v bool) error { p.ShowSmartCollections = &v; return nil })
		},
		func() error {
			return settingField(fields, "show_virtual_collections", func(v bool) error { p.ShowVirtualCollections = &v; return nil })
		},
		func() error {
			return settingField(fields, "downloaded_games", func(v DownloadedGamesMode) error {
				m, err := v.mode()
				p.DownloadedGames = &m
				return err
			})
		},
		func() error { return timeoutField(fields, "api_timeout", &p.APITimeout) },
		func() error { return timeoutField(fields, "download_timeout", &p.DownloadTimeout) },
		func() error {
			return settingField(fields, "log_level", func(v LogLevel) error {
				l, err := v.level()
				p.LogLevel = &l
				return err
			})
		},
		func() error {
			return settingField(fields, "download_art", func(v bool) error { p.DownloadArt = &v; return nil })
		},
		func() error {
			return settingField(fields, "unzip_downloads", func(v bool) error { p.UnzipDownloads = &v; return nil })
		},
		func() error {
			return settingField(fields, "art_kind", func(v config.ArtKind) error { p.ArtKind = &v; return nil })
		},
		func() error {
			return settingField(fields, "download_art_screenshot_preview", func(v bool) error { p.DownloadArtScreenshotPreview = &v; return nil })
		},
		func() error {
			return settingField(fields, "download_splash_art", func(v config.ArtKind) error { p.DownloadSplashArt = &v; return nil })
		},
		func() error {
			return settingField(fields, "additional_downloads", func(v map[string]json.RawMessage) error {
				a, err := decodeAdditionalDownloads(v)
				p.AdditionalDownloads = a
				return err
			})
		},
		func() error {
			return settingField(fields, "platform_order", func(v []string) error { p.PlatformOrder = &v; return nil })
		},
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return p, err
		}
	}
	for name := range fields {
		return p, badRequest(fmt.Errorf("invalid arguments: unknown field %q", name))
	}
	return p, nil
}

func settingsUpdate(_ context.Context, s *Server, args []byte, _ func(Event)) (any, error) {
	p, err := decodeSettingsPatch(args)
	if err != nil {
		return nil, err
	}
	c, err := s.g.UpdateSettings(p)
	if err != nil {
		return nil, err
	}
	return toSettings(c), nil
}
