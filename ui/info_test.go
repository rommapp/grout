package ui

import (
	"strings"
	"testing"

	"grout/settings"
)

func TestTokenExpiry(t *testing.T) {
	// A token with no expiry does not run out, which is worth saying rather
	// than leaving blank.
	if got := tokenExpiry(""); got == "" {
		t.Error("an empty expiry showed nothing")
	}

	if got := tokenExpiry("2030-06-01T12:00:00Z"); !strings.HasPrefix(got, "2030-06-01") {
		t.Errorf("expiry = %q, want it read as a date", got)
	}
}

// A date grout cannot read is shown as it arrived rather than hidden. A token
// with an unreadable expiry is worth noticing, not swallowing.
func TestTokenExpiry_UnreadableDateIsShown(t *testing.T) {
	if got := tokenExpiry("whenever"); got != "whenever" {
		t.Errorf("expiry = %q, want the value as it arrived", got)
	}
}

// The expiry row only makes sense for a host that has a token.
func TestServerFacts_TokenRowsFollowTheToken(t *testing.T) {
	without := serverFacts(settings.Host{RootURI: "http://romm.local"}, "4.0.0")
	for _, fact := range without {
		if fact.Label == localize("info_token_expires", "Expires") {
			t.Error("a host with no token showed an expiry")
		}
	}

	with := serverFacts(settings.Host{RootURI: "http://romm.local", Token: "abc"}, "4.0.0")
	found := false
	for _, fact := range with {
		if fact.Label == localize("info_token_expires", "Expires") {
			found = true
		}
	}
	if !found {
		t.Error("a host with a token showed no expiry")
	}
}

// A server that did not say its version still gets a row, or the screen would
// look like it failed to load rather than like the server was quiet.
func TestServerFacts_UnknownVersion(t *testing.T) {
	facts := serverFacts(settings.Host{}, "")

	for _, fact := range facts {
		if fact.Label == localize("info_romm_version", "Version") {
			if fact.Value == "" {
				t.Error("an unknown version showed nothing")
			}
			return
		}
	}
	t.Error("no version row")
}

// The repository QR is a file that may not have been made. An empty path must
// not become an image section pointing at nothing.
func TestInfoSections_WithoutAQRCode(t *testing.T) {
	with := infoSections(InfoInput{}, "/tmp/qr.png")
	without := infoSections(InfoInput{}, "")

	if len(without) != len(with)-1 {
		t.Errorf("got %d sections without a QR and %d with, want one fewer", len(without), len(with))
	}
}
