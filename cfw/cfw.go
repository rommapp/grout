package cfw

import (
	"log"
	"os"
	"strings"
)

type CFW string

const (
	AmberELEC CFW = "AMBERELEC"
	NextUI   CFW = "NEXTUI"
	MuOS     CFW = "MUOS"
	Knulli   CFW = "KNULLI"
	Spruce   CFW = "SPRUCE"
	ROCKNIX  CFW = "ROCKNIX"
	Trimui   CFW = "TRIMUI"
	Allium   CFW = "ALLIUM"
	Onion    CFW = "ONION"
	Koriki   CFW = "KORIKI"
	Batocera CFW = "BATOCERA"
	MinUI    CFW = "MINUI"
)

func GetCFW() CFW {
	cfwEnv := strings.ToUpper(os.Getenv("CFW"))
	cfw := CFW(cfwEnv)

	switch cfw {
	case AmberELEC, MuOS, NextUI, Knulli, Spruce, ROCKNIX, Trimui, Allium, Onion, Koriki, Batocera, MinUI:
		return cfw
	default:
		log.SetOutput(os.Stderr)
		log.Fatalf("Unsupported CFW: '%s'. Valid options: AmberELEC, NextUI, muOS, Knulli, Spruce, ROCKNIX, Trimui, Allium, Onion, Koriki, Batocera, MinUI", cfwEnv)
		return ""
	}
}

func (c CFW) IsBasedOnEmulationStation() bool {
	switch c {
	case AmberELEC, Knulli, ROCKNIX, Batocera:
		return true
	default:
		return false
	}
}
