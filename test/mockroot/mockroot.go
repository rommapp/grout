package mockroot

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/rommapp/grout/core/firmware"
	"github.com/rommapp/grout/core/firmware/host"
)

type Options struct {
	Firmware  string
	Device    string
	Platforms []string
	Force     bool
}

func Build(dir string, o Options) (map[string]string, error) {
	d, err := firmware.Load(o.Firmware)
	if err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	abs = filepath.Clean(abs)

	if entries, err := os.ReadDir(abs); err == nil {
		if len(entries) > 0 && !o.Force {
			return nil, fmt.Errorf("mockroot: %s is not empty, use force to build into it", abs)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	if o.Device == "" {
		o.Device = d.Devices.Default
	}
	if err := checkDevice(d, o.Device); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, err
	}
	mock, err := firmware.MockFS(o.Firmware)
	if err != nil {
		return nil, err
	}
	if err := copyTree(mock, abs); err != nil {
		return nil, err
	}

	env, err := firmware.MockEnv(o.Firmware)
	if err != nil {
		return nil, err
	}
	env["CFW"] = strings.ToUpper(o.Firmware)
	env["GROUT_ROOT"] = abs

	if o.Force {
		if err := clearOtherProbes(d, abs, o.Device); err != nil {
			return nil, err
		}
	}
	if err := selectDevice(d, abs, o.Device, env); err != nil {
		return nil, err
	}

	lookup := func(k string) string { return env[k] }
	f, err := firmware.Resolve(d, host.OS(lookup), firmware.Overrides{})
	if err != nil {
		return nil, err
	}
	if f.Device != o.Device {
		return nil, fmt.Errorf("mockroot: built root resolves to device %q, want %q", f.Device, o.Device)
	}

	for _, p := range []string{f.Card, f.Roms, f.Bios, f.Saves} {
		if err := mkdir(abs, p); err != nil {
			return nil, err
		}
	}

	want := map[string]bool{}
	for _, s := range o.Platforms {
		want[s] = true
	}
	for slug := range f.Rows() {
		if len(want) > 0 && !want[slug] {
			continue
		}
		dirs := f.RomDirs(slug)
		if len(dirs) == 0 {
			continue
		}
		if err := mkdir(abs, dirs[0]); err != nil {
			return nil, err
		}
		locs, _ := f.SaveLocations(slug, path.Join(dirs[0], "x"))
		for _, l := range locs {
			if l.Source != "definition" && l.Source != "override" {
				continue
			}
			if err := mkdir(abs, l.Dir); err != nil {
				return nil, err
			}
		}
	}
	return env, nil
}

func checkDevice(d *firmware.Definition, device string) error {
	var names []string
	seen := map[string]bool{}
	add := func(n string) {
		if !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	for _, r := range d.Devices.Rules {
		add(r.Device)
	}
	add(d.Devices.Default)
	if !seen[device] {
		return fmt.Errorf("mockroot: unknown device %q, defined devices: %s", device, strings.Join(names, ", "))
	}
	return nil
}

func clearOtherProbes(d *firmware.Definition, root, device string) error {
	for _, r := range d.Devices.Rules {
		if r.Device == device || r.File == "" || r.Contains == "" {
			continue
		}
		p := filepath.Join(root, filepath.FromSlash(path.Clean(r.File)))
		if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return nil
}

func selectDevice(d *firmware.Definition, root, device string, env map[string]string) error {
	for _, r := range d.Devices.Rules {
		if r.Device != device {
			continue
		}
		switch {
		case r.Env != "" && r.Equals != "":
			env[r.Env] = r.Equals
		case r.Env != "" && r.Prefix != "":
			env[r.Env] = r.Prefix
		case r.File != "" && r.Contains != "":
			p := filepath.Join(root, filepath.FromSlash(path.Clean(r.File)))
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				return err
			}
			return os.WriteFile(p, []byte(r.Contains+"\n"), 0o644)
		}
		return nil
	}
	return nil
}

func mkdir(root, devicePath string) error {
	return os.MkdirAll(filepath.Join(root, filepath.FromSlash(path.Clean(devicePath))), 0o755)
}

func copyTree(src fs.FS, dst string) error {
	return fs.WalkDir(src, ".", func(p string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(dst, filepath.FromSlash(p))
		if e.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := fs.ReadFile(src, p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}
