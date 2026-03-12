package ui

// Action types for each screen.
// Screens set these directly in their output, eliminating the need for exit code mapping.

type PlatformSelectionAction int

const (
	PlatformSelectionActionSelected PlatformSelectionAction = iota
	PlatformSelectionActionCollections
	PlatformSelectionActionSettings
	PlatformSelectionActionSaveSync
	PlatformSelectionActionQuit
)

type GameListAction int

const (
	GameListActionSelected GameListAction = iota
	GameListActionSearch
	GameListActionBIOS
	GameListActionBack
	GameListActionClearSearch
	GameListActionFilters
)

type GameDetailsAction int

const (
	GameDetailsActionDownload GameDetailsAction = iota
	GameDetailsActionOptions
	GameDetailsActionBack
)

type GameOptionsAction int

const (
	GameOptionsActionSaved GameOptionsAction = iota
	GameOptionsActionShowQR
	GameOptionsActionBack
	GameOptionsActionSyncNow
)

type SearchAction int

const (
	SearchActionApply SearchAction = iota
	SearchActionCancel
)

type CollectionListAction int

const (
	CollectionListActionSelected CollectionListAction = iota
	CollectionListActionSearch
	CollectionListActionClearSearch
	CollectionListActionBack
)

type CollectionPlatformSelectionAction int

const (
	CollectionPlatformSelectionActionSelected CollectionPlatformSelectionAction = iota
	CollectionPlatformSelectionActionBack
)

type SettingsAction int

const (
	SettingsActionSaved SettingsAction = iota
	SettingsActionGeneral
	SettingsActionCollections
	SettingsActionAdvanced
	SettingsActionPlatformMapping
	SettingsActionInfo
	SettingsActionCheckUpdate
	SettingsActionSaveSync
	SettingsActionBack
)

type GeneralSettingsAction int

const (
	GeneralSettingsActionSaved GeneralSettingsAction = iota
	GeneralSettingsActionBack
)

type CollectionsSettingsAction int

const (
	CollectionsSettingsActionSaved CollectionsSettingsAction = iota
	CollectionsSettingsActionBack
)

type AdvancedSettingsAction int

const (
	AdvancedSettingsActionSaved AdvancedSettingsAction = iota
	AdvancedSettingsActionRebuildCache
	AdvancedSettingsActionSyncArtwork
	AdvancedSettingsActionBack
)

type PlatformMappingAction int

const (
	PlatformMappingActionSaved PlatformMappingAction = iota
	PlatformMappingActionBack
)

type InfoAction int

const (
	InfoActionLogout InfoAction = iota
	InfoActionBack
)

type LogoutConfirmationAction int

const (
	LogoutConfirmationActionConfirm LogoutConfirmationAction = iota
	LogoutConfirmationActionCancel
)

type GameFiltersAction int

const (
	GameFiltersActionApply GameFiltersAction = iota
	GameFiltersActionCancel
)

type SaveConflictAction int

const (
	SaveConflictActionResolved SaveConflictAction = iota
	SaveConflictActionCancel
)

type SaveMappingAction int

const (
	SaveMappingActionSaved SaveMappingAction = iota
	SaveMappingActionBack
)

type SyncMenuAction int

const (
	SyncMenuActionSyncNow SyncMenuAction = iota
	SyncMenuActionSyncedGames
	SyncMenuActionHistory
	SyncMenuActionBack
)

type SaveSyncSettingsAction int

const (
	SaveSyncSettingsActionBack SaveSyncSettingsAction = iota
	SaveSyncSettingsActionSaveMapping
)

type SyncedGamesAction int

const (
	SyncedGamesActionBack SyncedGamesAction = iota
	SyncedGamesActionSyncNow
)

type SyncHistoryAction int

const (
	SyncHistoryActionBack SyncHistoryAction = iota
)

type UpdateCheckAction int

const (
	UpdateCheckActionComplete UpdateCheckAction = iota
)
