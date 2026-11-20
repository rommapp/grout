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
)

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
	switch kind {
	case ArtCover:
		return f.Def.Art.Cover
	case ArtPreview:
		return f.Def.Art.Preview
	case ArtSplash:
		return f.Def.Art.Splash
	case ArtVideo:
		return f.Def.Art.Video
	case ArtManual:
		return f.Def.Art.Manual
	case ArtBezel:
		return f.Def.Art.Bezel
	}
	return nil
}

func (f *Firmware) ArtDir(kind ArtKind, slug, romDir, platformName string) (string, bool) {
	fact := f.artFact(kind)
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
