package firmware

import (
	"fmt"
	"io/fs"
	"slices"
	"sort"
	"strings"
)

type factSpec struct {
	vars     []string
	array    bool
	enum     []string
	hook     func(string) bool
	noRange  bool
	hasHooks bool
}

type validator struct {
	d         *Definition
	known     map[string]bool
	reg       Registry
	inputs    fs.FS
	errs      []error
	upstream  bool
	versioned bool
}

var (
	styleValues  = []string{"plain", "tagged"}
	layoutValues = []string{"per-emulator", "beside-roms"}
	namingValues = []string{"retroarch", "keep-rom-ext"}
	artValues    = []string{"plain", "es-suffix"}
	formatValues = []string{"none", "emulationstation", "miyoo", "muos-text"}
	maintValues  = []string{"upstream", "community"}
)

func Validate(d *Definition, known map[string]bool, reg Registry, inputs fs.FS) []error {
	v := &validator{d: d, known: known, reg: reg, inputs: inputs, upstream: d.Maintained == "upstream"}
	v.versioned = isSet(d.Version.File) || d.Version.Hook != ""
	v.identity()
	v.version()
	v.paths()
	v.enums()
	v.saves()
	v.bios()
	v.art()
	v.gamelist()
	v.retroarch()
	v.platformSources()
	v.platforms()
	v.devices()
	v.packaging()
	return v.errs
}

func (v *validator) errorf(format string, args ...any) {
	v.errs = append(v.errs, fmt.Errorf(format, args...))
}

func isSet(f Fact) bool {
	return f.Value != nil || f.Hook != "" || f.Src != "" || f.Since != "" || f.Until != "" || len(f.Alts) > 0
}

func (v *validator) identity() {
	d := v.d
	if d.ID == "" {
		v.errorf("id: required")
	}
	if d.Name == "" {
		v.errorf("name: required")
	}
	switch {
	case d.Maintained == "":
		v.errorf("maintained: required, one of %s", strings.Join(maintValues, ", "))
	case !slices.Contains(maintValues, d.Maintained):
		v.errorf("maintained: %q is not one of %s", d.Maintained, strings.Join(maintValues, ", "))
	}
	if v.upstream {
		if d.Upstream.Repo == "" {
			v.errorf("upstream.repo: required when maintained = \"upstream\"")
		}
		if d.Upstream.Ref == "" {
			v.errorf("upstream.ref: required when maintained = \"upstream\"")
		}
	} else if d.Maintained == "community" && (d.Upstream.Repo != "" || d.Upstream.Ref != "") {
		v.errorf("upstream: not allowed when maintained = \"community\"")
	}
}

func (v *validator) version() {
	d := v.d
	if isSet(d.Version.File) && d.Version.Hook != "" {
		v.errorf("version: file and hook are mutually exclusive")
	}
	if isSet(d.Version.File) {
		v.fact("version.file", d.Version.File, factSpec{vars: []string{"card"}, noRange: true})
	}
	if d.Version.Hook != "" {
		v.nameHook("version.hook", d.Version.Hook, func(n string) bool { _, ok := v.reg.Version[n]; return ok })
	}
}

func (v *validator) nameHook(path, name string, exists func(string) bool) {
	if !exists(name) {
		v.errorf("%s: unknown hook %q", path, name)
	}
}

func (v *validator) paths() {
	p := v.d.Paths
	pathHook := func(n string) bool { _, ok := v.reg.Path[n]; return ok }
	entries := []struct {
		name string
		f    Fact
		vars []string
	}{
		{"card", p.Card, []string{}},
		{"roms", p.Roms, []string{"card"}},
		{"bios", p.Bios, []string{"card", "roms"}},
		{"saves", p.Saves, []string{"card", "roms", "bios"}},
	}
	for _, e := range entries {
		path := "paths." + e.name
		if !isSet(e.f) {
			v.errorf("%s: required", path)
			continue
		}
		v.fact(path, e.f, factSpec{vars: e.vars, hook: pathHook, hasHooks: true, noRange: e.name == "card"})
	}
	if !v.beside() {
		for i, leaf := range leaves(p.Saves) {
			if leaf.String() == "{roms}" {
				v.errorf("%s: {roms} requires saves.layout = \"beside-roms\"", leafPath("paths.saves", p.Saves, i))
			}
		}
	}
}

func (v *validator) beside() bool {
	for _, l := range leaves(v.d.Saves.Layout) {
		if l.String() == "beside-roms" {
			return true
		}
	}
	return false
}

func (v *validator) enums() {
	d := v.d
	if isSet(d.RomFolders.Style) {
		v.fact("rom_folders.style", d.RomFolders.Style, factSpec{enum: styleValues})
	}
	if isSet(d.Saves.Layout) {
		v.fact("saves.layout", d.Saves.Layout, factSpec{enum: layoutValues})
	}
	if isSet(d.Saves.Naming) {
		v.fact("saves.naming", d.Saves.Naming, factSpec{enum: namingValues})
	}
	if d.Art.Naming != "" && !slices.Contains(artValues, d.Art.Naming) {
		v.errorf("art.naming: %q is not one of %s", d.Art.Naming, strings.Join(artValues, ", "))
	}
	if d.Gamelist.Format != "" && !slices.Contains(formatValues, d.Gamelist.Format) {
		v.errorf("gamelist.format: %q is not one of %s", d.Gamelist.Format, strings.Join(formatValues, ", "))
	}
}

