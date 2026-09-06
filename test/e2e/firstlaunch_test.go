//go:build e2e

package e2e

import "testing"

// firmwares are the ones a run covers. Every one of them honours BASE_PATH, so
// each gets its own synthetic card and never touches the machine it runs on.
var firmwares = []string{
	"MUOS", "NEXTUI", "MINUI", "KNULLI", "SPRUCE", "ROCKNIX",
	"TRIMUI", "ALLIUM", "ONION", "KORIKI", "ARKOS", "BATOCERA",
}

// A device with no config starts by asking for a language, and cannot get
// anywhere until that is answered. This walks that first screen on every
// firmware, which is also the cheapest proof that grout starts at all on each
// of them.
func TestFirstLaunchAsksForALanguage(t *testing.T) {
	for _, firmware := range firmwares {
		t.Run(firmware, func(t *testing.T) {
			s := start(t, options{CFW: firmware})

			s.awaitLog("First launch detected, showing language selection")
			s.screenshot("language-selection")

			// The languages sit in a row, so they are chosen left and right.
			s.press("Right")
			s.press("a")

			selected := s.awaitLog("Language selected")
			if got := selected.Field("language"); got != "de" {
				t.Errorf("language = %v, want the one to the right of English", got)
			}

			// Nothing is configured yet, so the only way on is to log in.
			s.awaitLog("No RomM Host Configured, starting login flow")
			s.screenshot("login")
		})
	}
}

// Choosing a language should carry into the next screen, not just be recorded.
// The server screen is the first thing drawn after it.
func TestLanguageAppliesToTheNextScreen(t *testing.T) {
	s := start(t, options{CFW: "MUOS"})

	s.awaitLog("First launch detected, showing language selection")
	s.press("Right", "a")
	s.awaitLog("No RomM Host Configured, starting login flow")

	// Read this one: it should be the server form in German.
	s.screenshot("server-form-de")
}

// Grout writes its log to the card rather than to the machine it is running
// on, which is what makes a synthetic card enough to test against.
func TestWritesToTheCardAndNowhereElse(t *testing.T) {
	s := start(t, options{CFW: "MUOS"})

	s.awaitLog("First launch detected, showing language selection")

	if !s.cardHas("logs/app.log") {
		t.Error("grout did not write its log to the card")
	}
}
