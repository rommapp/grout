package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"time"
)

type GameMeta struct {
	GameID      int
	PlatformID  int
	Name        string
	Summary     string
	ReleaseDate time.Time
	Rating      float64
	HasRating   bool
	Players     string
	Genres      []string
	Companies   []string
	Regions     []string
	Languages   []string
	MD5ByName   map[string]string
}

type OnCardGame struct {
	GameID     int
	PlatformID int
	LaunchPath string
	Folder     bool
}

func (l *Library) Meta(ctx context.Context, gameID int) (GameMeta, error) {
	m := GameMeta{GameID: gameID, MD5ByName: map[string]string{}}
	var releaseMS int64
	err := l.Store.DB.QueryRowContext(ctx, `SELECT platform_id, name, summary, player_count, first_release_date, average_rating
		FROM games WHERE id = ?`, gameID).Scan(&m.PlatformID, &m.Name, &m.Summary, &m.Players, &releaseMS, &m.Rating)
	if errors.Is(err, sql.ErrNoRows) {
		return GameMeta{}, fmt.Errorf("game %d: %w", gameID, ErrUnknownGame)
	}
	if err != nil {
		return GameMeta{}, err
	}
	if releaseMS != 0 {
		m.ReleaseDate = time.UnixMilli(releaseMS).UTC()
	}
	m.HasRating = m.Rating > 0
	for dim, dst := range map[Dim]*[]string{
		DimGenre:    &m.Genres,
		DimCompany:  &m.Companies,
		DimRegion:   &m.Regions,
		DimLanguage: &m.Languages,
	} {
		vals, err := l.metaValues(ctx, gameID, dim)
		if err != nil {
			return GameMeta{}, err
		}
		*dst = vals
	}
	rows, err := l.Store.DB.QueryContext(ctx, `SELECT file_name, md5 FROM game_files WHERE game_id = ? AND md5 != '' ORDER BY file_id`, gameID)
	if err != nil {
		return GameMeta{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var name, md5 string
		if err := rows.Scan(&name, &md5); err != nil {
			return GameMeta{}, err
		}
		m.MD5ByName[name] = md5
	}
	return m, rows.Err()
}

func (l *Library) metaValues(ctx context.Context, gameID int, dim Dim) ([]string, error) {
	rows, err := l.Store.DB.QueryContext(ctx, `SELECT value FROM game_meta WHERE game_id = ? AND dim = ? ORDER BY value`, gameID, string(dim))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (l *Library) OnCard(ctx context.Context, platformID int) ([]OnCardGame, error) {
	rows, err := l.Store.DB.QueryContext(ctx, `SELECT id, platform_id FROM games
		WHERE (? = 0 OR platform_id = ?)
		AND (id IN (SELECT game_id FROM downloads) OR id IN (SELECT game_id FROM local_files WHERE game_id != 0))
		ORDER BY id`, platformID, platformID)
	if err != nil {
		return nil, err
	}
	var cands []OnCardGame
	for rows.Next() {
		var g OnCardGame
		if err := rows.Scan(&g.GameID, &g.PlatformID); err != nil {
			rows.Close()
			return nil, err
		}
		cands = append(cands, g)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	out := []OnCardGame{}
	for _, g := range cands {
		launch, ok, err := l.LaunchFile(ctx, g.GameID)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		g.LaunchPath = launch
		if l.Firmware != nil && l.Firmware.Host != nil {
			info, err := l.Firmware.Host.Stat(launch)
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil {
				return nil, fmt.Errorf("stat %s: %w", launch, err)
			}
			g.Folder = info.IsDir()
		}
		out = append(out, g)
	}
	return out, nil
}
