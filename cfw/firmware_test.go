package cfw

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// setupDevice points the firmware packages at a temporary root; most read
// BASE_PATH to find the SD card.
func setupDevice(t *testing.T) {
	t.Helper()
	t.Setenv("BASE_PATH", t.TempDir())
}

// A firmware in All with no entry here returns empty strings for every path at
// runtime, silently.
func TestFirmwares_DescribeEverySupportedFirmware(t *testing.T) {
	for _, c := range All {
		if Lookup(c) == nil {
			t.Errorf("%s is supported but has no entry in firmwares", c)
		}
	}

	for c, f := range firmwares {
		if !slices.Contains(All, c) {
			t.Errorf("firmwares has an entry for %s, which is not in All", c)
		}
		if f.ID() != c {
			t.Errorf("firmwares[%s] has id %s; the key and the id must agree", c, f.ID())
		}
	}
}

// The paths grout cannot work without. A firmware missing one is unusable, so
// catch it here rather than in a support ticket.
func TestFirmwares_HaveTheEssentialPaths(t *testing.T) {
	setupDevice(t)

	for _, c := range All {
		f := Lookup(c)
		if f.RomDirectory() == "" {
			t.Errorf("%s has no rom directory", c)
		}
		if f.BIOSDirectory() == "" {
			t.Errorf("%s has no BIOS directory", c)
		}
		if f.BaseSavePath() == "" {
			t.Errorf("%s has no base save path", c)
		}
		if len(f.Platforms()) == 0 {
			t.Errorf("%s has no platform table", c)
		}
		if f.ArtDirectory(ArtCover, "/roms/gba", "gba", "Game Boy Advance") == "" {
			t.Errorf("%s has no cover art directory", c)
		}
	}
}

// Every firmware needs somewhere to put saves, whether that is its own tree or
// the rom folder.
func TestFirmwares_ResolveSaveDirectories(t *testing.T) {
	for _, c := range All {
		f := Lookup(c)
		if len(f.SaveDirectories()) == 0 {
			t.Errorf("%s resolves no save directories", c)
		}

		// A firmware that keeps saves beside the roms reuses the platform
		// table, and must not also carry a save table that nothing reads.
		if f.SavesBesideRoms() {
			if f.saveDirectories != nil {
				t.Errorf("%s keeps saves beside roms but also declares a save table", c)
			}
			if len(f.SaveDirectories()) != len(f.Platforms()) {
				t.Errorf("%s keeps saves beside roms, so its save and platform tables should be the same", c)
			}
		}
	}
}

// Several behaviours follow from the frontend, and must agree with it.
func TestFirmwares_EmulationStationFamilyIsConsistent(t *testing.T) {
	for _, c := range All {
		f := Lookup(c)
		es := f.IsBasedOnEmulationStation()

		if es != (f.Gamelist() == GamelistEmulationStation) {
			t.Errorf("%s: IsBasedOnEmulationStation=%v but gamelist format is %v", c, es, f.Gamelist())
		}
		if es != (f.sidecarDirectories != nil) {
			t.Errorf("%s: IsBasedOnEmulationStation=%v but sidecar directories present=%v", c, es, f.sidecarDirectories != nil)
		}
		if es != (f.GroutLauncherPath() != "") {
			t.Errorf("%s: IsBasedOnEmulationStation=%v but launcher path is %q", c, es, f.GroutLauncherPath())
		}

		// The method on CFW must agree with the registry it now reads from.
		if c.IsBasedOnEmulationStation() != es {
			t.Errorf("%s: CFW method and firmware entry disagree", c)
		}
	}
}

// A gamelist needs both a file and a command; one without the other writes a
// broken shortcut.
func TestFirmwares_GamelistShortcutIsCompleteOrAbsent(t *testing.T) {
	setupDevice(t)

	for _, c := range All {
		f := Lookup(c)
		hasFile := f.GroutGamelist() != ""
		hasLauncher := f.GroutLauncherPath() != ""
		if hasFile != hasLauncher {
			t.Errorf("%s has gamelist=%v launcher=%v; it needs both or neither", c, hasFile, hasLauncher)
		}
	}
}

// Every firmware must look somewhere for a BIOS file, never a bare relative
// path.
func TestFirmwares_BIOSFilePathsAreAbsolute(t *testing.T) {
	setupDevice(t)

	for _, c := range All {
		paths := Lookup(c).BIOSFilePaths("scph1001.bin", "psx")
		if len(paths) == 0 {
			t.Errorf("%s returns no BIOS paths", c)
			continue
		}
		for _, path := range paths {
			if path == "" || path == "scph1001.bin" {
				t.Errorf("%s returned %q, which is not a location", c, path)
			}
		}
	}
}

// Every accessor must survive a nil entry, since GetCFW returns the empty
// firmware for anything unrecognised.
func TestFirmware_NilIsSafe(t *testing.T) {
	var f *Firmware

	if f.ID() != "" {
		t.Error("nil firmware should have no id")
	}
	if f.RomDirectory() != "" || f.BIOSDirectory() != "" || f.BaseSavePath() != "" {
		t.Error("nil firmware should have no paths")
	}
	if f.Platforms() != nil || f.SaveDirectories() != nil {
		t.Error("nil firmware should have no tables")
	}
	if f.ArtDirectory(ArtCover, "/roms", "gba", "GBA") != "" {
		t.Error("nil firmware should have no art directory")
	}
	if f.GroutGamelist() != "" || f.GroutLauncherPath() != "" {
		t.Error("nil firmware should have no gamelist")
	}
	if f.Gamelist() != GamelistNone {
		t.Error("nil firmware should have no gamelist format")
	}
	if f.SavesBesideRoms() || f.KeepsRomExtInSaves() || f.IsBasedOnEmulationStation() {
		t.Error("nil firmware should claim no capabilities")
	}
	if f.BIOSFilePaths("x", "psx") != nil {
		t.Error("nil firmware should return no BIOS paths")
	}
	if got := f.RomFolderBase("Game Boy Advance", nil); got != "Game Boy Advance" {
		t.Errorf("nil firmware should pass the folder name through, got %q", got)
	}

	if Lookup("NOT_A_FIRMWARE") != nil {
		t.Error("an unknown firmware should have no entry")
	}
}

