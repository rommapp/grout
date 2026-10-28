package muos

import (
	"bufio"
	"encoding/json"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// Pickles is muOS's own libretro frontend, the default since Andromeda. It
// keeps saves apart from RetroArch's, under <storage>/pickles/sram/<core>/<rom
// folder>, beside the save root rather than in it, where <core> is the core
// file less "_libretro.so". Folders are relative to the save root, hence "../".
//
// Which runtime a rom folder uses is muOS's to say. These are the files it
// says it in, read the way its frontend reads them.
const (
	// contentConfigDir holds a core.cfg per rom folder: the core file, the
	// system, the catalogue, the lookup flag and the tagged core, one a line.
	contentConfigDir = "/opt/muos/share/info/content"
	// userManifestDir holds assign.json, rom folder to system, and may
	// override the bundled libretro.json.
	userManifestDir    = StoragePath + "/info/manifest"
	bundledManifestDir = "/opt/muos/share/info/manifest"
)

// picklesTag marks a core assigned to Pickles rather than RetroArch.
const picklesTag = "mu-"

const picklesSaveRoot = "../pickles/sram"

// PicklesSaveFolders returns the Pickles save folders a rom folder's saves
// could be in, relative to the save root, and whether Pickles is where they
// belong. It returns nothing on a muOS without Pickles.
func PicklesSaveFolders(romFolder string) ([]string, bool) {
	if os.Getenv("BASE_PATH") != "" {
		return nil, false
	}
	return picklesSaveFolders("/", romFolder)
}

type coreManifest map[string]struct {
	Default string `json:"default"`
	Cores   map[string]struct {
		Core string `json:"core"`
	} `json:"cores"`
}

func picklesSaveFolders(root, romFolder string) ([]string, bool) {
	manifest := readManifest(root)

	system, coreFile, tagged := readContentConfig(root, romFolder)
	if system == "" {
		// muOS assigns a folder the first time it is browsed, trying Pickles
		// first with the system's default core. One it has not seen yet will
		// end up there.
		system = assignedSystem(root, romFolder)
		entry, ok := manifest[system]
		if !ok {
			return nil, false
		}
		coreFile = entry.Cores[entry.Default].Core
		tagged = picklesTag + entry.Default
	}

	onPickles := strings.HasPrefix(tagged, picklesTag)

	var folders []string
	add := func(file string) {
		name := strings.TrimSuffix(file, "_libretro.so")
		if name == "" {
			return
		}
		folder := path.Join(picklesSaveRoot, name, filepath.ToSlash(romFolder))
		for _, existing := range folders {
			if existing == folder {
				return
			}
		}
		folders = append(folders, folder)
	}

	if onPickles {
		add(coreFile)
	}
	cores := manifest[system].Cores
	for _, key := range slices.Sorted(maps.Keys(cores)) {
		add(cores[key].Core)
	}
	if !onPickles {
		add(coreFile)
	}
	return folders, onPickles && len(folders) > 0
}

// readContentConfig reads a rom folder's core.cfg, returning the system, the
// core file and the tagged core, or empty strings when muOS has not assigned
// the folder yet.
func readContentConfig(root, romFolder string) (system, coreFile, tagged string) {
	file, err := os.Open(filepath.Join(root, contentConfigDir, romFolder, "core.cfg"))
	if err != nil {
		return "", "", ""
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, strings.TrimSpace(scanner.Text()))
	}
	if len(lines) < 5 {
		return "", "", ""
	}
	return lines[1], lines[0], lines[4]
}

// assignedSystem looks a rom folder up in assign.json by muOS's own key: the
// folder's name in lowercase without spaces, dashes, underscores or pluses.
func assignedSystem(root, romFolder string) string {
	data, err := os.ReadFile(filepath.Join(root, userManifestDir, "assign.json"))
	if err != nil {
		return ""
	}
	var assign map[string]string
	if json.Unmarshal(data, &assign) != nil {
		return ""
	}

	key := strings.ToLower(filepath.Base(romFolder))
	key = strings.NewReplacer(" ", "", "-", "", "_", "", "+", "").Replace(key)
	return assign[key]
}

// readManifest reads libretro.json, preferring the user's copy as muOS does.
func readManifest(root string) coreManifest {
	for _, dir := range []string{userManifestDir, bundledManifestDir} {
		data, err := os.ReadFile(filepath.Join(root, dir, "libretro.json"))
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
