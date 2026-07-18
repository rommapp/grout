package romm

const (
	endpointHeartbeat = "/api/heartbeat"
	endpointConfig    = "/api/config"

	endpointPlatforms           = "/api/platforms"
	endpointPlatformByID        = "/api/platforms/%d"
	endpointPlatformIdentifiers = "/api/platforms/identifiers"

	endpointRoms           = "/api/roms"
	endpointRomByID        = "/api/roms/%d"
	endpointRomsByHash     = "/api/roms/by-hash"
	endpointRomIdentifiers = "/api/roms/identifiers"

	endpointCollections           = "/api/collections"
	endpointCollectionByID        = "/api/collections/%d"
	endpointSmartCollections      = "/api/collections/smart"
	endpointVirtualCollections    = "/api/collections/virtual"
	endpointCollectionIdentifiers = "/api/collections/identifiers"

	endpointFirmware            = "/api/firmware"
	endpointFirmwareIdentifiers = "/api/firmware/identifiers"

	endpointSaves          = "/api/saves"
	endpointSaveByID       = "/api/saves/%d"
	endpointSaveSummary    = "/api/saves/summary"
	endpointSaveContent    = "/api/saves/%d/content"
	endpointSaveDownloaded = "/api/saves/%d/downloaded"

	endpointDevices    = "/api/devices"
	endpointDeviceByID = "/api/devices/%s"

	endpointSyncNegotiate       = "/api/sync/negotiate"
	endpointSyncSessionComplete = "/api/sync/sessions/%d/complete"

	endpointTokenExchange = "/api/client-tokens/exchange"
	endpointCurrentUser   = "/api/users/me"

	endpointDeviceAuthInit  = "/api/auth/device/init"
	endpointDeviceAuthToken = "/api/auth/device/token"
)
