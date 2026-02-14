package internal

type ReleaseChannel string

const (
	ReleaseChannelMatchRomM ReleaseChannel = "match_romm"
	ReleaseChannelStable    ReleaseChannel = "stable"
	ReleaseChannelBeta      ReleaseChannel = "beta"
)

type SaveSyncMode string

const (
	SaveSyncModeOff    SaveSyncMode = "off"
	SaveSyncModeManual SaveSyncMode = "manual"
)

type DownloadedGamesMode string

const (
	DownloadedGamesModeDoNothing DownloadedGamesMode = "do_nothing"
	DownloadedGamesModeMark      DownloadedGamesMode = "mark"
	DownloadedGamesModeFilter    DownloadedGamesMode = "filter"
)

type CollectionView string

const (
	CollectionViewPlatform CollectionView = "platform"
	CollectionViewUnified  CollectionView = "unified"
)

type LogLevel string

const (
	LogLevelDebug LogLevel = "DEBUG"
	LogLevelInfo  LogLevel = "INFO"
	LogLevelError LogLevel = "ERROR"
)
