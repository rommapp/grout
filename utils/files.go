package utils

import (
	"os"

	gaba "github.com/UncleJunVIP/gabagool/v2/pkg/gabagool"
)

func DeleteFile(path string) bool {
	logger := gaba.GetLogger()

	err := os.Remove(path)
	if err != nil {
		logger.Error("Issue removing file",
			"path", path,
			"error", err)
		return false
	} else {
		logger.Debug("Removed file", "path", path)
		return true
	}
}
