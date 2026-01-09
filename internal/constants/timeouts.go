package constants

import "time"

const (
	DefaultHTTPTimeout = 10 * time.Second
	UpdaterTimeout     = 10 * time.Minute
	LoginTimeout       = 6 * time.Second // Timeout for login attempts
	ValidationTimeout  = 3 * time.Second // Fast timeout for pre-flight connection checks
)
