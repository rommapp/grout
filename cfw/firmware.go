package cfw

import (
	"grout/cfw/allium"
	"grout/cfw/arkos"
	"grout/cfw/batocera"
	"grout/cfw/knulli"
	"grout/cfw/koriki"
	"grout/cfw/minui"
	"grout/cfw/muos"
	"grout/cfw/nextui"
	"grout/cfw/onion"
	"grout/cfw/rocknix"
	"grout/cfw/spruce"
	"grout/cfw/trimui"
)

// Firmware describes one custom firmware: where it keeps things, and which of
// them it has at all.
//
// A nil function or map means the firmware has no such thing. Callers treat an
// empty result as "skip", never as an error.
type Firmware struct {
	id CFW

	romDirectory  func() string
	biosDirectory func() string
	baseSavePath  func() string

	// coverDirectory takes both a rom directory and a platform because the two
	// families disagree: most firmwares keep art beside the roms, muOS and
	// TrimUI keep a catalogue keyed by platform.
	coverDirectory func(romDir, platformFSSlug, platformName string) string

	// sidecarDirectories is nil outside the EmulationStation family.
	sidecarDirectories func(romDir string) (video, manual, bezel string)

	// previewDirectory and splashDirectory are muOS-only.
	previewDirectory func(platformFSSlug, platformName string) string
	splashDirectory  func(platformFSSlug, platformName string) string

	// biosFilePaths, when set, replaces the default single BIOS location.
	biosFilePaths func(relativePath, platformFSSlug string) []string

	// romFolderBase strips folder decorations before matching. MinUI and NextUI
	// allow "Game Boy Advance (GBA)" and match on the tag.
	romFolderBase func(path string, tagParser func(string) string) string

	// platforms maps a RomM filesystem slug to the rom folders this firmware
	// accepts.
	platforms map[string][]string

	// saveDirectories maps a slug to emulator save folders, most preferred
	// first, and is nil when savesBesideRoms is set.
	saveDirectories map[string][]string

	// savesBesideRoms means the platform table doubles as the save table.
	savesBesideRoms bool

	// groutGamelist, when set, is where the launcher shortcut entry goes.
	groutGamelist func() string
	// groutLauncherPath is the command that entry runs; it follows the
	// firmware's package layout.
	groutLauncherPath string

	// gamelist is how this firmware expects game metadata to be written.
	gamelist GamelistFormat

	// emulationStationBased implies the gamelist format, the sidecar
	// directories and the art filename suffixes.
	emulationStationBased bool

	// inputMapping is the firmware's controller mapping, embedded in its
	// package. nil for firmwares that use gabagool's default.
	inputMapping func() ([]byte, error)

	// packaging describes the release archive.
	packaging Packaging

	// display, when set, reports quirks of the specific device this firmware
	// is running on.
	display func() DisplayQuirks

	// keepsRomExtInSaves names saves after the whole rom file rather than
	// stripping the extension as RetroArch does. Fallback only, when the
	// convention cannot be read off saves already on the device (issue #245).
	keepsRomExtInSaves bool
}

func (f *Firmware) IsBasedOnEmulationStation() bool {
	return f != nil && f.emulationStationBased
}

func (f *Firmware) KeepsRomExtInSaves() bool { return f != nil && f.keepsRomExtInSaves }

// DisplayQuirks describes handling a particular device needs. Rotation is in
// degrees rather than a toolkit enum so this package stays free of the UI.
type DisplayQuirks struct {
	// RotationDegrees the framebuffer needs, clockwise.
	RotationDegrees int
	// DisableKeyboardAndJoystick suppresses input sources that produce
	// spurious events on some devices.
	DisableKeyboardAndJoystick bool
}

// Display reports quirks of the device this firmware is running on.
func (f *Firmware) Display() DisplayQuirks {
	if f == nil || f.display == nil {
		return DisplayQuirks{}
	}
	return f.display()
}

// Packaging describes a firmware's release archive. The asset names must match
// what .github/workflows/release.yml publishes, or in-app update silently does
// nothing.
type Packaging struct {
	// Asset names the release zip, or is empty when the firmware ships one per
	// architecture in ArchAssets.
	Asset string
	// ArchAssets names the release zip per GOARCH.
	ArchAssets map[string]string
	// LaunchScript is the path inside the archive of the script the frontend
	// runs.
	LaunchScript string
	// InstallDepth is how many directories up from the binary the archive's
	// extraction root sits.
	InstallDepth int
}

