//go:build e2e

package e2e

import (
	"strings"
	"testing"
)

// fixtureRom is what the library fixture holds, so a downloaded file can be
// checked against what the server actually served rather than just existing.
const fixtureRom = "not a real rom"

// Downloading a game puts it in the folder the user mapped for its platform,
// which is the whole point of the mapping.
func TestDownloadPutsTheRomInItsMappedFolder(t *testing.T) {
	s := start(t, options{CFW: "MUOS", Server: romm(t), Platforms: []string{"snes"}})
	s.awaitLog("Configuration Loaded!")

	s.press("a") // the platform
	s.press("a") // the game
	s.press("a") // download it

	s.awaitCardFile("ROMS/snes/Test Game (USA).sfc")
	s.press("a") // acknowledge "Download Completed!"
	s.screenshot("download-complete")

	// The file has to be what the server served. An empty file in the right
	// place would pass a test that only looked for the name.
	if got := string(s.cardFile("ROMS/snes/Test Game (USA).sfc")); got != fixtureRom {
		t.Errorf("downloaded %q, want the contents the server holds", got)
	}
}

// The EmulationStation firmwares read a gamelist.xml beside the roms, so a
// download that does not write one leaves a game the frontend cannot see.
func TestDownloadWritesTheGamelistForEmulationStation(t *testing.T) {
	s := start(t, options{
		CFW: "KNULLI", Server: romm(t), Platforms: []string{"snes"},
		// Every EmulationStation card has a tools folder, and grout adds
		// itself to the gamelist in it at startup.
		Existing: []string{"roms/tools"},
	})
	s.awaitLog("Configuration Loaded!")

	s.press("a", "a", "a")

	// Knulli keeps its roms in a lowercase folder where muOS uses uppercase,
	// which is the sort of difference this suite exists to pin down.
	s.awaitCardFile("roms/snes/Test Game (USA).sfc")

	// The metadata is written once the user acknowledges the download, not
	// when the file lands. Nothing auto-continues unless artwork is being
	// fetched too.
	s.press("a")
	s.awaitCardFile("roms/snes/gamelist.xml")

	gamelist := string(s.cardFile("roms/snes/gamelist.xml"))
	if !strings.Contains(gamelist, "Test Game") {
		t.Errorf("gamelist does not mention the game that was downloaded:\n%s", gamelist)
	}
	// The path is what the frontend launches, so it has to be the file that
	// was actually written rather than the name it had on the server.
	if !strings.Contains(gamelist, "Test Game (USA).sfc") {
		t.Errorf("gamelist does not point at the rom on disk:\n%s", gamelist)
	}
}

// muOS keeps its own metadata rather than a gamelist, so the same download
// has to leave something different behind.
func TestDownloadWritesMuOSMetadata(t *testing.T) {
	s := start(t, options{CFW: "MUOS", Server: romm(t), Platforms: []string{"snes"}})
	s.awaitLog("Configuration Loaded!")

	s.press("a", "a", "a")
	s.awaitCardFile("ROMS/snes/Test Game (USA).sfc")
	s.press("a") // acknowledge, which is when metadata is written
	s.awaitStill()

	if s.cardHas("ROMS/snes/gamelist.xml") {
		t.Error("muOS was given an EmulationStation gamelist")
	}
}

// A platform the user has not mapped has nowhere to put a download, and its
// games are not offered at all.
func TestDownloadOnlyOffersMappedPlatforms(t *testing.T) {
	s := start(t, options{CFW: "MUOS", Server: romm(t), Platforms: []string{"gba"}})
	s.awaitLog("Configuration Loaded!")

	s.press("a", "a", "a")
	s.awaitCardFile("ROMS/gba/Another Game (Europe).gba")
	s.press("a")

	// The snes game is on the server but its platform was never mapped.
	if s.cardHas("ROMS/snes/Test Game (USA).sfc") {
		t.Error("a game from an unmapped platform was downloaded")
	}
}
