//go:build !dryrun

package main

import "fmt"

func runScenario(name string) error {
	return fmt.Errorf("scenarios require the dryrun build tag: go run -tags dryrun ./tools/save-sync-dry-run -scenario %s", name)
}
