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
)

// All lists every supported firmware. Table-driven tests range over it so that
// adding one without teaching it a directory or a save path is a test failure
// rather than an empty string at runtime.
var All = []CFW{
	NextUI, MuOS, Knulli, Spruce, ROCKNIX, Trimui,
	Allium, Onion, Koriki, ArkOS, Batocera, MinUI,
}

// ErrUnsupported reports a firmware name grout does not recognise.
var ErrUnsupported = errors.New("unsupported CFW")

// EnvVar names the environment variable each firmware's launch script sets to
// tell grout which one it is running on.
const EnvVar = "CFW"

// Supported reports whether c is a firmware grout knows.
func (c CFW) Supported() bool {
	for _, known := range All {
		if c == known {
			return true
		}
	}
	return false
}

// Parse returns the firmware named by s, matched without regard to case or
// surrounding whitespace.
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

// Active returns the firmware grout is running on, named by the CFW
// environment variable. The application resolves this once at startup and
// reports the error to the user; nothing deeper in the stack should have to
// decide what to do about a misconfigured device.
func Active() (CFW, error) {
	return Parse(os.Getenv(EnvVar))
}

// GetCFW returns the active firmware, or "" when the environment names one
// grout does not support.
//
// It used to call log.Fatalf, which killed the process from inside path
// helpers that run during rendering, and forced tests to set the environment
// variable purely to avoid being terminated. Callers already treat "" as "this
// firmware has no such thing", so an unknown firmware now takes that path.
// Startup validates properly through Active.
func GetCFW() CFW {
	c, err := Active()
	if err != nil {
		slog.Default().Debug("Unsupported CFW environment variable", "error", err)
		return ""
	}
	return c
}

func (c CFW) IsBasedOnEmulationStation() bool {
	switch c {
	case Knulli, ROCKNIX, ArkOS, Batocera:
		return true
	default:
		return false
	}
}
