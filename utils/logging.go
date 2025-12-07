package utils

import (
	"log"
	"os"
)

func LogStandardFatal(msg string, err error) {
	log.SetOutput(os.Stderr)
	log.Fatalf("%s: %v", msg, err)
}
