package firmware

import (
	"path"
	"strings"
)

type SaveLocation struct {
	Dir       string
	Label     string
	Source    string
	Preferred bool
}

type Dropped struct {
	Slug   string
	Dir    string
	Reason string
}

var validSaveExtensions = map[string]bool{
	".srm": true,
	".sav": true,
	".dsv": true,
	".mcr": true,
	".mcd": true,
	".brm": true,
	".eep": true,
	".sra": true,
	".fla": true,
	".mpk": true,
	".nv":  true,
}

func (f *Firmware) RomDirs(slug string) []string {
	if v, ok := f.Overrides.RomFolders[slug]; ok && v != "" {
		return []string{path.Join(f.Roms, v)}
	}
	var out []string
	for _, r := range f.Rows()[slug].Roms {
		out = append(out, path.Join(f.Roms, r))
	}
	return out
}

func Tag(folder string) string {
	var b strings.Builder
	rest := folder
	for {
		open := strings.IndexByte(rest, '(')
		if open < 0 {
			break
		}
		end := strings.IndexByte(rest[open:], ')')
		if end < 0 {
			break
		}
		b.WriteString(rest[open+1 : open+end])
		rest = rest[open+end+1:]
	}
	return b.String()
}

func (f *Firmware) SaveLocations(slug, romPath string) ([]SaveLocation, []Dropped) {
	romDir := path.Clean(path.Dir(romPath))
	layout, _ := f.Def.Saves.Layout.Pick(f.Version)
	if layout.String() == "beside-roms" {
		return []SaveLocation{f.location(romDir, "definition", false)}, nil
	}

	var defs []string
	if row, ok := f.Rows()[slug]; ok {
		for _, s := range row.Saves {
			defs = append(defs, path.Join(f.Saves, s))
		}
	}
	defs = f.tagFirst(defs, path.Base(romDir))

	var hookDirs []string
	preferred := false
	if name := f.Def.Saves.ExtraFoldersHook; name != "" {
		folders, pref := f.reg.Folders[name](f.Host, path.Base(romDir))
		preferred = pref
		for _, d := range folders {
			if !path.IsAbs(d) {
				d = path.Join(f.Saves, d)
			}
			hookDirs = append(hookDirs, path.Clean(d))
		}
	}

	var dropped []Dropped
	var overrideDir string
	if v, ok := f.Overrides.SaveFolders[slug]; ok && v != "" {
		dir := path.Join(f.Saves, v)
		if _, err := f.Host.Stat(dir); err != nil {
			dropped = append(dropped, Dropped{Slug: slug, Dir: dir, Reason: "override folder does not exist"})
		} else {
			overrideDir = dir
		}
	}

	var locs []SaveLocation
	seen := map[string]bool{}
	add := func(dir, source string, pref bool) {
		if seen[dir] {
			return
		}
		seen[dir] = true
		locs = append(locs, f.location(dir, source, pref))
	}
	if preferred {
		for _, d := range hookDirs {
			add(d, "hook", true)
		}
	}
	if overrideDir != "" {
		add(overrideDir, "override", false)
	}
	for _, d := range defs {
		add(d, "definition", false)
	}
	if !preferred {
		for _, d := range hookDirs {
			add(d, "hook", false)
		}
	}
	return locs, dropped
}

func (f *Firmware) location(dir, source string, preferred bool) SaveLocation {
	label := path.Base(dir)
	if name := f.Def.Saves.LabelHook; name != "" {
		label = f.reg.Label[name](relSlash(f.Saves, dir))
	}
	return SaveLocation{Dir: dir, Label: label, Source: source, Preferred: preferred}
}

func (f *Firmware) tagFirst(dirs []string, parent string) []string {
	tag := Tag(parent)
	for i, d := range dirs {
		base := path.Base(d)
		if strings.EqualFold(base, parent) || (tag != "" && strings.EqualFold(base, tag)) {
			out := make([]string, 0, len(dirs))
			out = append(out, d)
			out = append(out, dirs[:i]...)
			return append(out, dirs[i+1:]...)
		}
	}
	return dirs
}

func (f *Firmware) SaveName(romFile, dir string) string {
	if name := f.Def.Saves.NamingHook; name != "" {
		vars := map[string]string{"card": f.Card, "roms": f.Roms, "bios": f.Bios, "saves": f.Saves}
		if keep, ext, ok := f.reg.Naming[name](f.Host, vars); ok {
			return saveFileName(romFile, ext, keep)
		}
	}
	if keep, ext, ok := f.inferSaveName(romFile, dir); ok {
		return saveFileName(romFile, ext, keep)
	}
	naming, _ := f.Def.Saves.Naming.Pick(f.Version)
	if naming.String() == "keep-rom-ext" {
		return saveFileName(romFile, "sav", true)
	}
	return saveFileName(romFile, "srm", false)
}

func saveFileName(romFile, ext string, keepRomExt bool) string {
	ext = strings.TrimPrefix(ext, ".")
	if keepRomExt {
		return romFile + "." + ext
	}
	return strings.TrimSuffix(romFile, path.Ext(romFile)) + "." + ext
}

func (f *Firmware) inferSaveName(romFile, dir string) (keep bool, ext string, ok bool) {
	entries, err := f.Host.ReadDir(dir)
	if err != nil {
		return false, "", false
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		saveExt := path.Ext(name)
		if !validSaveExtensions[strings.ToLower(saveExt)] {
			continue
		}
		if !ok {
			ext = saveExt
			ok = true
		}
		stem := strings.TrimSuffix(name, saveExt)
		if inner := path.Ext(stem); inner != "" && strings.EqualFold(inner, path.Ext(romFile)) {
			return true, saveExt, true
		}
	}
	return false, ext, ok
}

func relSlash(base, target string) string {
	b := strings.Split(strings.Trim(path.Clean(base), "/"), "/")
	t := strings.Split(strings.Trim(path.Clean(target), "/"), "/")
	i := 0
	for i < len(b) && i < len(t) && b[i] == t[i] {
		i++
	}
	var parts []string
	for range b[i:] {
		parts = append(parts, "..")
	}
	parts = append(parts, t[i:]...)
	return strings.Join(parts, "/")
}
