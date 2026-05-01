package cfw

import (
	"log"
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

func GetCFW() CFW {
	cfwEnv := strings.ToUpper(os.Getenv("CFW"))
	cfw := CFW(cfwEnv)

	switch cfw {
	case MuOS, NextUI, Knulli, Spruce, ROCKNIX, Trimui, Allium, Onion, Koriki, ArkOS, Batocera, MinUI:
		return cfw
	default:
		log.SetOutput(os.Stderr)
		log.Fatalf("Unsupported CFW: '%s'. Valid options: NextUI, muOS, Knulli, Spruce, ROCKNIX, Trimui, Allium, Onion, Koriki, ArkOS, Batocera, MinUI", cfwEnv)
		return ""
	}
}

func (c CFW) IsBasedOnEmulationStation() bool {
	switch c {
	case Knulli, ROCKNIX, ArkOS, Batocera:
		return true
	default:
		return false
	}
}
