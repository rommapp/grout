package cache

import (
	"errors"
	"fmt"
)

var (
	ErrNotInitialized  = errors.New("cache manager not initialized")
	ErrCacheMiss       = errors.New("cache miss")
	ErrDBClosed        = errors.New("database connection closed")
	ErrInvalidCacheKey = errors.New("invalid cache key")
)

type Error struct {
	Op        string // Operation name: "get", "save", "delete", etc.
	Key       string // Cache key if applicable
	CacheType string // "platform", "collection", "rom_id", "artwork"
	Err       error  // Underlying error
}

func (e *Error) Error() string {
	if e.Key != "" {
		return fmt.Sprintf("cache %s [%s:%s]: %v", e.Op, e.CacheType, e.Key, e.Err)
	}
	if e.CacheType != "" {
		return fmt.Sprintf("cache %s [%s]: %v", e.Op, e.CacheType, e.Err)
	}
	return fmt.Sprintf("cache %s: %v", e.Op, e.Err)
}

func (e *Error) Unwrap() error {
	return e.Err
}

func newCacheError(op, cacheType, key string, err error) *Error {
	return &Error{
		Op:        op,
		Key:       key,
		CacheType: cacheType,
		Err:       err,
	}
}
