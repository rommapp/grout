//go:build e2e

package e2e

import "testing"

// A device that is already paired opens on its library rather than on the
// login screen, and the platforms it lists are the ones the server holds that
// the user has a folder for.
func TestPairedDeviceReachesItsLibrary(t *testing.T) {
	s := start(t, options{
		CFW:       "MUOS",
		Server:    romm(t),
		Platforms: []string{"snes", "gba"},
	})

	// Grout asks the server what it has before it can draw anything.
	s.awaitLog("Configuration Loaded!")
	s.screenshot("platform-list")
}

// The platforms on screen come from the server, so an empty mapping means an
// empty list however much the server holds.
func TestUnmappedPlatformsAreNotOffered(t *testing.T) {
	s := start(t, options{
		CFW:       "MUOS",
		Server:    romm(t),
		Platforms: []string{"snes"},
	})

	s.awaitLog("Configuration Loaded!")
	s.screenshot("only-snes-mapped")
}

// Every firmware has to be able to talk to the same server: the API is the
// same, and only the paths on the card differ.
func TestEveryFirmwareReachesTheServer(t *testing.T) {
	for _, firmware := range firmwares {
		t.Run(firmware, func(t *testing.T) {
			s := start(t, options{
				CFW:       firmware,
				Server:    romm(t),
				Platforms: []string{"snes", "gba"},
			})

			s.awaitLog("Configuration Loaded!")
			s.screenshot("library")
		})
	}
}
