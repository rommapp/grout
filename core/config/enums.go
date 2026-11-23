package config

import "log/slog"

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

func DownloadedGamesModes() []DownloadedGamesMode {
	return []DownloadedGamesMode{DownloadedGamesModeDoNothing, DownloadedGamesModeMark, DownloadedGamesModeFilter}
}

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

func LogLevels() []LogLevel {
	return []LogLevel{LogLevelDebug, LogLevelInfo, LogLevelError}
}

type RomStorage string

const (
	RomStorageAuto RomStorage = ""
	RomStorageSD1  RomStorage = "rom"
	RomStorageSD2  RomStorage = "sdcard"
	RomStorageUSB  RomStorage = "usb"
)

type ArtKind string

const (
	ArtKindNone       ArtKind = ""
	ArtKindDefault    ArtKind = "Default"
	ArtKindBox2D      ArtKind = "Box2D"
	ArtKindBox3D      ArtKind = "Box3D"
	ArtKindMixImage   ArtKind = "Miximage"
	ArtKindMixImageV2 ArtKind = "Miximage_v2"
	ArtKindMarquee    ArtKind = "Marquee"
	ArtKindLogo       ArtKind = "Logo"
	ArtKindTitle      ArtKind = "Title"
	ArtKindScreenshot ArtKind = "Screenshot"
	ArtKindVideo      ArtKind = "Video"
)

func CoverArtKinds() []ArtKind {
	return []ArtKind{ArtKindDefault, ArtKindBox2D, ArtKindBox3D, ArtKindMixImage, ArtKindMixImageV2}
}

func SplashArtKinds() []ArtKind {
	return []ArtKind{ArtKindNone, ArtKindMarquee, ArtKindTitle}
}

func MarqueeArtKinds() []ArtKind {
	return []ArtKind{ArtKindNone, ArtKindMarquee, ArtKindLogo}
}

func (l LogLevel) Slog() slog.Level {
	switch l {
	case LogLevelDebug:
		return slog.LevelDebug
	case LogLevelInfo:
		return slog.LevelInfo
	default:
		return slog.LevelError
	}
}
