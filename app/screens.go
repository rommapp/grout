package main

import (
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/router"
)

// Screen identifiers for the router
type Screen = router.Screen

const (
	ScreenPlatformSelection Screen = iota
	ScreenGameList
	ScreenGameDetails
	ScreenGameOptions
	ScreenSearch
	ScreenCollectionList
	ScreenCollectionPlatformSelection
	ScreenSettings
	ScreenGeneralSettings
	ScreenCollectionsSettings
	ScreenAdvancedSettings
	ScreenPlatformMapping
	ScreenSaveSyncSettings
	ScreenInfo
	ScreenLogoutConfirmation
	ScreenRebuildCache
	ScreenSaveSync
	ScreenBIOSDownload
	ScreenArtworkSync
	ScreenUpdateCheck
)
