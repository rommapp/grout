package api

import (
	"fmt"
	"time"

	"github.com/rommapp/grout"
	"github.com/rommapp/grout/core/config"
	"github.com/rommapp/grout/core/library"
	"github.com/rommapp/grout/core/romm"
)

type GameState string

const (
	GameStateNotDownloaded GameState = "not_downloaded"
	GameStatePartial       GameState = "partial"
	GameStateDownloaded    GameState = "downloaded"
)

type GameKind string

const (
	GameKindSingle GameKind = "single"
	GameKindMulti  GameKind = "multi"
	GameKindNested GameKind = "nested"
)

type CollectionKind string

const (
	CollectionKindRegular CollectionKind = "regular"
	CollectionKindSmart   CollectionKind = "smart"
	CollectionKindVirtual CollectionKind = "virtual"
)

type DownloadedGamesMode string

const (
	DownloadedGamesDoNothing DownloadedGamesMode = "do_nothing"
	DownloadedGamesMark      DownloadedGamesMode = "mark"
	DownloadedGamesFilter    DownloadedGamesMode = "filter"
)

type LogLevel string

const (
	LogLevelDebug LogLevel = "DEBUG"
	LogLevelInfo  LogLevel = "INFO"
	LogLevelError LogLevel = "ERROR"
)

type StatusResult struct {
	ConfigSource  string `json:"config_source"`
	Host          string `json:"host"`
	ServerVersion string `json:"server_version"`
	Snapshots     bool   `json:"snapshots"`
}

type HostInfo struct {
	DisplayName        string `json:"display_name"`
	RootURI            string `json:"root_uri"`
	Port               int    `json:"port"`
	Username           string `json:"username"`
	HasToken           bool   `json:"has_token"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify"`
}

type DirectoryMapping struct {
	Slug         string `json:"slug"`
	RelativePath string `json:"relative_path"`
}

type Settings struct {
	Hosts                  []HostInfo                  `json:"hosts"`
	DirectoryMappings      map[string]DirectoryMapping `json:"directory_mappings"`
	ShowRegularCollections bool                        `json:"show_regular_collections"`
	ShowSmartCollections   bool                        `json:"show_smart_collections"`
	ShowVirtualCollections bool                        `json:"show_virtual_collections"`
	DownloadedGames        DownloadedGamesMode         `json:"downloaded_games"`
	APITimeout             int                         `json:"api_timeout"`
	DownloadTimeout        int                         `json:"download_timeout"`
	LogLevel               LogLevel                    `json:"log_level"`
	PlatformOrder          []string                    `json:"platform_order"`
}

type HostArgs struct {
	DisplayName        string `json:"display_name"`
	RootURI            string `json:"root_uri"`
	Port               int    `json:"port"`
	Username           string `json:"username"`
	Token              string `json:"token"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify"`
}

type DirectoryMappingArgs struct {
	FSSlug       string `json:"fs_slug"`
	Slug         string `json:"slug"`
	RelativePath string `json:"relative_path"`
	Enabled      bool   `json:"enabled"`
}

type RefreshArgs struct {
	Full bool `json:"full"`
	Scan bool `json:"scan"`
}

type RefreshSummary struct {
	Platforms        int      `json:"platforms"`
	Games            int      `json:"games"`
	Collections      int      `json:"collections"`
	Deleted          int      `json:"deleted"`
	FailedPlatforms  []string `json:"failed_platforms"`
	CollectionErrors []string `json:"collection_errors"`
	PurgeError       string   `json:"purge_error"`
}

type ScanResult struct {
	Files          int      `json:"files"`
	Matched        int      `json:"matched"`
	Platforms      int      `json:"platforms"`
	SkippedFolders []string `json:"skipped_folders"`
}

type RefreshResult struct {
	Refresh RefreshSummary `json:"refresh"`
	Scan    *ScanResult    `json:"scan"`
}

type PlatformsArgs struct {
	All bool `json:"all"`
}

type Platform struct {
	ID       int      `json:"id"`
	Slug     string   `json:"slug"`
	FSSlug   string   `json:"fs_slug"`
	Name     string   `json:"name"`
	RomCount int      `json:"rom_count"`
	Enabled  bool     `json:"enabled"`
	Proposed bool     `json:"proposed"`
	RomDirs  []string `json:"rom_dirs"`
}

type CollectionKey struct {
	Kind     CollectionKind `json:"kind"`
	RemoteID string         `json:"remote_id"`
}

type GamesQuery struct {
	PlatformID     int                 `json:"platform_id"`
	Collection     *CollectionKey      `json:"collection"`
	Search         string              `json:"search"`
	Filters        map[string][]string `json:"filters"`
	HideDownloaded bool                `json:"hide_downloaded"`
}

type GamesArgs struct {
	GamesQuery
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
}

type Game struct {
	ID         int       `json:"id"`
	PlatformID int       `json:"platform_id"`
	Name       string    `json:"name"`
	Kind       GameKind  `json:"kind"`
	State      GameState `json:"state"`
}

type GamesResult struct {
	Total int    `json:"total"`
	Games []Game `json:"games"`
}

type FilterValuesArgs struct {
	Query GamesQuery `json:"query"`
	Dim   string     `json:"dim"`
}

type Collection struct {
	Kind     CollectionKind `json:"kind"`
	RemoteID string         `json:"remote_id"`
	Name     string         `json:"name"`
	Games    int            `json:"games"`
}

type GameStateArgs struct {
	GameID int `json:"game_id"`
}

