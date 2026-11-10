package onion

import (
	"os"
	"path/filepath"
	"regexp"
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
