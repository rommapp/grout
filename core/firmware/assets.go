package firmware

import (
	"fmt"
	"path"
	"slices"
)

type ArtKind string

const (
	ArtCover   ArtKind = "cover"
	ArtPreview ArtKind = "preview"
	ArtSplash  ArtKind = "splash"
	ArtVideo   ArtKind = "video"
	ArtManual  ArtKind = "manual"
	ArtBezel   ArtKind = "bezel"

	ArtMarquee   ArtKind = "marquee"
	ArtThumbnail ArtKind = "thumbnail"
	ArtBoxback   ArtKind = "boxback"
	ArtFanart    ArtKind = "fanart"
)

func ArtKinds() []ArtKind {
	return []ArtKind{ArtCover, ArtPreview, ArtSplash, ArtVideo, ArtManual, ArtBezel, ArtMarquee, ArtThumbnail, ArtBoxback, ArtFanart}
}

var esSuffix = map[ArtKind]string{
	ArtThumbnail: "-thumb",
	ArtMarquee:   "-marquee",
	ArtBoxback:   "-boxback",
	ArtFanart:    "-fanart",
}

func (f *Firmware) ArtFileName(kind ArtKind, stem string) string {
	switch kind {
	case ArtVideo:
		return stem + ".mp4"
	case ArtManual:
		return stem + ".pdf"
	}
	if f.Def.Art.Naming == "es-suffix" {
		return stem + esSuffix[kind] + ".png"
	}
	return stem + ".png"
}

type MultiLayout struct {
	Folder   string
	Playlist []byte
	Beside   bool
}

func (f *Firmware) MultiLayout(fsName string, m3u []byte) MultiLayout {
	if hook := f.reg.Layout[f.Def.RomFolders.MultiLayout]; hook != nil {
		folder, playlist, beside := hook(fsName, m3u)
		return MultiLayout{Folder: folder, Playlist: playlist, Beside: beside}
	}
	return MultiLayout{Folder: fsName, Playlist: m3u}
}

const defaultBiosTemplate = "{bios}/{rel}"

func (f *Firmware) BiosPaths(slug, rel string) []string {
	row, hasRow := f.Rows()[slug]
	var templates []string
	if hasRow && len(row.Bios) > 0 {
		templates = row.Bios
	} else if picked, ok := f.Def.Bios.Default.Pick(f.Version); ok {
		templates = picked.Strings()
	}
	if len(templates) == 0 {
		templates = []string{defaultBiosTemplate}
	}

	var saveFolders []string
	if hasRow {
		saveFolders = row.Saves
	}
	base := map[string]string{
		"card":  f.Card,
		"roms":  f.Roms,
		"bios":  f.Bios,
		"saves": f.Saves,
		"name":  path.Base(rel),
		"rel":   rel,
	}

	expand := func(templates []string) []string {
		var out []string
		for _, t := range templates {
			if slices.Contains(Variables(t), "save_folder") {
				for _, s := range saveFolders {
					vars := cloneVars(base)
					vars["save_folder"] = s
					if v, err := ExpandEnv(t, vars, f.Host.Getenv); err == nil {
						out = appendUnique(out, path.Clean(v))
					}
				}
				continue
			}
			if v, err := ExpandEnv(t, base, f.Host.Getenv); err == nil {
				out = appendUnique(out, path.Clean(v))
			}
		}
		return out
	}

	out := expand(templates)
	if len(out) == 0 {
		out = expand([]string{defaultBiosTemplate})
	}
	return out
}

func cloneVars(m map[string]string) map[string]string {
	out := make(map[string]string, len(m)+1)
	for k, v := range m {
		out[k] = v
	}
	return out
}

func appendUnique(s []string, v string) []string {
	if slices.Contains(s, v) {
		return s
	}
	return append(s, v)
}

func (f *Firmware) artFact(kind ArtKind) *Fact {
	return artFactOf(f.Def, kind)
}

func artFactOf(d *Definition, kind ArtKind) *Fact {
	switch kind {
	case ArtCover:
		return d.Art.Cover
	case ArtPreview:
		return d.Art.Preview
	case ArtSplash:
		return d.Art.Splash
	case ArtVideo:
		return d.Art.Video
	case ArtManual:
		return d.Art.Manual
	case ArtBezel:
		return d.Art.Bezel
	case ArtMarquee:
		return d.Art.Marquee
	case ArtThumbnail:
		return d.Art.Thumbnail
	case ArtBoxback:
		return d.Art.Boxback
	case ArtFanart:
		return d.Art.Fanart
	}
	return nil
}

var artVars = []string{"card", "roms", "rom_dir", "rom_folder", "system"}

func (f *Firmware) ArtDir(kind ArtKind, slug, romDir, platformName string) (string, bool) {
	return f.expandArtFact(f.artFact(kind), slug, romDir, platformName)
}

func (f *Firmware) expandArtFact(fact *Fact, slug, romDir, platformName string) (string, bool) {
	if fact == nil {
		return "", false
	}
	picked, ok := fact.Pick(f.Version)
	if !ok {
		return "", false
	}
	tmpl := picked.String()
	if tmpl == "" {
		return "", false
	}
	system := platformName
	if row, ok := f.Rows()[slug]; ok && row.System != "" {
		system = row.System
	}
	vars := map[string]string{
		"card":       f.Card,
		"roms":       f.Roms,
		"bios":       f.Bios,
		"saves":      f.Saves,
		"rom_dir":    romDir,
		"rom_folder": path.Base(romDir),
		"system":     system,
	}
	v, err := ExpandEnv(tmpl, vars, f.Host.Getenv)
	if err != nil {
		return "", false
	}
	return path.Clean(v), true
}

func (f *Firmware) InputMapping() ([]byte, error) {
	in, ok := f.Def.Devices.Inputs[f.Device]
	if !ok {
		return nil, nil
	}
	if in.File != "" {
		return InputFile(f.Def.ID, in.File)
	}
	fn := f.reg.Input[in.Hook]
	if in.Hook == "" || fn == nil {
		return nil, fmt.Errorf("devices.inputs.%s: input hook %q is not registered", f.Device, in.Hook)
	}
	return fn(f.Host)
}

func (f *Firmware) Quirks() Quirks {
	return f.Def.Devices.Quirks[f.Device]
}
