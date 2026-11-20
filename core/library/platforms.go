package library

import (
	"context"
	"path"
	"sort"
)

type Platform struct {
	ID       int
	Slug     string
	FSSlug   string
	Name     string
	RomCount int
	Enabled  bool
	Proposed bool
	RomDirs  []string
}

func (l *Library) Platforms(ctx context.Context) ([]Platform, error) {
	all, err := l.load(ctx)
	if err != nil {
		return nil, err
	}
	out := all[:0]
	for _, p := range all {
		if p.Enabled && p.RomCount > 0 {
			out = append(out, p)
		}
	}
	disambiguate(out)
	return out, nil
}

func (l *Library) AllPlatforms(ctx context.Context) ([]Platform, error) {
	all, err := l.load(ctx)
	if err != nil {
		return nil, err
	}
	disambiguate(all)
	return all, nil
}

func (l *Library) load(ctx context.Context) ([]Platform, error) {
	rows, err := l.Store.DB.QueryContext(ctx, `SELECT id, slug, fs_slug, name, custom_name, rom_count FROM platforms`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Platform
	for rows.Next() {
		var p Platform
		var name, custom string
		if err := rows.Scan(&p.ID, &p.Slug, &p.FSSlug, &name, &custom, &p.RomCount); err != nil {
			return nil, err
		}
		p.Name = name
		if custom != "" {
			p.Name = custom
		}
		_, p.Enabled = l.Config.DirectoryMappings[p.FSSlug]
		p.Proposed = l.Firmware != nil && len(l.Firmware.RomDirs(p.Slug)) > 0
		p.RomDirs = l.RomDirs(p)
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	l.order(out)
	return out, nil
}

func (l *Library) order(ps []Platform) {
	rank := map[string]int{}
	for i, s := range l.Config.PlatformOrder {
		if _, ok := rank[s]; !ok {
			rank[s] = i
		}
	}
	sort.SliceStable(ps, func(i, j int) bool {
		ri, iok := rank[ps[i].FSSlug]
		rj, jok := rank[ps[j].FSSlug]
		if iok != jok {
			return iok
		}
		if iok {
			return ri < rj
		}
		fi, fj := fold(ps[i].Name), fold(ps[j].Name)
		if fi != fj {
			return fi < fj
		}
		return ps[i].ID < ps[j].ID
	})
}

func disambiguate(ps []Platform) {
	count := map[string]int{}
	for _, p := range ps {
		count[p.Name]++
	}
	for i := range ps {
		if count[ps[i].Name] > 1 {
			ps[i].Name += " (" + ps[i].FSSlug + ")"
		}
	}
}

func (l *Library) RomDirs(p Platform) []string {
	if m, ok := l.Config.DirectoryMappings[p.FSSlug]; ok && m.RelativePath != "" && l.Firmware != nil {
		return []string{path.Join(l.Firmware.Roms, m.RelativePath)}
	}
	if l.Firmware == nil {
		return nil
	}
	return l.Firmware.RomDirs(p.Slug)
}
