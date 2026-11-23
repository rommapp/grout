package firmware

import (
	"fmt"
	"sort"
)

type Fact struct {
	Value any
	Src   string
	Since string
	Until string
	Hook  string
	Alts  []Fact
}

func (f Fact) String() string {
	s, _ := f.Value.(string)
	return s
}

func (f Fact) Strings() []string {
	switch v := f.Value.(type) {
	case string:
		return []string{v}
	case []string:
		return v
	}
	return nil
}

func (f *Fact) UnmarshalTOML(data any) error {
	switch v := data.(type) {
	case map[string]any:
		return f.fromTable(v)
	case []map[string]any:
		return f.fromTables(v)
	case []any:
		tables := make([]map[string]any, 0, len(v))
		for _, e := range v {
			m, ok := e.(map[string]any)
			if !ok {
				return fmt.Errorf("fact alternatives must be inline tables, got %T", e)
			}
			tables = append(tables, m)
		}
		return f.fromTables(tables)
	}
	return fmt.Errorf("fact must be an inline table or an array of inline tables, got %T", data)
}

func (f *Fact) fromTables(tables []map[string]any) error {
	alts := make([]Fact, 0, len(tables))
	for _, m := range tables {
		var a Fact
		if err := a.fromTable(m); err != nil {
			return err
		}
		alts = append(alts, a)
	}
	*f = Fact{Alts: alts}
	return nil
}

func (f *Fact) fromTable(m map[string]any) error {
	var out Fact
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := m[k]
		switch k {
		case "value":
			val, err := factValue(v)
			if err != nil {
				return err
			}
			out.Value = val
		case "src":
			s, err := factString(k, v)
			if err != nil {
				return err
			}
			out.Src = s
		case "since":
			s, err := factString(k, v)
			if err != nil {
				return err
			}
			out.Since = s
		case "until":
			s, err := factString(k, v)
			if err != nil {
				return err
			}
			out.Until = s
		case "hook":
			s, err := factString(k, v)
			if err != nil {
				return err
			}
			out.Hook = s
		default:
			return fmt.Errorf("unknown fact key %q", k)
		}
	}
	*f = out
	return nil
}

func factString(key string, v any) (string, error) {
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("fact key %q must be a string, got %T", key, v)
	}
	return s, nil
}

func factValue(v any) (any, error) {
	switch x := v.(type) {
	case string:
		return x, nil
	case []string:
		return x, nil
	case []any:
		out := make([]string, 0, len(x))
		for _, e := range x {
			s, ok := e.(string)
			if !ok {
				return nil, fmt.Errorf("fact value array must hold strings, got %T", e)
			}
			out = append(out, s)
		}
		return out, nil
	}
	return nil, fmt.Errorf("fact value must be a string or an array of strings, got %T", v)
}

type Row struct {
	Roms   []string `toml:"roms"`
	Saves  []string `toml:"saves"`
	Bios   []string `toml:"bios"`
	System string   `toml:"system"`
	Since  string   `toml:"since"`
	Until  string   `toml:"until"`
}

type DeviceRule struct {
	Device   string `toml:"device"`
	Env      string `toml:"env"`
	Equals   string `toml:"equals"`
	Prefix   string `toml:"prefix"`
	File     string `toml:"file"`
	Contains string `toml:"contains"`
}

type Quirks struct {
	Rotation                int  `toml:"rotation"`
	DisableKeyboardJoystick bool `toml:"disable_keyboard_joystick"`
}

type Packaging struct {
	Asset        string   `toml:"asset"`
	LaunchScript string   `toml:"launch_script"`
	InstallDepth int      `toml:"install_depth"`
	Files        []string `toml:"files"`
}

type Input struct {
	File string
	Hook string
}

func (i *Input) UnmarshalTOML(data any) error {
	switch v := data.(type) {
	case string:
		*i = Input{File: v}
		return nil
	case map[string]any:
		out := Input{}
		for k, val := range v {
			s, ok := val.(string)
			if k != "hook" || !ok {
				return fmt.Errorf("input must be a file name or { hook = \"...\" }, got key %q", k)
			}
			out.Hook = s
		}
		*i = out
		return nil
	}
	return fmt.Errorf("input must be a file name or { hook = \"...\" }, got %T", data)
}

type Definition struct {
	ID         string `toml:"id"`
	Name       string `toml:"name"`
	Maintained string `toml:"maintained"`

	Upstream struct {
		Repo string `toml:"repo"`
		Ref  string `toml:"ref"`
	} `toml:"upstream"`

	Version struct {
		File Fact   `toml:"file"`
		Hook string `toml:"hook"`
	} `toml:"version"`

	Paths struct {
		Card  Fact `toml:"card"`
		Roms  Fact `toml:"roms"`
		Bios  Fact `toml:"bios"`
		Saves Fact `toml:"saves"`
	} `toml:"paths"`

	RomFolders struct {
		Style       Fact   `toml:"style"`
		MultiLayout string `toml:"multi_layout"`
	} `toml:"rom_folders"`

	Saves struct {
		Layout           Fact   `toml:"layout"`
		Naming           Fact   `toml:"naming"`
		NamingHook       string `toml:"naming_hook"`
		ExtraFoldersHook string `toml:"extra_folders_hook"`
		LabelHook        string `toml:"label_hook"`
	} `toml:"saves"`

	Bios struct {
		Default Fact `toml:"default"`
	} `toml:"bios"`

	Art struct {
		Naming    string `toml:"naming"`
		Cover     *Fact  `toml:"cover"`
		Preview   *Fact  `toml:"preview"`
		Splash    *Fact  `toml:"splash"`
		Video     *Fact  `toml:"video"`
		Manual    *Fact  `toml:"manual"`
		Bezel     *Fact  `toml:"bezel"`
		Marquee   *Fact  `toml:"marquee"`
		Thumbnail *Fact  `toml:"thumbnail"`
		Boxback   *Fact  `toml:"boxback"`
		Fanart    *Fact  `toml:"fanart"`
	} `toml:"art"`

	Gamelist struct {
		Format        string `toml:"format"`
		LauncherEntry *struct {
			Gamelist string `toml:"gamelist"`
			Path     string `toml:"path"`
		} `toml:"launcher_entry"`
	} `toml:"gamelist"`

	RetroArch *struct {
		Config    *Fact `toml:"config"`
		Overrides *Fact `toml:"overrides"`
	} `toml:"retroarch"`

	PlatformSources map[string]string `toml:"platform_sources"`
	Platforms       map[string]Row    `toml:"platforms"`

	Devices struct {
		Default string            `toml:"default"`
		Rules   []DeviceRule      `toml:"rules"`
		Inputs  map[string]Input  `toml:"inputs"`
		Quirks  map[string]Quirks `toml:"quirks"`
	} `toml:"devices"`

	Packaging Packaging `toml:"packaging"`
}