// Only the minarch firmwares name saves after the whole rom file.
func TestFirmwares_KeepsRomExtIsMinarchOnly(t *testing.T) {
	minarch := []CFW{NextUI, MinUI}
	for _, c := range All {
		want := slices.Contains(minarch, c)
		if got := DefaultKeepsRomExt(c); got != want {
			t.Errorf("DefaultKeepsRomExt(%s) = %v, want %v", c, got, want)
		}
	}
}

// muOS and TrimUI keep artwork in a catalogue keyed by platform rather than
// beside the roms.
func TestFirmwares_CatalogueFirmwaresIgnoreTheRomDirectory(t *testing.T) {
	setupDevice(t)

	for _, c := range []CFW{MuOS, Trimui} {
		f := Lookup(c)
		a := f.ArtDirectory(ArtCover, "/roms/gba", "gba", "Game Boy Advance")
		b := f.ArtDirectory(ArtCover, "/somewhere/else", "gba", "Game Boy Advance")
		if a != b {
			t.Errorf("%s art directory changed with the rom directory: %q vs %q", c, a, b)
		}
	}

	// And the reverse: firmwares that store art beside the roms must follow
	// the rom directory.
	for _, c := range []CFW{Knulli, Spruce, MinUI} {
		f := Lookup(c)
		a := f.ArtDirectory(ArtCover, "/roms/gba", "gba", "Game Boy Advance")
		b := f.ArtDirectory(ArtCover, "/somewhere/else", "gba", "Game Boy Advance")
		if a == b {
			t.Errorf("%s art directory ignored the rom directory", c)
		}
	}
}

// Every firmware ships a release archive, and its asset name must match what
// the release workflow publishes. ArkOS built and published Grout-ArkOS.zip
// while the updater returned "" for it, so in-app update silently did nothing.
func TestFirmwares_HavePackaging(t *testing.T) {
	for _, c := range All {
		p := Lookup(c).Packaging()

		if p.Asset == "" && len(p.ArchAssets) == 0 {
			t.Errorf("%s has no release asset name", c)
		}
		if p.LaunchScript == "" {
			t.Errorf("%s has no launch script path", c)
		}
		if p.InstallDepth < 1 {
			t.Errorf("%s has install depth %d; it must be at least 1", c, p.InstallDepth)
		}
	}
}

// Every architecture grout builds for must resolve to an asset.
func TestPackaging_AssetNamePerArch(t *testing.T) {
	for _, c := range All {
		p := Lookup(c).Packaging()
		if p.ArchAssets == nil {
			// One asset for every architecture.
			if got := p.AssetName("arm64"); got != p.Asset {
				t.Errorf("%s: AssetName = %q, want %q", c, got, p.Asset)
			}
			continue
		}
		for _, arch := range []string{"arm64", "amd64", "386"} {
			if p.AssetName(arch) == "" {
				t.Errorf("%s has no asset for GOARCH %s", c, arch)
			}
		}
	}
}

// Two firmwares sharing an asset name would have one overwrite the other's
// release.
func TestFirmwares_AssetNamesAreUnique(t *testing.T) {
	seen := map[string]CFW{}
	for _, c := range All {
		p := Lookup(c).Packaging()
		names := []string{p.Asset}
		for _, n := range p.ArchAssets {
			names = append(names, n)
		}
		for _, name := range names {
			if name == "" {
				continue
			}
			if other, clash := seen[name]; clash {
				t.Errorf("%s and %s both publish %q", other, c, name)
			}
			seen[name] = c
		}
	}
}

// Only the firmwares that allow a tag in the folder name resolve one.
func TestFirmwares_TaggedRomFoldersMatchesRomFolderBase(t *testing.T) {
	tagged := []CFW{NextUI, MinUI}
	for _, c := range All {
		want := slices.Contains(tagged, c)
		if got := Lookup(c).UsesTaggedRomFolders(); got != want {
			t.Errorf("%s: UsesTaggedRomFolders = %v, want %v", c, got, want)
		}
	}
}

// A rotation the toolkit cannot express would be silently ignored by app.
func TestFirmwares_DisplayRotationIsQuarterTurns(t *testing.T) {
	setupDevice(t)

	for _, c := range All {
		switch d := Lookup(c).Display(); d.RotationDegrees {
		case 0, 90, 180, 270:
		default:
			t.Errorf("%s reports rotation %d; only quarter turns are supported", c, d.RotationDegrees)
		}
	}
}

// The registry's asset names are only useful if they match what the release
// workflow actually publishes. Nothing else connects the two, and a mismatch
// makes in-app update do nothing rather than fail.
func TestFirmwares_AssetNamesMatchReleaseWorkflow(t *testing.T) {
	workflow, err := os.ReadFile(filepath.Join("..", ".github", "workflows", "release.yml"))
	if err != nil {
		t.Skipf("release workflow not readable: %v", err)
	}
	published := string(workflow)

	for _, c := range All {
		p := Lookup(c).Packaging()
		names := []string{p.Asset}
		for _, n := range p.ArchAssets {
			names = append(names, n)
		}
		for _, name := range names {
			if name == "" {
				continue
			}
			if !strings.Contains(published, name) {
				t.Errorf("%s publishes %q, which release.yml never produces", c, name)
			}
		}
	}
}
