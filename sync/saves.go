package sync

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
