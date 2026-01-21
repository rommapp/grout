package internal

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
