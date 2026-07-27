package server

import (
	"context"
	"time"
)

// MemorySessionAdapter is the built-in in-memory session adapter.
// It is useful for local development, tests, and single-process deployments.
type MemorySessionAdapter struct{}

// NewMemorySessionAdapter returns the built-in in-memory session adapter.
func NewMemorySessionAdapter() SessionStorage {
	return MemorySessionAdapter{}
}

// MemorySessionStorage is kept as a compatibility alias.
func MemorySessionStorage() SessionStorage {
	return NewMemorySessionAdapter()
}

func (MemorySessionAdapter) GetName() string {
	return "memory"
}

func (MemorySessionAdapter) OpenSessionStorage(ctx context.Context, config SessionStorageConfig) (SessionStorageRuntime, error) {
	_ = ctx

	ttl := config.TTL
	if ttl <= 0 {
		ttl = defaultTTL
	}

	return newMemorySessionRuntime(func() memorySessionOptions {
		return memorySessionOptions{
			StorageName: "memory",
			TTL:         ttl,
		}
	}), nil
}

var _ SessionStorage = MemorySessionAdapter{}
var _ SessionStorageRuntime = (*memorySessionRuntime)(nil)
var _ SessionStorageInstance = (*memorySessionInstance)(nil)

func normalizeStorageTTL(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return defaultTTL
	}

	return ttl
}