type GameStateResult struct {
	State GameState `json:"state"`
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func seconds(d config.DurationSeconds) int {
	return int(d.Duration() / time.Second)
}

func toStatus(s grout.Status) StatusResult {
	return StatusResult{
		ConfigSource:  s.ConfigSource,
		Host:          s.Host,
		ServerVersion: s.ServerVersion,
		Snapshots:     s.Snapshots,
	}
}

func toSettings(c config.Config) Settings {
	out := Settings{
		Hosts:                  []HostInfo{},
		DirectoryMappings:      map[string]DirectoryMapping{},
		ShowRegularCollections: c.ShowRegularCollections,
		ShowSmartCollections:   c.ShowSmartCollections,
		ShowVirtualCollections: c.ShowVirtualCollections,
		DownloadedGames:        DownloadedGamesMode(c.DownloadedGames),
		APITimeout:             seconds(c.ApiTimeout),
		DownloadTimeout:        seconds(c.DownloadTimeout),
		LogLevel:               LogLevel(c.LogLevel),
		PlatformOrder:          nonNil(append([]string(nil), c.PlatformOrder...)),
	}
	for _, h := range c.Hosts {
		out.Hosts = append(out.Hosts, HostInfo{
			DisplayName:        h.DisplayName,
			RootURI:            h.RootURI,
			Port:               h.Port,
			Username:           h.Username,
			HasToken:           h.Token != "",
			InsecureSkipVerify: h.InsecureSkipVerify,
		})
	}
	for slug, m := range c.DirectoryMappings {
		out.DirectoryMappings[slug] = DirectoryMapping{Slug: m.RomMSlug, RelativePath: m.RelativePath}
	}
	return out
}

func (a HostArgs) host() config.Host {
	return config.Host{
		DisplayName:        a.DisplayName,
		RootURI:            a.RootURI,
		Port:               a.Port,
		Username:           a.Username,
		Token:              a.Token,
		InsecureSkipVerify: a.InsecureSkipVerify,
	}
}

func toRefresh(r grout.RefreshResult) RefreshResult {
	out := RefreshResult{Refresh: RefreshSummary{
		Platforms:        r.Refresh.Platforms,
		Games:            r.Refresh.Games,
		Collections:      r.Refresh.Collections,
		Deleted:          r.Refresh.Deleted,
		FailedPlatforms:  nonNil(r.Refresh.FailedPlatforms),
		CollectionErrors: nonNil(r.Refresh.CollectionErrors),
		PurgeError:       r.Refresh.PurgeError,
	}}
	if r.Scan != nil {
		s := toScan(*r.Scan)
		out.Scan = &s
	}
	return out
}

func toScan(s library.ScanResult) ScanResult {
	return ScanResult{
		Files:          s.Files,
		Matched:        s.Matched,
		Platforms:      s.Platforms,
		SkippedFolders: nonNil(s.SkippedFolders),
	}
}

func toPlatforms(ps []library.Platform) []Platform {
	out := make([]Platform, 0, len(ps))
	for _, p := range ps {
		out = append(out, Platform{
			ID:       p.ID,
			Slug:     p.Slug,
			FSSlug:   p.FSSlug,
			Name:     p.Name,
			RomCount: p.RomCount,
			Enabled:  p.Enabled,
			Proposed: p.Proposed,
			RomDirs:  nonNil(p.RomDirs),
		})
	}
	return out
}

func toGameState(s library.DownloadState) GameState {
	switch s {
	case library.Partial:
		return GameStatePartial
	case library.Downloaded:
		return GameStateDownloaded
	}
	return GameStateNotDownloaded
}

func toGameKind(k string) GameKind {
	switch k {
	case "multi":
		return GameKindMulti
	case "nested":
		return GameKindNested
	}
	return GameKindSingle
}

func toGames(gs []library.Game) []Game {
	out := make([]Game, 0, len(gs))
	for _, g := range gs {
		out = append(out, Game{
			ID:         g.ID,
			PlatformID: g.PlatformID,
			Name:       g.Name,
			Kind:       toGameKind(g.Kind),
			State:      toGameState(g.State),
		})
	}
	return out
}

func toCollectionKind(k romm.CollectionKind) CollectionKind {
	switch k {
	case romm.CollectionSmart:
		return CollectionKindSmart
	case romm.CollectionVirtual:
		return CollectionKindVirtual
	}
	return CollectionKindRegular
}

func toCollections(rows []library.CollectionRow) []Collection {
	out := make([]Collection, 0, len(rows))
	for _, r := range rows {
		out = append(out, Collection{
			Kind:     toCollectionKind(r.Key.Kind),
			RemoteID: r.Key.RemoteID,
			Name:     r.Name,
			Games:    r.Games,
		})
	}
	return out
}

func (k CollectionKey) key() (library.CollectionKey, error) {
	var kind romm.CollectionKind
	switch k.Kind {
	case CollectionKindRegular:
		kind = romm.CollectionRegular
	case CollectionKindSmart:
		kind = romm.CollectionSmart
	case CollectionKindVirtual:
		kind = romm.CollectionVirtual
	default:
		return library.CollectionKey{}, badRequest(fmt.Errorf("unknown collection kind %q", k.Kind))
	}
	return library.CollectionKey{Kind: kind, RemoteID: k.RemoteID}, nil
}

func (q GamesQuery) query() (library.Query, error) {
	out := library.Query{
		PlatformID:     q.PlatformID,
		Search:         q.Search,
		HideDownloaded: q.HideDownloaded,
	}
	if q.Collection != nil {
		k, err := q.Collection.key()
		if err != nil {
			return out, err
		}
		out.Collection = &k
	}
	if len(q.Filters) > 0 {
		out.Filters = make(map[library.Dim][]string, len(q.Filters))
		for d, vs := range q.Filters {
			out.Filters[library.Dim(d)] = vs
		}
	}
	return out, nil
}
