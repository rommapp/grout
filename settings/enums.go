package settings

type ReleaseChannel string

const (
	ReleaseChannelMatchRomM ReleaseChannel = "match_romm"
	ReleaseChannelStable    ReleaseChannel = "stable"
	ReleaseChannelBeta      ReleaseChannel = "beta"
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

// LaunchSyncMode is when a game's save is synced around playing it. Empty
// means never.
type LaunchSyncMode string

const (
	LaunchSyncOff            LaunchSyncMode = ""
	LaunchSyncBefore         LaunchSyncMode = "before"
	LaunchSyncBeforeAndAfter LaunchSyncMode = "before_and_after"
)

// BeforeLaunch reports whether the save is brought up to date as the game starts.
func (m LaunchSyncMode) BeforeLaunch() bool {
	return m == LaunchSyncBefore || m == LaunchSyncBeforeAndAfter
}

// AfterQuit reports whether the save is uploaded once the game is quit.
func (m LaunchSyncMode) AfterQuit() bool { return m == LaunchSyncBeforeAndAfter }

// RomStorage pins muOS roms to one card. Empty means automatic. The values
// are muOS's own names for each storage.
type RomStorage string

const (
	RomStorageAuto RomStorage = ""
	RomStorageSD1  RomStorage = "rom"
	RomStorageSD2  RomStorage = "sdcard"
	RomStorageUSB  RomStorage = "usb"
)

// RomStorageEnvVar carries the choice to the firmware layer, which reads the
// environment rather than the config, as it does for BASE_PATH.
const RomStorageEnvVar = "GROUT_MUOS_ROM_STORAGE"
