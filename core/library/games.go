package library

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"grout/core/romm"
)

type Dim string

const (
	DimGenre     Dim = "genre"
	DimFranchise Dim = "franchise"
	DimCompany   Dim = "company"
	DimGameMode  Dim = "game_mode"
	DimAgeRating Dim = "age_rating"
	DimRegion    Dim = "region"
	DimLanguage  Dim = "language"
	DimTag       Dim = "tag"
)

func Dims() []Dim {
	return []Dim{DimGenre, DimFranchise, DimCompany, DimGameMode, DimAgeRating, DimRegion, DimLanguage, DimTag}
}

func validDim(d Dim) bool {
	for _, k := range Dims() {
		if k == d {
			return true
		}
	}
	return false
}

type CollectionKey struct {
	Kind     romm.CollectionKind
	RemoteID string
}

type DownloadState int

const (
	NotDownloaded DownloadState = iota
	Partial
	Downloaded
)

type Query struct {
	PlatformID     int
	Collection     *CollectionKey
	Search         string
	Filters        map[Dim][]string
	HideDownloaded bool
}

type Game struct {
	ID         int
	PlatformID int
	Name       string
	Kind       string
	State      DownloadState
}

func (l *Library) Games(ctx context.Context, q Query) ([]Game, error) {
	where, args, err := l.conditions(ctx, q, "")
	if err != nil {
		return nil, err
	}
	if where == "" {
		return nil, nil
	}
	outer := ""
	if q.HideDownloaded {
		outer = " WHERE state < 2"
	}
	rows, err := l.Store.DB.QueryContext(ctx,
		`SELECT id, platform_id, name, kind, state FROM (SELECT g.id, g.platform_id, g.name, g.kind, g.sort_key, `+stateExpr+` AS state FROM games g WHERE `+where+`)`+outer+` ORDER BY sort_key, id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Game
	for rows.Next() {
		var g Game
		if err := rows.Scan(&g.ID, &g.PlatformID, &g.Name, &g.Kind, &g.State); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (l *Library) FilterValues(ctx context.Context, q Query, d Dim) ([]string, error) {
	if !validDim(d) {
		return nil, fmt.Errorf("unknown filter dimension %q", d)
	}
	where, args, err := l.conditions(ctx, q, d)
	if err != nil {
		return nil, err
	}
	if where == "" {
		return nil, nil
	}
	rows, err := l.Store.DB.QueryContext(ctx,
		`SELECT DISTINCT m.value FROM game_meta m JOIN games g ON g.id = m.game_id WHERE m.dim = ? AND `+where,
		append([]any{string(d)}, args...)...)
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(out, func(i, j int) bool {
		fi, fj := fold(out[i]), fold(out[j])
		if fi != fj {
			return fi < fj
		}
		return out[i] < out[j]
	})
	return out, nil
}

func (l *Library) enabledPlatformIDs(ctx context.Context) ([]int, error) {
	all, err := l.load(ctx)
	if err != nil {
		return nil, err
	}
	var ids []int
	for _, p := range all {
		if p.Enabled {
			ids = append(ids, p.ID)
		}
	}
	return ids, nil
}

func endsWithPunct(s string) bool {
	r, _ := utf8.DecodeLastRuneInString(s)
	return !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.Is(unicode.Mn, r)
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

func (l *Library) conditions(ctx context.Context, q Query, skip Dim) (string, []any, error) {
	for d := range q.Filters {
		if !validDim(d) {
			return "", nil, fmt.Errorf("unknown filter dimension %q", d)
		}
	}
	ids, err := l.enabledPlatformIDs(ctx)
	if err != nil {
		return "", nil, err
	}
	if len(ids) == 0 {
		return "", nil, nil
	}
	args := make([]any, 0, len(ids)+8)
	for _, id := range ids {
		args = append(args, id)
	}
	conds := []string{"g.platform_id IN (" + placeholders(len(ids)) + ")"}
	if q.PlatformID != 0 {
		conds = append(conds, "g.platform_id = ?")
		args = append(args, q.PlatformID)
	}
	if q.Collection != nil {
		conds = append(conds, `EXISTS (SELECT 1 FROM collection_games cg WHERE cg.game_id = g.id AND cg.kind = ? AND cg.remote_id = ?)`)
		args = append(args, string(q.Collection.Kind), q.Collection.RemoteID)
	}
	for _, raw := range strings.Fields(q.Search) {
		tok := fold(raw)
		if tok == "" {
			continue
		}
		if endsWithPunct(raw) {
			conds = append(conds, "instr(g.search_name || ' ', ?) > 0")
			args = append(args, tok+" ")
			continue
		}
		conds = append(conds, "instr(g.search_name, ?) > 0")
		args = append(args, tok)
	}
	dims := make([]string, 0, len(q.Filters))
	for d := range q.Filters {
		dims = append(dims, string(d))
	}
	sort.Strings(dims)
	for _, d := range dims {
		vals := q.Filters[Dim(d)]
		if Dim(d) == skip || len(vals) == 0 {
			continue
		}
		conds = append(conds, `EXISTS (SELECT 1 FROM game_meta fm WHERE fm.game_id = g.id AND fm.dim = ? AND fm.value IN (`+placeholders(len(vals))+`))`)
		args = append(args, d)
		for _, v := range vals {
			args = append(args, v)
		}
	}
	return strings.Join(conds, " AND "), args, nil
}
