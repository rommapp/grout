package muos

import (
	"encoding/json"
	"maps"
	"path"
	"slices"
	"strings"

	"grout/core/firmware/host"
)

const (
	contentConfigDir   = "/opt/muos/share/info/content"
	userManifestDir    = "/run/muos/storage/info/manifest"
	bundledManifestDir = "/opt/muos/share/info/manifest"
	picklesSaveRoot    = "../pickles/sram"
	picklesTag         = "mu-"
	externalTag        = "ext-"
	libretroSuffix     = "_libretro.so"
)

type coreEntry struct {
	Core string `json:"core"`
}

type coreSystem struct {
	Default  string               `json:"default"`
	Runtime  string               `json:"runtime"`
	Libretro map[string]coreEntry `json:"libretro"`
	External map[string]coreEntry `json:"external"`
}

type coreManifest map[string]coreSystem

func PicklesFolders(h host.Host, romFolder string) ([]string, bool) {
	manifest := readManifest(h)
	if manifest == nil {
		return nil, false
	}

	system, coreFile, tagged := readContentConfig(h, romFolder)
	if system == "" {
		system = assignedSystem(h, romFolder)
		entry, ok := manifest[system]
		if !ok {
			return nil, false
		}
		coreFile, tagged, ok = defaultCore(entry)
		if !ok {
			return nil, false
		}
	}

	onPickles := len(tagged) >= len(picklesTag) && strings.EqualFold(tagged[:len(picklesTag)], picklesTag)

	var folders []string
	add := func(file string) {
		name, ok := strings.CutSuffix(path.Base(file), libretroSuffix)
		if !ok || name == "" {
			return
		}
		folder := path.Join(picklesSaveRoot, name, romFolder)
		if !slices.Contains(folders, folder) {
			folders = append(folders, folder)
		}
	}

	if onPickles {
		add(coreFile)
	}
	cores := manifest[system].Libretro
	for _, key := range slices.Sorted(maps.Keys(cores)) {
		add(cores[key].Core)
	}
	if !onPickles {
		add(coreFile)
	}
	return folders, onPickles && len(folders) > 0
}

func defaultCore(s coreSystem) (coreFile, tagged string, ok bool) {
	runtimes := []string{"pickles", "retroarch", "external"}
	preferred := strings.ToLower(s.Runtime)
	if !slices.Contains(runtimes, preferred) {
		preferred = "pickles"
	}
	order := append([]string{preferred}, slices.DeleteFunc(slices.Clone(runtimes), func(r string) bool { return r == preferred })...)
	for _, runtime := range order {
		group := s.Libretro
		if runtime == "external" {
			group = s.External
		}
		entry, found := group[s.Default]
		if !found {
			continue
		}
		switch runtime {
		case "pickles":
			return entry.Core, picklesTag + s.Default, true
		case "external":
			return entry.Core, externalTag + s.Default, true
		default:
			return entry.Core, s.Default, true
		}
	}
	return "", "", false
}

func readContentConfig(h host.Host, romFolder string) (system, coreFile, tagged string) {
	data, err := h.ReadFile(path.Join(contentConfigDir, romFolder, "core.cfg"))
	if err != nil {
		return "", "", ""
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if len(lines) < 5 {
		return "", "", ""
	}
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}
	return lines[1], lines[0], lines[4]
}

func assignedSystem(h host.Host, romFolder string) string {
	data, err := h.ReadFile(path.Join(userManifestDir, "assign.json"))
	if err != nil {
		return ""
	}
	var assign map[string]string
	if json.Unmarshal(data, &assign) != nil {
		return ""
	}
	key := strings.ToLower(path.Base(romFolder))
	key = strings.NewReplacer(" ", "", "-", "", "_", "", "+", "").Replace(key)
	return assign[key]
}

func readManifest(h host.Host) coreManifest {
	for _, dir := range []string{userManifestDir, bundledManifestDir} {
		data, err := h.ReadFile(path.Join(dir, "core.json"))
		if err != nil {
			continue
		}
		var manifest coreManifest
		if json.Unmarshal(data, &manifest) == nil {
			return manifest
		}
	}
	return nil
}
