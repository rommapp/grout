package onion

import (
	"encoding/json"
	"grout/files"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// keyValue matches a `key = "value"` line, the form Onion writes its per-game
// settings and RetroArch writes its core info in.
func keyValue(text, key string) string {
	pattern := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(key) + `\s*=\s*"([^"]*)"`)
	if match := pattern.FindStringSubmatch(text); match != nil {
		return match[1]
	}
	return ""
}

// GameCore is the core a game has been given of its own (Y menu > Game core),
// such as "mgba_libretro", or "" when it uses its system's default.
//
// Onion keeps that choice beside the ROM, in .game_config/<rom name>.cfg.
func GameCore(romPath string) string {
	name := strings.TrimSuffix(filepath.Base(romPath), filepath.Ext(romPath))
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(romPath), ".game_config", name+".cfg"))
	if err != nil {
		return ""
	}
	return keyValue(string(raw), "core")
}

// SaveFolderForRom is the folder under the save root that a game's saves are in
// when it runs on a core of its own, or "" when it does not.
//
// Onion keeps each core's saves in a folder named for the core, so a game moved
// to another core reads and writes a different folder from the rest of its
// system. The name is the one in the core's info file.
func SaveFolderForRom(romPath string) string {
	core := GameCore(romPath)
	if core == "" {
		return ""
	}
	info, err := os.ReadFile(filepath.Join(GetBasePath(), "RetroArch", ".retroarch", "cores", core+".info"))
	if err != nil {
		return ""
	}
	return keyValue(string(info), "corename")
}

// Core is a core a game could run on. ID is the name Onion stores for it, such
// as "mgba_libretro"; Name is what the core calls itself, which is also the
// name of its folder of saves.
type Core struct {
	ID   string
	Name string
}

// GameCores is the cores a game could run on and which of them it runs on now.
type GameCores struct {
	Choices []Core
	// Current is the core the game launches with, its own choice or else the
	// system's default.
	Current string
	// Default is the core the system launches its games with.
	Default string
}

func coresDirectory() string {
	return filepath.Join(GetBasePath(), "RetroArch", ".retroarch", "cores")
}

// coreNamed describes an installed core, or reports that it is not installed.
func coreNamed(id string) (Core, bool) {
	if _, err := os.Stat(filepath.Join(coresDirectory(), id+".so")); err != nil {
		return Core{}, false
	}
	name := id
	if info, err := os.ReadFile(filepath.Join(coresDirectory(), id+".info")); err == nil {
		if corename := keyValue(string(info), "corename"); corename != "" {
			name = corename
		}
	}
	return Core{ID: id, Name: name}, true
}

// GameCoreChoices lists the cores a game can be given. folders are the save
// folders the system's saves can be in; only a core that keeps its saves in one
// of them is offered, because a save in any other folder is one Grout never
// finds. The core the game runs on now and the system's default are always
// included, so what the game does today is never missing from the list.
func GameCoreChoices(romPath string, folders []string) (GameCores, error) {
	entries, err := os.ReadDir(coresDirectory())
	if err != nil {
		return GameCores{}, err
	}

	result := GameCores{Default: defaultCore(emulatorDirectoryFor(romPath))}
	result.Current = GameCore(romPath)
	if result.Current == "" {
		result.Current = result.Default
	}

	seen := map[string]bool{}
	add := func(core Core) {
		if !seen[core.ID] {
			seen[core.ID] = true
			result.Choices = append(result.Choices, core)
		}
	}

	for _, entry := range entries {
		id, isInfo := strings.CutSuffix(entry.Name(), ".info")
		if !isInfo {
			continue
		}
		core, installed := coreNamed(id)
		if !installed {
			continue
		}
		for _, folder := range folders {
			if strings.EqualFold(folder, core.Name) {
				add(core)
				break
			}
		}
	}
	for _, id := range []string{result.Default, result.Current} {
		if id == "" {
			continue
		}
		if core, installed := coreNamed(id); installed {
			add(core)
		}
	}

	sort.Slice(result.Choices, func(i, j int) bool {
		return strings.ToLower(result.Choices[i].Name) < strings.ToLower(result.Choices[j].Name)
	})
	return result, nil
}

// emulatorDirectoryFor finds the Emu folder whose games live where this ROM
// does. Each system's folder says where its games are in its config.json.
func emulatorDirectoryFor(romPath string) string {
	want := canonical(filepath.Dir(romPath))
	configs, _ := filepath.Glob(filepath.Join(GetBasePath(), "Emu", "*", "config.json"))
	for _, path := range configs {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var config struct {
			RomPath string `json:"rompath"`
		}
		if json.Unmarshal(raw, &config) != nil || config.RomPath == "" {
			continue
		}
		emuDir := filepath.Dir(path)
		if canonical(filepath.Join(emuDir, config.RomPath)) == want {
			return emuDir
		}
	}
	return ""
}

func canonical(path string) string {
	path = filepath.Clean(path)
	if real, err := filepath.EvalSymlinks(path); err == nil {
		return real
	}
	return path
}

var (
	defaultCoreVariable = regexp.MustCompile(`(?m)^\s*default_core=["']?([A-Za-z0-9_\-]+)`)
	coreInCommand       = regexp.MustCompile(`\.retroarch/cores/([A-Za-z0-9_\-]+)\.so`)
)

// defaultCore is the core a system launches its games with, read from its
// launch script the way Onion's game menu does: a default_core variable, which
// names the core without its _libretro suffix, or else the core in the command.
func defaultCore(emulatorDir string) string {
	if emulatorDir == "" {
		return ""
	}
	raw, err := os.ReadFile(filepath.Join(emulatorDir, "launch.sh"))
	if err != nil {
		return ""
	}
	if match := defaultCoreVariable.FindStringSubmatch(string(raw)); match != nil {
		return match[1] + "_libretro"
	}
	if match := coreInCommand.FindStringSubmatch(string(raw)); match != nil {
		return match[1]
	}
	return ""
}

var coreSetting = regexp.MustCompile(`^\s*core\s*=`)

// SetGameCore gives a game a core of its own, or takes the choice away when
// core is empty or is the system's default, as Onion's game menu does. Other
// settings in the game's file are left as they are.
func SetGameCore(romPath, core string) error {
	name := strings.TrimSuffix(filepath.Base(romPath), filepath.Ext(romPath))
	dir := filepath.Join(filepath.Dir(romPath), ".game_config")
	path := filepath.Join(dir, name+".cfg")

	var kept []string
	if raw, err := os.ReadFile(path); err == nil {
		for _, line := range strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n") {
			if strings.TrimSpace(line) != "" && !coreSetting.MatchString(line) {
				kept = append(kept, line)
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	if core != "" && core != defaultCore(emulatorDirectoryFor(romPath)) {
		kept = append(kept, `core = "`+core+`"`)
	}

	if len(kept) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		os.Remove(dir) // only succeeds when nothing else is in it
		return nil
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return files.WriteFileAtomic(path, []byte(strings.Join(kept, "\n")+"\n"), 0644)
}
