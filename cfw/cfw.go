package cfw

import (
	"log"
	"os"
	"strings"
)

type CFW string

const (
	NextUI CFW = "NEXTUI"
	MuOS   CFW = "MUOS"
	Knulli CFW = "KNULLI"
	Spruce CFW = "SPRUCE"
)

func GetCFW() CFW {
	cfwEnv := strings.ToUpper(os.Getenv("CFW"))
	cfw := CFW(cfwEnv)

	switch cfw {
	case MuOS, NextUI, Knulli, Spruce:
		return cfw
	default:
		log.SetOutput(os.Stderr)
		log.Fatalf("Unsupported CFW: '%s'. Valid options: NextUI, muOS, Knulli, Spruce", cfwEnv)
		return ""
	}
}