// AssetName returns the release zip for goarch, or "" if this firmware has no
// build for it.
func (p Packaging) AssetName(goarch string) string {
	if p.ArchAssets != nil {
		return p.ArchAssets[goarch]
	}
	return p.Asset
}

// Packaging describes this firmware's release archive.
func (f *Firmware) Packaging() Packaging {
	if f == nil {
		return Packaging{}
	}
	return f.packaging
}

// InputMapping returns the firmware's embedded controller mapping. A nil slice
// means it uses the toolkit default.
func (f *Firmware) InputMapping() ([]byte, error) {
	if f == nil || f.inputMapping == nil {
		return nil, nil
	}
	return f.inputMapping()
}

// GamelistFormat is how a firmware expects game metadata to be recorded.
type GamelistFormat int

const (
	GamelistNone GamelistFormat = iota
	// GamelistEmulationStation is a gamelist.xml, reloaded on frontend restart.
	GamelistEmulationStation
	// GamelistMiyoo is a miyoogamelist.xml, read on launch.
	GamelistMiyoo
	// GamelistMuOSText is one text file per game in muOS's catalogue.
	GamelistMuOSText
)

func (f *Firmware) Gamelist() GamelistFormat {
	if f == nil {
		return GamelistNone
	}
	return f.gamelist
}

// GroutLauncherPath is the command the shortcut entry runs.
func (f *Firmware) GroutLauncherPath() string {
	if f == nil {
		return ""
	}
	return f.groutLauncherPath
}

// ID is the value the launch script puts in the CFW environment variable.
func (f *Firmware) ID() CFW {
	if f == nil {
		return ""
	}
	return f.id
}

// besideRoms adapts a firmware that keeps artwork in the rom directory.
func besideRoms(dir func(romDir string) string) func(string, string, string) string {
	return func(romDir, _, _ string) string { return dir(romDir) }
}

// inCatalogue adapts a firmware that keeps artwork in a catalogue keyed by
// platform rather than beside the roms.
func inCatalogue(dir func(platformFSSlug, platformName string) string) func(string, string, string) string {
	return func(_, platformFSSlug, platformName string) string {
		return dir(platformFSSlug, platformName)
	}
}

// esSidecars builds the video/manual/bezel resolver for the EmulationStation
// family, whose members differ only in which package the functions come from.
func esSidecars(video, manual, bezel func(string) string) func(string) (string, string, string) {
	return func(romDir string) (string, string, string) {
		return video(romDir), manual(romDir), bezel(romDir)
	}
}

