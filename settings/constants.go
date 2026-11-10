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

// The time to wait for RomM to answer the first request of a launch sync. A sync
// the player asked for, and the upload after a game, can afford to wait longer
// than one that holds up the start of a game.
const (
	LaunchProbeTimeout    = 5 * time.Second
	OnRequestProbeTimeout = 8 * time.Second
)

// StartupRetryDelays are the waits between the attempts to reach the server when
// Grout opens, which is often just after Wi-Fi came up and before it can look
// names up. About ten seconds in all, then the error is real enough to show.
var StartupRetryDelays = []time.Duration{1 * time.Second, 2 * time.Second, 3 * time.Second, 4 * time.Second}

// LaunchRetryDelays are the same for a sync as a game starts, where the game is
// waiting, so only a couple of quick tries and only for a network that is not
// ready yet.
var LaunchRetryDelays = []time.Duration{1 * time.Second, 2 * time.Second}
