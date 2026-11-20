package library

import (
	"context"
	"sort"

	"github.com/rommapp/grout/core/romm"
)

type CollectionRow struct {
	Key   CollectionKey
	Name  string
	Games int
}

func (l *Library) enabledKinds() []any {
	var kinds []any
	if l.Config.ShowRegularCollections {
		kinds = append(kinds, string(romm.CollectionRegular))
	}
	if l.Config.ShowSmartCollections {
		kinds = append(kinds, string(romm.CollectionSmart))
	}
	if l.Config.ShowVirtualCollections {
		kinds = append(kinds, string(romm.CollectionVirtual))
	}
	return kinds
}

func (l *Library) Collections(ctx context.Context) ([]CollectionRow, error) {
	kinds := l.enabledKinds()
	ids, err := l.enabledPlatformIDs(ctx)
	if err != nil {
		return nil, err
	}
	if len(kinds) == 0 || len(ids) == 0 {
		return nil, nil
	}
	args := append([]any{}, kinds...)
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := l.Store.DB.QueryContext(ctx,
		`SELECT c.kind, c.remote_id, c.name, COUNT(DISTINCT g.id)
		FROM collections c
		JOIN collection_games cg ON cg.kind = c.kind AND cg.remote_id = c.remote_id
		JOIN games g ON g.id = cg.game_id
		WHERE c.kind IN (`+placeholders(len(kinds))+`) AND g.platform_id IN (`+placeholders(len(ids))+`)
		GROUP BY c.kind, c.remote_id, c.name`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CollectionRow
	for rows.Next() {
		var r CollectionRow
		var kind string
		if err := rows.Scan(&kind, &r.Key.RemoteID, &r.Name, &r.Games); err != nil {
			return nil, err
		}
		r.Key.Kind = romm.CollectionKind(kind)
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(out, func(i, j int) bool {
		fi, fj := fold(out[i].Name), fold(out[j].Name)
		if fi != fj {
			return fi < fj
		}
		if out[i].Key.Kind != out[j].Key.Kind {
			return out[i].Key.Kind < out[j].Key.Kind
		}
		return out[i].Key.RemoteID < out[j].Key.RemoteID
	})
	return out, nil
}

func (l *Library) CollectionPlatforms(ctx context.Context, k CollectionKey) ([]Platform, error) {
	rows, err := l.Store.DB.QueryContext(ctx,
		`SELECT g.platform_id, COUNT(DISTINCT g.id)
		FROM collection_games cg JOIN games g ON g.id = cg.game_id
		WHERE cg.kind = ? AND cg.remote_id = ?
		GROUP BY g.platform_id`, string(k.Kind), k.RemoteID)
	if err != nil {
		return nil, err
	}
	counts := map[int]int{}
	for rows.Next() {
		var id, n int
		if err := rows.Scan(&id, &n); err != nil {
			rows.Close()
			return nil, err
		}
		counts[id] = n
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	all, err := l.load(ctx)
	if err != nil {
		return nil, err
	}
	out := all[:0]
	for _, p := range all {
		if n := counts[p.ID]; p.Enabled && n > 0 {
			p.RomCount = n
			out = append(out, p)
		}
	}
	disambiguate(out)
	return out, nil
}
