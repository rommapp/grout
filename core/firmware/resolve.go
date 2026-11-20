package firmware

import (
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/rommapp/grout/core/firmware/host"
)

type Firmware struct {
	Def       *Definition
	Host      host.Host
	Overrides Overrides
	Version   string
	Device    string
	Card      string
	Roms      string
	Bios      string
	Saves     string
	reg       Registry
}

func Resolve(d *Definition, h host.Host, ov Overrides) (*Firmware, error) {
	return ResolveWith(d, h, ov, Hooks())
}

func ResolveWith(d *Definition, h host.Host, ov Overrides, reg Registry) (*Firmware, error) {
	f := &Firmware{Def: d, Host: h, Overrides: ov, reg: reg}
	vars := map[string]string{"muos_rom_storage": ov.MuOSRomStorage}

	card, err := resolvePath("paths.card", d.Paths.Card, "", h, vars, reg)
	if err != nil {
		return nil, err
	}
	f.Card = card
	vars["card"] = card

	version, err := resolveVersion(d, h, vars, reg)
	if err != nil {
		return nil, err
	}
	f.Version = version

	for _, p := range []struct {
		name string
		fact Fact
		dst  *string
	}{
		{"roms", d.Paths.Roms, &f.Roms},
		{"bios", d.Paths.Bios, &f.Bios},
		{"saves", d.Paths.Saves, &f.Saves},
	} {
		v, err := resolvePath("paths."+p.name, p.fact, version, h, vars, reg)
		if err != nil {
			return nil, err
		}
		*p.dst = v
		vars[p.name] = v
	}

	if n := d.Saves.NamingHook; n != "" && reg.Naming[n] == nil {
		return nil, fmt.Errorf("saves.naming_hook: naming hook %q is not registered", n)
	}
	if n := d.Saves.ExtraFoldersHook; n != "" && reg.Folders[n] == nil {
		return nil, fmt.Errorf("saves.extra_folders_hook: folders hook %q is not registered", n)
	}
	if n := d.Saves.LabelHook; n != "" && reg.Label[n] == nil {
		return nil, fmt.Errorf("saves.label_hook: label hook %q is not registered", n)
	}

	if err := f.checkTemplateEnv(); err != nil {
		return nil, err
	}

	f.Device = resolveDevice(d, h)
	return f, nil
}

func (f *Firmware) checkTemplateEnv() error {
	d := f.Def
	type entry struct {
		field string
		tmpls []string
	}
	var entries []entry
	addFact := func(field string, fact *Fact) {
		if fact == nil {
			return
		}
		if picked, ok := fact.Pick(f.Version); ok && picked.Hook == "" {
			entries = append(entries, entry{field, picked.Strings()})
		}
	}
	addFact("bios.default", &d.Bios.Default)
	for _, k := range []struct {
		name string
		f    *Fact
	}{{"cover", d.Art.Cover}, {"preview", d.Art.Preview}, {"splash", d.Art.Splash}, {"video", d.Art.Video}, {"manual", d.Art.Manual}, {"bezel", d.Art.Bezel}} {
		addFact("art."+k.name, k.f)
	}
	if le := d.Gamelist.LauncherEntry; le != nil {
		entries = append(entries, entry{"gamelist.launcher_entry.gamelist", []string{le.Gamelist}})
	}
	if r := d.RetroArch; r != nil {
		addFact("retroarch.config", r.Config)
		addFact("retroarch.overrides", r.Overrides)
	}
	rows := f.Rows()
	slugs := make([]string, 0, len(rows))
	for slug := range rows {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)
	for _, slug := range slugs {
		for i, b := range rows[slug].Bios {
			entries = append(entries, entry{fmt.Sprintf("platforms.%s.bios[%d]", slug, i), []string{b}})
		}
	}
	for _, e := range entries {
		for _, t := range e.tmpls {
			for _, name := range Variables(t) {
				if !IsEnvVar(name) {
					continue
				}
				key := strings.TrimPrefix(name, envPrefix)
				if f.Host.Getenv(key) == "" {
					return fmt.Errorf("%s: environment variable %s is not set", e.field, key)
				}
			}
		}
	}
	return nil
}

func resolvePath(field string, fact Fact, version string, h host.Host, vars map[string]string, reg Registry) (string, error) {
	picked, ok := fact.Pick(version)
	if !ok {
		return "", fmt.Errorf("%s: no value for version %q", field, version)
	}
	if picked.Hook != "" {
		fn, ok := reg.Path[picked.Hook]
		if !ok {
			return "", fmt.Errorf("%s: path hook %q is not registered", field, picked.Hook)
		}
		v, err := fn(h, vars)
		if err != nil {
			return "", fmt.Errorf("%s: %w", field, err)
		}
		return v, nil
	}
	tmpl, ok := picked.Value.(string)
	if !ok || tmpl == "" {
		return "", fmt.Errorf("%s: no value", field)
	}
	v, err := ExpandEnv(tmpl, vars, h.Getenv)
	if err != nil {
		return "", fmt.Errorf("%s: %w", field, err)
	}
	return v, nil
}

func resolveVersion(d *Definition, h host.Host, vars map[string]string, reg Registry) (string, error) {
	if d.Version.Hook != "" {
		fn, ok := reg.Version[d.Version.Hook]
		if !ok {
			return "", fmt.Errorf("version.hook: version hook %q is not registered", d.Version.Hook)
		}
		v, err := fn(h)
		if err != nil {
			return "", fmt.Errorf("version.hook: %w", err)
		}
		return strings.TrimSpace(v), nil
	}
	file := d.Version.File
	if file.Value == nil && file.Hook == "" && len(file.Alts) == 0 {
		return "", nil
	}
	p, err := resolvePath("version.file", file, "", h, vars, reg)
	if err != nil {
		return "", err
	}
	data, err := h.ReadFile(p)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("version.file: %w", err)
	}
	line, _, _ := strings.Cut(string(data), "\n")
	return strings.TrimSpace(line), nil
}

func resolveDevice(d *Definition, h host.Host) string {
	for _, r := range d.Devices.Rules {
		switch {
		case r.Env != "" && r.Equals != "":
			if h.Getenv(r.Env) == r.Equals {
				return r.Device
			}
		case r.Env != "" && r.Prefix != "":
			if strings.HasPrefix(h.Getenv(r.Env), r.Prefix) {
				return r.Device
			}
		case r.File != "" && r.Contains != "":
			data, err := h.ReadFile(r.File)
			if err == nil && strings.Contains(string(data), r.Contains) {
				return r.Device
			}
		}
	}
	return d.Devices.Default
}

func (f *Firmware) Rows() map[string]Row {
	out := make(map[string]Row, len(f.Def.Platforms))
	for slug, row := range f.Def.Platforms {
		if InRange(f.Version, row.Since, row.Until) {
			out[slug] = row
		}
	}
	return out
}

func Active(getenv func(string) string) (string, error) {
	return activeIn(getenv, IDs())
}

func activeIn(getenv func(string) string, ids []string) (string, error) {
	id := strings.ToLower(strings.TrimSpace(getenv("CFW")))
	for _, k := range ids {
		if k == id && id != "" {
			return id, nil
		}
	}
	return "", fmt.Errorf("unknown firmware %q in CFW, known: %s", id, strings.Join(ids, ", "))
}
