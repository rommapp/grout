package utils

import "sync/atomic"

var kidModeEnabled atomic.Bool

// InitKidMode initializes the kid mode state from config
func InitKidMode(config *Config) {
	kidModeEnabled.Store(config.KidMode)
}

// IsKidModeEnabled returns whether kid mode is currently enabled
func IsKidModeEnabled() bool {
	return kidModeEnabled.Load()
}

// SetKidMode sets the kid mode state
func SetKidMode(enabled bool) {
	kidModeEnabled.Store(enabled)
}