// firmwares describes every supported device. A literal rather than init-time
// registration: written once, never mutated, no ordering rules.
var firmwares = map[CFW]*Firmware{
	MuOS: {
		id:               MuOS,
		romDirectory:     muos.GetRomDirectory,
		biosDirectory:    muos.GetBIOSDirectory,
		baseSavePath:     muos.GetBaseSavePath,
		coverDirectory:   inCatalogue(muos.GetArtDirectory),
		previewDirectory: muos.GetPreviewDirectory,
		splashDirectory:  muos.GetSplashDirectory,
		platforms:        muos.Platforms,
		saveDirectories:  muos.SaveDirectories,
		gamelist:         GamelistMuOSText,
		inputMapping:     muos.GetInputMappingBytes,
		packaging:        Packaging{Asset: "Grout.muxapp", LaunchScript: "Grout/mux_launch.sh", InstallDepth: 2},
	},
	NextUI: {
		id:                 NextUI,
		romDirectory:       nextui.GetRomDirectory,
		biosDirectory:      nextui.GetBIOSDirectory,
		baseSavePath:       nextui.GetBaseSavePath,
		coverDirectory:     besideRoms(nextui.GetArtDirectory),
		biosFilePaths:      nextui.GetBIOSFilePaths,
		romFolderBase:      nextui.RomFolderBase,
		platforms:          nextui.Platforms,
		saveDirectories:    nextui.SaveDirectories,
		keepsRomExtInSaves: true,
		packaging:          Packaging{Asset: "Grout.pak.zip", LaunchScript: "launch.sh", InstallDepth: 1},
		display:            nextuiDisplay,
	},
	MinUI: {
		id:                 MinUI,
		romDirectory:       minui.GetRomDirectory,
		biosDirectory:      minui.GetBIOSDirectory,
		baseSavePath:       minui.GetBaseSavePath,
		coverDirectory:     besideRoms(minui.GetArtDirectory),
		biosFilePaths:      minui.GetBIOSFilePaths,
		romFolderBase:      minui.RomFolderBase,
		platforms:          minui.Platforms,
		saveDirectories:    minui.SaveDirectories,
		keepsRomExtInSaves: true,
		inputMapping:       minui.GetInputMappingBytes,
		packaging:          Packaging{Asset: "Grout-MinUI.zip", LaunchScript: "Grout.pak/launch.sh", InstallDepth: 2},
		display:            minuiDisplay,
	},
	Trimui: {
		id:              Trimui,
		romDirectory:    trimui.GetRomDirectory,
		biosDirectory:   trimui.GetBIOSDirectory,
		baseSavePath:    trimui.GetBaseSavePath,
		coverDirectory:  inCatalogue(trimui.GetArtDirectory),
		platforms:       trimui.Platforms,
		saveDirectories: trimui.SaveDirectories,
		packaging:       Packaging{Asset: "Grout-Trimui.zip", LaunchScript: "Grout/launch.sh", InstallDepth: 3},
	},

	// The Miyoo family: art beside the roms, saves in their own tree.
	Spruce: {
		id:              Spruce,
		romDirectory:    spruce.GetRomDirectory,
		biosDirectory:   spruce.GetBIOSDirectory,
		baseSavePath:    spruce.GetBaseSavePath,
		coverDirectory:  besideRoms(spruce.GetArtDirectory),
		platforms:       spruce.Platforms,
		saveDirectories: spruce.SaveDirectories,
		gamelist:        GamelistMiyoo,
		inputMapping:    spruce.GetInputMappingBytes,
		packaging:       Packaging{Asset: "Grout.spruce.zip", LaunchScript: "Grout/launch.sh", InstallDepth: 3},
		display:         spruceDisplay,
	},
	Allium: {
		id:              Allium,
		romDirectory:    allium.GetRomDirectory,
		biosDirectory:   allium.GetBIOSDirectory,
		baseSavePath:    allium.GetBaseSavePath,
		coverDirectory:  besideRoms(allium.GetArtDirectory),
		platforms:       allium.Platforms,
		saveDirectories: allium.SaveDirectories,
		gamelist:        GamelistMiyoo,
		inputMapping:    allium.GetInputMappingBytes,
		packaging:       Packaging{Asset: "Grout-Allium.zip", LaunchScript: "Grout.pak/launch.sh", InstallDepth: 3},
	},
	Onion: {
		id:              Onion,
		romDirectory:    onion.GetRomDirectory,
		biosDirectory:   onion.GetBIOSDirectory,
		baseSavePath:    onion.GetBaseSavePath,
		coverDirectory:  besideRoms(onion.GetArtDirectory),
		platforms:       onion.Platforms,
		saveDirectories: onion.SaveDirectories,
		gamelist:        GamelistMiyoo,
		inputMapping:    onion.GetInputMappingBytes,
		packaging:       Packaging{Asset: "Grout-Onion.zip", LaunchScript: "Grout/launch.sh", InstallDepth: 3},
	},
	Koriki: {
		id:              Koriki,
		romDirectory:    koriki.GetRomDirectory,
		biosDirectory:   koriki.GetBIOSDirectory,
		baseSavePath:    koriki.GetBaseSavePath,
		coverDirectory:  besideRoms(koriki.GetArtDirectory),
		platforms:       koriki.Platforms,
		saveDirectories: koriki.SaveDirectories,
		gamelist:        GamelistMiyoo,
		inputMapping:    koriki.GetInputMappingBytes,
		packaging:       Packaging{Asset: "Grout-Koriki.zip", LaunchScript: "Grout/launch.sh", InstallDepth: 3},
	},

	// The EmulationStation family: separate directories per art kind, and a
	// launcher shortcut written into a gamelist.
	Knulli: {
		id:                    Knulli,
		romDirectory:          knulli.GetRomDirectory,
		biosDirectory:         knulli.GetBIOSDirectory,
		baseSavePath:          knulli.GetBaseSavePath,
		coverDirectory:        besideRoms(knulli.GetArtDirectory),
		sidecarDirectories:    esSidecars(knulli.GetVideoDirectory, knulli.GetManualDirectory, knulli.GetBezelDirectory),
		groutGamelist:         knulli.GetGroutGamelist,
		platforms:             knulli.Platforms,
		saveDirectories:       knulli.SaveDirectories,
		gamelist:              GamelistEmulationStation,
		groutLauncherPath:     "./Grout/Grout.sh",
		emulationStationBased: true,
		packaging:             Packaging{Asset: "Grout-Knulli.zip", LaunchScript: "Grout/Grout.sh", InstallDepth: 2},
	},
	ROCKNIX: {
		id:                    ROCKNIX,
		romDirectory:          rocknix.GetRomDirectory,
		biosDirectory:         rocknix.GetBIOSDirectory,
		baseSavePath:          rocknix.GetBaseSavePath,
		coverDirectory:        besideRoms(rocknix.GetArtDirectory),
		sidecarDirectories:    esSidecars(rocknix.GetVideoDirectory, rocknix.GetManualDirectory, rocknix.GetBezelDirectory),
		groutGamelist:         rocknix.GetGroutGamelist,
		platforms:             rocknix.Platforms,
		savesBesideRoms:       true,
		gamelist:              GamelistEmulationStation,
		groutLauncherPath:     "./Grout.sh",
		emulationStationBased: true,
		inputMapping:          rocknix.GetInputMappingBytes,
		packaging:             Packaging{Asset: "Grout-ROCKNIX.zip", LaunchScript: "Grout.sh", InstallDepth: 2},
	},
	ArkOS: {
		id:                    ArkOS,
		romDirectory:          arkos.GetRomDirectory,
		biosDirectory:         arkos.GetBIOSDirectory,
		baseSavePath:          arkos.GetBaseSavePath,
		coverDirectory:        besideRoms(arkos.GetArtDirectory),
		sidecarDirectories:    esSidecars(arkos.GetVideoDirectory, arkos.GetManualDirectory, arkos.GetBezelDirectory),
		groutGamelist:         arkos.GetGroutGamelist,
		platforms:             arkos.Platforms,
		savesBesideRoms:       true,
		gamelist:              GamelistEmulationStation,
		groutLauncherPath:     "./Grout.sh",
		emulationStationBased: true,
		inputMapping:          arkos.GetInputMappingBytes,
		packaging:             Packaging{Asset: "Grout-ArkOS.zip", LaunchScript: "Grout.sh", InstallDepth: 2},
	},
	Batocera: {
		id:                    Batocera,
		romDirectory:          batocera.GetRomDirectory,
		biosDirectory:         batocera.GetBIOSDirectory,
		baseSavePath:          batocera.GetBaseSavePath,
		coverDirectory:        besideRoms(batocera.GetArtDirectory),
		sidecarDirectories:    esSidecars(batocera.GetVideoDirectory, batocera.GetManualDirectory, batocera.GetBezelDirectory),
		groutGamelist:         batocera.GetGroutGamelist,
		platforms:             batocera.Platforms,
		savesBesideRoms:       true,
		gamelist:              GamelistEmulationStation,
		groutLauncherPath:     "./Grout/Grout.sh",
		emulationStationBased: true,
		packaging: Packaging{ArchAssets: map[string]string{
			"arm64": "Grout-Batocera-arm64.zip",
			"amd64": "Grout-Batocera-amd64.zip",
			"386":   "Grout-Batocera-x86.zip",
		},
			LaunchScript: "Grout.sh",
			InstallDepth: 2},
	},
}

