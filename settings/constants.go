package settings

import "time"

const (
	MultipleFilesIcon      = "\U000F0222"
	MultipleDownloadedIcon = "\U000F09E9"
)

const (
	DefaultHTTPTimeout = 10 * time.Second
	UpdaterTimeout     = 10 * time.Minute
	LoginTimeout       = 6 * time.Second
	ValidationTimeout  = 3 * time.Second
)

// StartupRetryDelays are the waits between the attempts to reach the server when
// Grout opens, which is often just after Wi-Fi came up and before it can look
// names up. About six seconds of waiting in all, then the error is real enough to show.
var StartupRetryDelays = []time.Duration{1 * time.Second, 2 * time.Second, 3 * time.Second}
