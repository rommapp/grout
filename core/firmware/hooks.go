package firmware

import (
	"github.com/rommapp/grout/core/firmware/host"
	"github.com/rommapp/grout/core/firmware/leaf"
	"github.com/rommapp/grout/core/firmware/muos"
	"github.com/rommapp/grout/core/firmware/nextui"
	"github.com/rommapp/grout/core/firmware/spruce"
)

type Overrides struct {
	RomFolders     map[string]string
	SaveFolders    map[string]string
	MuOSRomStorage string
}

type PathHook func(h host.Host, vars map[string]string) (string, error)

type VersionHook func(h host.Host) (string, error)

type NamingHook func(h host.Host, vars map[string]string) (keepRomExt bool, ext string, ok bool)

type FoldersHook func(h host.Host, romFolder string) (folders []string, preferred bool)

type LabelHook func(saveFolder string) string

type InputHook func(h host.Host) ([]byte, error)

type Registry struct {
	Path    map[string]PathHook
	Version map[string]VersionHook
	Naming  map[string]NamingHook
	Folders map[string]FoldersHook
	Label   map[string]LabelHook
	Input   map[string]InputHook
}

var hooks = Registry{
	Path: map[string]PathHook{
		"leaf_card_root": leaf.CardRoot,
		"muos_rom_root":  muos.RomRoot,
	},
	Naming: map[string]NamingHook{
		"nextui_save_format": nextui.SaveFormat,
	},
	Folders: map[string]FoldersHook{
		"muos_pickles":      muos.PicklesFolders,
		"spruce_arch_saves": spruce.ArchSaves,
	},
	Label: map[string]LabelHook{
		"muos_label": muos.Label,
	},
	Input: map[string]InputHook{
		"muos_sdl_map_input": muos.SDLMapInput,
	},
}

func Hooks() Registry {
	return hooks
}

func MergeHooks(base, extra Registry) Registry {
	return Registry{
		Path:    mergeMap(base.Path, extra.Path),
		Version: mergeMap(base.Version, extra.Version),
		Naming:  mergeMap(base.Naming, extra.Naming),
		Folders: mergeMap(base.Folders, extra.Folders),
		Label:   mergeMap(base.Label, extra.Label),
		Input:   mergeMap(base.Input, extra.Input),
	}
}

func mergeMap[V any](base, extra map[string]V) map[string]V {
	out := make(map[string]V, len(base)+len(extra))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}
