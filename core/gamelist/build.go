package gamelist

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/rommapp/grout/core/firmware"
	"github.com/rommapp/grout/core/firmware/host"
	"github.com/rommapp/grout/core/gamelist/entry"
	"github.com/rommapp/grout/core/library"
)

var ErrNoPlaylist = errors.New("multi-file game has no playlist")

var ErrOutsideRomDir = errors.New("launch file is outside the rom folder")

type Game struct {
	Meta       library.GameMeta
	Platform   library.Platform
	RomDir     string
	LaunchPath string
	Folder     bool
}

func stem(launchPath string) string {
	base := path.Base(launchPath)
	return strings.TrimSuffix(base, path.Ext(base))
}

func relInside(dir, p string) (string, bool) {
	dir = path.Clean(dir)
	p = path.Clean(p)
	rest, ok := strings.CutPrefix(p, dir+"/")
	if !ok || rest == "" {
		return "", false
	}
	return "./" + rest, true
}

func Build(fw *firmware.Firmware, h host.Host, g Game, omitRegion bool) (entry.Entry, error) {
	if g.Folder {
		return entry.Entry{}, fmt.Errorf("%s: %w", g.LaunchPath, ErrNoPlaylist)
	}
	rel, ok := relInside(g.RomDir, g.LaunchPath)
	if !ok {
		return entry.Entry{}, fmt.Errorf("%s: %w", g.LaunchPath, ErrOutsideRomDir)
	}
	s := stem(g.LaunchPath)
	name := g.Meta.Name
	if name == "" {
		name = s
	}
	if len(g.Meta.Regions) > 0 && !omitRegion {
		name += " (" + strings.Join(g.Meta.Regions, ", ") + ")"
	}
	e := entry.Entry{
		Path:        rel,
		Name:        name,
		Desc:        g.Meta.Summary,
		ReleaseDate: g.Meta.ReleaseDate,
		Rating:      g.Meta.Rating,
		HasRating:   g.Meta.HasRating,
		Players:     g.Meta.Players,
		Genres:      g.Meta.Genres,
		Developers:  g.Meta.Companies,
		Regions:     g.Meta.Regions,
		Languages:   g.Meta.Languages,
		MD5:         g.Meta.MD5ByName[path.Base(g.LaunchPath)],
		Art:         map[firmware.ArtKind]string{},
	}
	for _, kind := range firmware.ArtKinds() {
		dir, ok := fw.ArtDir(kind, g.Platform.Slug, g.RomDir, g.Platform.Name)
		if !ok {
			continue
		}
		file := path.Join(dir, fw.ArtFileName(kind, s))
		artRel, ok := relInside(g.RomDir, file)
		if !ok {
			continue
		}
		info, err := h.Stat(file)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return entry.Entry{}, fmt.Errorf("stat %s: %w", file, err)
		}
		if info.Mode().IsRegular() {
			e.Art[kind] = artRel
		}
	}
	return e, nil
}
