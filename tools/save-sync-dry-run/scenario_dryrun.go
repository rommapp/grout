//go:build dryrun

package main

import (
	"grout/sync"
	"os"
)

func runScenario(name string) error {
	return sync.RunScenario(name, os.Stdout)
}
