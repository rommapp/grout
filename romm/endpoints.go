package romm

const (
	endpointHeartbeat = "/api/heartbeat"
	endpointLogin     = "/api/login"
	endpointConfig    = "/api/config"

	endpointPlatforms           = "/api/platforms"
	endpointPlatformByID        = "/api/platforms/%d"
	endpointPlatformIdentifiers = "/api/platforms/identifiers"

	endpointRoms           = "/api/roms"
	endpointRomByID        = "/api/roms/%d"
	endpointRomsDownload   = "/api/roms/download"
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
	endpointDevices        = "/api/devices"
	endpointDeviceByID     = "/api/devices/%s"

	endpointSyncNegotiate       = "/api/sync/negotiate"
	endpointSyncSessionComplete = "/api/sync/sessions/%d/complete"
)