// Lookup returns the description of firmware c, or nil if unsupported. A nil
// Firmware is safe to use; every accessor reports absence rather than panicking.
func Lookup(c CFW) *Firmware { return firmwares[c] }

// Active returns the description of the firmware grout is running on.
func ActiveFirmware() *Firmware { return Lookup(GetCFW()) }

func (f *Firmware) RomDirectory() string {
	if f == nil || f.romDirectory == nil {
		return ""
	}
	return f.romDirectory()
}

func (f *Firmware) BIOSDirectory() string {
	if f == nil || f.biosDirectory == nil {
		return ""
	}
	return f.biosDirectory()
}

func (f *Firmware) BaseSavePath() string {
	if f == nil || f.baseSavePath == nil {
		return ""
	}
	return f.baseSavePath()
}

// Platforms maps a RomM filesystem slug to the rom folders this firmware
// accepts for it.
func (f *Firmware) Platforms() map[string][]string {
	if f == nil {
		return nil
	}
	return f.platforms
}

// SaveDirectories maps a RomM filesystem slug to emulator save folders, most
// preferred first. Firmwares that keep saves beside the roms return the
// platform table.
func (f *Firmware) SaveDirectories() map[string][]string {
	if f == nil {
		return nil
	}
	if f.savesBesideRoms {
		return f.platforms
	}
	return f.saveDirectories
}

