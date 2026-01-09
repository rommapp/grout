package cache

import "time"

type Config interface {
	GetApiTimeout() time.Duration
	GetShowCollections() bool
	GetShowSmartCollections() bool
	GetShowVirtualCollections() bool
}