func (v *validator) saves() {
	s := v.d.Saves
	if s.NamingHook != "" {
		v.nameHook("saves.naming_hook", s.NamingHook, func(n string) bool { _, ok := v.reg.Naming[n]; return ok })
	}
	if s.ExtraFoldersHook != "" {
		v.nameHook("saves.extra_folders_hook", s.ExtraFoldersHook, func(n string) bool { _, ok := v.reg.Folders[n]; return ok })
	}
	if s.LabelHook != "" {
		v.nameHook("saves.label_hook", s.LabelHook, func(n string) bool { _, ok := v.reg.Label[n]; return ok })
	}
}

var biosVars = []string{"bios", "name", "rel", "save_folder"}

func (v *validator) bios() {
	if isSet(v.d.Bios.Default) {
		v.fact("bios.default", v.d.Bios.Default, factSpec{vars: biosVars, array: true})
	}
}

func (v *validator) art() {
	a := v.d.Art
	kinds := []struct {
		name string
		f    *Fact
	}{{"cover", a.Cover}, {"preview", a.Preview}, {"splash", a.Splash}, {"video", a.Video}, {"manual", a.Manual}, {"bezel", a.Bezel}}
	for _, k := range kinds {
		if k.f != nil {
			v.fact("art."+k.name, *k.f, factSpec{vars: []string{"card", "roms", "rom_dir", "rom_folder", "system"}})
		}
	}
}

func (v *validator) gamelist() {
	le := v.d.Gamelist.LauncherEntry
	if le == nil {
		return
	}
	v.template("gamelist.launcher_entry.gamelist", le.Gamelist, []string{"card", "roms"})
}

func (v *validator) retroarch() {
	r := v.d.RetroArch
	if r == nil {
		return
	}
	vars := []string{"card", "roms", "bios", "saves"}
	if r.Config == nil {
		v.errorf("retroarch.config: required")
	} else {
		v.fact("retroarch.config", *r.Config, factSpec{vars: vars})
	}
	if r.Overrides != nil {
		v.fact("retroarch.overrides", *r.Overrides, factSpec{vars: vars})
	}
}

func (v *validator) platformSources() {
	ps := v.d.PlatformSources
	if !v.upstream {
		if len(ps) > 0 {
			v.errorf("platform_sources: not allowed without upstream")
		}
		return
	}
	for _, k := range sortedKeys(ps) {
		if k != "roms" && k != "saves" {
			v.errorf("platform_sources.%s: unknown key", k)
		}
	}
	if ps["roms"] == "" {
		v.errorf("platform_sources.roms: required when maintained = \"upstream\"")
	}
	if ps["saves"] == "" && !v.beside() {
		v.errorf("platform_sources.saves: required unless saves.layout = \"beside-roms\"")
	}
}

func (v *validator) platforms() {
	beside := v.beside()
	for _, slug := range sortedKeys(v.d.Platforms) {
		row := v.d.Platforms[slug]
		path := "platforms." + slug
		if !v.known[slug] {
			v.errorf("%s: unknown platform slug", path)
		}
		if len(row.Roms) > 0 && strings.TrimSpace(row.Roms[0]) == "" {
			v.errorf("%s: roms: first entry is blank", path)
		}
		if beside && len(row.Saves) > 0 {
			v.errorf("%s: saves must be empty when saves.layout = \"beside-roms\"", path)
		}
		v.rangeOf(path, row.Since, row.Until, false)
		for i, b := range row.Bios {
			v.template(fmt.Sprintf("%s.bios[%d]", path, i), b, biosVars)
		}
	}
}

func (v *validator) devices() {
	dv := v.d.Devices
	names := map[string]bool{}
	if dv.Default != "" {
		names[dv.Default] = true
	}
	for i, r := range dv.Rules {
		path := fmt.Sprintf("devices.rules[%d]", i)
		if r.Device == "" {
			v.errorf("%s: device is required", path)
		} else {
			names[r.Device] = true
		}
		v.probe(path, r)
	}
	for _, name := range sortedKeys(dv.Inputs) {
		path := "devices.inputs." + name
		in := dv.Inputs[name]
		if !names[name] {
			v.errorf("%s: device is not in devices.rules or devices.default", path)
		}
		switch {
		case in.File == "" && in.Hook == "":
			v.errorf("%s: needs a file or a hook", path)
		case in.File != "" && in.Hook != "":
			v.errorf("%s: file and hook are mutually exclusive", path)
		case in.Hook != "":
			v.nameHook(path, in.Hook, func(n string) bool { _, ok := v.reg.Input[n]; return ok })
		default:
			if _, err := fs.Stat(v.inputs, in.File); err != nil {
				v.errorf("%s: input file %q does not exist", path, in.File)
			}
		}
	}
	for _, name := range sortedKeys(dv.Quirks) {
		if !names[name] {
			v.errorf("devices.quirks.%s: device is not in devices.rules or devices.default", name)
		}
	}
}

