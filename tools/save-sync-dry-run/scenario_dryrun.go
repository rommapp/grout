//go:build dryrun

package main

import (
	"grout/saves"
	"os"
)

func runScenario(name string) error {
	return saves.RunScenario(name, os.Stdout)
}