// SavesBesideRoms reports whether saves live in the rom folder.
func (f *Firmware) SavesBesideRoms() bool { return f != nil && f.savesBesideRoms }

// GroutGamelist is where the launcher shortcut entry goes, or "" if the
// firmware has no gamelist.
func (f *Firmware) GroutGamelist() string {
	if f == nil || f.groutGamelist == nil {
		return ""
	}
	return f.groutGamelist()
}

// BIOSFilePaths returns every location to look for a BIOS file, most specific
// first.
func (f *Firmware) BIOSFilePaths(relativePath, platformFSSlug string) []string {
	if f == nil {
		return nil
	}
	if f.biosFilePaths != nil {
		return f.biosFilePaths(relativePath, platformFSSlug)
	}
	return []string{joinPath(f.BIOSDirectory(), relativePath)}
}

// UsesTaggedRomFolders reports whether this firmware allows a tag in the folder
// name, as in "Game Boy Advance (GBA)", and matches platforms on the tag.
func (f *Firmware) UsesTaggedRomFolders() bool { return f != nil && f.romFolderBase != nil }

// RomFolderBase strips folder decorations before matching; most firmwares use
// the name as-is.
func (f *Firmware) RomFolderBase(path string, tagParser func(string) string) string {
	if f == nil || f.romFolderBase == nil {
		return path
	}
	return f.romFolderBase(path, tagParser)
}

// ArtDirectory returns where a kind of artwork goes, or "" if the firmware
// keeps none.
func (f *Firmware) ArtDirectory(slot ArtSlot, romDir, platformFSSlug, platformName string) string {
	if f == nil {
		return ""
	}

	cover := func() string {
		if f.coverDirectory == nil {
			return ""
		}
		return f.coverDirectory(romDir, platformFSSlug, platformName)
	}

	switch slot {
	case ArtCover:
		return cover()

	case ArtMarquee, ArtBoxback, ArtFanart:
		// Share the cover's directory, told apart by filename suffix. See
		// ArtFileName.
		if f.sidecarDirectories == nil {
			return ""
		}
		return cover()

	case ArtVideo, ArtManual, ArtBezel:
		if f.sidecarDirectories == nil {
			return ""
		}
		video, manual, bezel := f.sidecarDirectories(romDir)
		switch slot {
		case ArtVideo:
			return video
		case ArtManual:
			return manual
		default:
			return bezel
		}

	case ArtScreenshotPreview:
		if f.previewDirectory == nil {
			return ""
		}
		return f.previewDirectory(platformFSSlug, platformName)

	case ArtThumbnail:
		// muOS calls this the splash directory.
		if f.splashDirectory == nil {
			return ""
		}
		return f.splashDirectory(platformFSSlug, platformName)

	default:
		return ""
	}
}

// The A30 reports a portrait framebuffer and needs rotating for landscape.
func spruceDisplay() DisplayQuirks {
	if spruce.DetectDevice() == spruce.DeviceA30 {
		return DisplayQuirks{RotationDegrees: 270}
	}
	return DisplayQuirks{}
}

func minuiDisplay() DisplayQuirks {
	switch minui.DetectDevice() {
	case minui.DeviceZero28:
		return DisplayQuirks{RotationDegrees: 90}
	case minui.DeviceMiyooFlip:
		return DisplayQuirks{DisableKeyboardAndJoystick: true}
	default:
		return DisplayQuirks{}
	}
}

func nextuiDisplay() DisplayQuirks {
	if nextui.DetectDevice() == nextui.DeviceMiyooFlip {
		return DisplayQuirks{DisableKeyboardAndJoystick: true}
	}
	return DisplayQuirks{}
}