func (v *validator) probe(path string, r DeviceRule) {
	switch {
	case r.Env != "" && r.File != "":
		v.errorf("%s: env and file probes are mutually exclusive", path)
	case r.Env != "":
		if (r.Equals == "") == (r.Prefix == "") || r.Contains != "" {
			v.errorf("%s: env needs exactly one of equals and prefix", path)
		}
	case r.File != "":
		if r.Contains == "" || r.Equals != "" || r.Prefix != "" {
			v.errorf("%s: file needs contains", path)
		}
	default:
		v.errorf("%s: needs an env or file probe", path)
	}
}

func (v *validator) packaging() {
	p := v.d.Packaging
	if p.Asset == "" {
		v.errorf("packaging.asset: required")
	}
	if p.LaunchScript == "" {
		v.errorf("packaging.launch_script: required")
	}
	if p.InstallDepth <= 0 {
		v.errorf("packaging.install_depth: must be greater than 0")
	}
}

func leaves(f Fact) []Fact {
	if len(f.Alts) > 0 {
		return f.Alts
	}
	return []Fact{f}
}

func leafPath(path string, f Fact, i int) string {
	if len(f.Alts) > 0 {
		return fmt.Sprintf("%s[%d]", path, i)
	}
	return path
}

func (v *validator) fact(path string, f Fact, spec factSpec) {
	ls := leaves(f)
	for i, l := range ls {
		v.leaf(leafPath(path, f, i), l, spec)
	}
	for i := range f.Alts {
		for j := i + 1; j < len(f.Alts); j++ {
			if overlaps(f.Alts[i], f.Alts[j]) {
				v.errorf("%s: alternatives %d and %d have overlapping ranges", path, i, j)
			}
		}
	}
}

func overlaps(a, b Fact) bool {
	lo, hi := a.Since, a.Until
	if b.Since != "" && (lo == "" || CompareVersions(b.Since, lo) > 0) {
		lo = b.Since
	}
	if b.Until != "" && (hi == "" || CompareVersions(b.Until, hi) < 0) {
		hi = b.Until
	}
	if hi == "" || lo == "" {
		return true
	}
	return CompareVersions(lo, hi) < 0
}

func (v *validator) leaf(path string, f Fact, spec factSpec) {
	hasValue := f.Value != nil
	hasHook := f.Hook != ""
	if hasValue == hasHook {
		v.errorf("%s: exactly one of value and hook is required", path)
	}
	if hasHook {
		switch {
		case !spec.hasHooks:
			v.errorf("%s: hook %q not allowed, this field accepts values only", path, f.Hook)
		case !spec.hook(f.Hook):
			v.errorf("%s: unknown hook %q", path, f.Hook)
		}
	}
	if hasValue {
		v.value(path, f, spec)
	}
	if v.upstream && hasValue && f.Src == "" {
		v.errorf("%s: src is required for upstream definitions", path)
	}
	if !v.upstream && f.Src != "" {
		v.errorf("%s: src is not allowed unless maintained = \"upstream\"", path)
	}
	v.rangeOf(path, f.Since, f.Until, spec.noRange)
}

func (v *validator) value(path string, f Fact, spec factSpec) {
	var strs []string
	switch x := f.Value.(type) {
	case string:
		strs = []string{x}
	case []string:
		if !spec.array {
			v.errorf("%s: value must be a single string", path)
			return
		}
		strs = x
	default:
		v.errorf("%s: value must be a string", path)
		return
	}
	for _, s := range strs {
		if s == "" {
			v.errorf("%s: value is empty", path)
			continue
		}
		if spec.enum != nil && !slices.Contains(spec.enum, s) {
			v.errorf("%s: %q is not one of %s", path, s, strings.Join(spec.enum, ", "))
		}
		if spec.enum == nil {
			v.template(path, s, spec.vars)
		}
	}
}

func (v *validator) template(path, tmpl string, allowed []string) {
	for _, name := range Variables(tmpl) {
		if !slices.Contains(allowed, name) {
			v.errorf("%s: template variable {%s} is not available here", path, name)
		}
	}
}

func (v *validator) rangeOf(path, since, until string, noRange bool) {
	if since == "" && until == "" {
		return
	}
	switch {
	case noRange:
		v.errorf("%s: a version range is not allowed here", path)
	case !v.versioned:
		v.errorf("%s: since and until require [version] to be set", path)
	}
	if since != "" && until != "" && CompareVersions(since, until) >= 0 {
		v.errorf("%s: version range is empty, since %q is not before until %q", path, since, until)
	}
}

func sortedKeys[M ~map[string]V, V any](m M) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
