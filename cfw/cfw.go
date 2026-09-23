package cfw

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

type CFW string

const (
	NextUI   CFW = "NEXTUI"
	MuOS     CFW = "MUOS"
	Knulli   CFW = "KNULLI"
	Spruce   CFW = "SPRUCE"
	ROCKNIX  CFW = "ROCKNIX"
	Trimui   CFW = "TRIMUI"
	Allium   CFW = "ALLIUM"
	Onion    CFW = "ONION"
	Koriki   CFW = "KORIKI"
	ArkOS    CFW = "ARKOS"
	Batocera CFW = "BATOCERA"
	MinUI    CFW = "MINUI"
	Anbernic CFW = "ANBERNIC"
)

// All lists every supported firmware. Conformance tests range over it.
var All = []CFW{
	NextUI, MuOS, Knulli, Spruce, ROCKNIX, Trimui,
	Allium, Onion, Koriki, ArkOS, Batocera, MinUI,
	Anbernic,
}

// ErrUnsupported reports a firmware name grout does not recognise.
var ErrUnsupported = errors.New("unsupported CFW")

// EnvVar is set by each firmware's launch script.
const EnvVar = "CFW"

// Supported reports whether c is a known firmware.
func (c CFW) Supported() bool {
	for _, known := range All {
		if c == known {
			return true
		}
	}
	return false
}

// Parse returns the firmware named by s, ignoring case and surrounding space.
func Parse(s string) (CFW, error) {
	c := CFW(strings.ToUpper(strings.TrimSpace(s)))
	if !c.Supported() {
		names := make([]string, len(All))
		for i, known := range All {
			names[i] = string(known)
		}
		return "", fmt.Errorf("%w: %q; valid options are %s", ErrUnsupported, s, strings.Join(names, ", "))
	}
	return c, nil
}

// Active returns the firmware named by the CFW environment variable. Resolved
// once at startup so nothing deeper has to handle a misconfigured device.
func Active() (CFW, error) {
	return Parse(os.Getenv(EnvVar))
}

// GetCFW returns the active firmware, or "" if unsupported. Callers treat ""
// as "no such thing here"; startup validates through Active.
func GetCFW() CFW {
	c, err := Active()
	if err != nil {
		slog.Default().Debug("Unsupported CFW environment variable", "error", err)
		return ""
	}
	return c
}

// IsBasedOnEmulationStation decides where artwork goes and what it is called.
func (c CFW) IsBasedOnEmulationStation() bool {
	return Lookup(c).IsBasedOnEmulationStation()
}
