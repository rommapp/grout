package emulationstation

import (
	"fmt"
	"os"
)

const (
	flagPath = "./es_restart_request"
)

func ScheduleESRestart() error {
	file, err := os.Create(flagPath)
	if err != nil {
		return fmt.Errorf("unable to create restart flag file: %w", err)
	}
	defer file.Close()

	return nil
}
