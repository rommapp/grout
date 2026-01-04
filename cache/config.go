package cache

import "time"

// Config defines the configuration interface needed by the cache package.
// This interface is implemented by utils.Config.
type Config interface {
	GetApiTimeout() time.Duration
	GetShowCollections() bool
	GetShowSmartCollections() bool
	GetShowVirtualCollections() bool
}
