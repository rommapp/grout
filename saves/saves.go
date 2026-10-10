package saves

import (
	"path/filepath"
	"strings"
)

// ValidSaveExtensions contains file extensions recognized as save files across
// emulators commonly found on supported CFWs (RetroArch cores and standalone).
// This is needed since some CFWs keep the saves alongside the ROMs.
var ValidSaveExtensions = map[string]bool{
	// Universal / RetroArch
	".srm": true, // RetroArch standard (SRAM dump)
	".sav": true, // Most standalone emulators

	// Nintendo DS
	".dsv": true, // DeSmuME

	// PlayStation 1
	".mcr": true, // Mednafen, Beetle PSX, ePSXe
	".mcd": true, // DuckStation

	// Sega CD / Mega CD
	".brm": true, // Genesis Plus GX (backup RAM)

	// N64 (standalone Mupen64Plus)
	".eep": true, // EEPROM
	".sra": true, // SRAM
	".fla": true, // FlashRAM
	".mpk": true, // Controller Pak

	// Arcade / MAME / FBNeo
	".nv": true, // NVRAM
}

// PlatformSaveSuffixes lists multi-part save suffixes for platforms whose emulator
// appends a tag between the ROM basename and the extension. They are matched before
// ValidSaveExtensions, and only for their platform, so a generic extension like .bin
// (also a ROM track extension) is never treated as a save elsewhere.
var PlatformSaveSuffixes = map[string][]string{
	// Flycast per-content VMUs: <rom>.A1.bin ... <rom>.D1.bin, one card per controller
	// port. Only port A1 holds real data, B1-D1 are blank formatted cards (issue #254).
	"dc": {".A1.bin"},
}

// splitSaveName splits a save filename into the name the emulator derived from the ROM
// and its save suffix (".A1.bin", ".srm", ...). ok is false when name is not a save
// file for fsSlug. Platform suffixes are matched case-insensitively.
func splitSaveName(fsSlug, name string) (base, suffix string, ok bool) {
	lower := strings.ToLower(name)
	for _, s := range PlatformSaveSuffixes[fsSlug] {
		if len(name) > len(s) && strings.HasSuffix(lower, strings.ToLower(s)) {
			return name[:len(name)-len(s)], name[len(name)-len(s):], true
		}
	}
	ext := filepath.Ext(name)
	if !ValidSaveExtensions[strings.ToLower(ext)] {
		return "", "", false
	}
	return strings.TrimSuffix(name, ext), ext, true
}

// platformSaveSuffix returns the platform save suffix whose final extension is
// serverExt (without its leading dot), so a save downloaded from RomM, which only
// reports the last extension ("bin"), is written back under the full suffix the
// emulator expects ("A1.bin").
func platformSaveSuffix(fsSlug, serverExt string) (string, bool) {
	for _, s := range PlatformSaveSuffixes[fsSlug] {
		if strings.EqualFold(strings.TrimPrefix(filepath.Ext(s), "."), serverExt) {
			return strings.TrimPrefix(s, "."), true
		}
	}
	return "", false
}
