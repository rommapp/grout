package constants

var LibretroCoreToBIOS = mustLoadJSONMap[string, CoreBIOS]("bios/core_requirements.json")
var PlatformToLibretroCores = mustLoadJSONMap[string, []string]("bios/platform_cores.json")

// BIOSFile represents a single BIOS/firmware file requirement
type BIOSFile struct {
	FileName     string // e.g., "gba_bios.bin"
	RelativePath string // e.g., "gba_bios.bin" or "psx/scph5500.bin"
	MD5Hash      string // e.g., "a860e8c0b6d573d191e4ec7db1b1e4f6" (optional, empty string if unknown)
	Optional     bool   // true if BIOS file is optional for the emulator to function
}

// CoreBIOS represents all BIOS requirements for a Libretro core
type CoreBIOS struct {
	CoreName    string     // e.g., "mgba_libretro"
	DisplayName string     // e.g., "Nintendo - Game Boy Advance (mGBA)"
	Files       []BIOSFile // List of BIOS files for this core
}

// CoreBIOSSubdirectories maps Libretro core names (without _libretro suffix)
// to their required BIOS subdirectory within the system BIOS directory.
// Cores not in this map use the root BIOS directory.
var CoreBIOSSubdirectories = mustLoadJSONMap[string, string]("bios/core_subdirectories.json")
