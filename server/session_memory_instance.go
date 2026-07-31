package server

import (
	"strings"
	"time"
)

type memorySessionInstance struct {
	adapter *memorySessionRuntime
	id      string
}

func (i *memorySessionInstance) ID() string {
	if i == nil {
		return ""
	}

	return i.id
}

func (i *memorySessionInstance) Token() string {
	if i == nil || i.adapter == nil {
		return ""
	}

	rec, ok := i.adapter.record(i.id)
	if !ok {
		return ""
	}

	rec.mu.RLock()
	token := rec.token
	rec.mu.RUnlock()

	return token
}

func (i *memorySessionInstance) Get(key string, out any) (bool, error) {
	if i == nil || i.adapter == nil {
		return false, ErrSessionNotAvailable
	}

	if strings.TrimSpace(key) == "" {
		return false, ErrSessionInvalidKey
	}

	if out == nil {
		return false, ErrSessionInvalidOut
	}

	rec, ok := i.adapter.instanceRecord(i.id)
	if !ok {
		return false, ErrSessionNotAvailable
	}

	rec.mu.RLock()
	value, exists := rec.data[key]
	rec.mu.RUnlock()

	if !exists {
		return false, nil
	}

	return true, assign(out, value)
}

func (i *memorySessionInstance) Has(key string) bool {
	if i == nil || i.adapter == nil || strings.TrimSpace(key) == "" {
		return false
	}

	rec, ok := i.adapter.instanceRecord(i.id)
	if !ok {
		return false
	}

	rec.mu.RLock()
	_, exists := rec.data[key]
	rec.mu.RUnlock()

	return exists
}

func (i *memorySessionInstance) GetAll() map[string]any {
	if i == nil || i.adapter == nil {
		return nil
	}

	rec, ok := i.adapter.instanceRecord(i.id)
	if !ok {
		return nil
	}

	rec.mu.RLock()
	defer rec.mu.RUnlock()

	return cloneMap(rec.data)
}

func (i *memorySessionInstance) Set(key string, value any) error {
	if i == nil || i.adapter == nil {
		return ErrSessionNotAvailable
	}

	if strings.TrimSpace(key) == "" {
		return ErrSessionInvalidKey
	}

	rec, ok := i.adapter.instanceRecord(i.id)
	if !ok {
		return ErrSessionNotAvailable
	}

	rec.mu.Lock()
	rec.data[key] = cloneSessionValue(value)
	rec.updatedAt = time.Now()
	rec.mu.Unlock()

	return nil
}

func (i *memorySessionInstance) Delete(key string) error {
	if i == nil || i.adapter == nil {
		return ErrSessionNotAvailable
	}

	if strings.TrimSpace(key) == "" {
		return ErrSessionInvalidKey
	}

	rec, ok := i.adapter.instanceRecord(i.id)
	if !ok {
		return ErrSessionNotAvailable
	}

	rec.mu.Lock()
	delete(rec.data, key)
	rec.updatedAt = time.Now()
	rec.mu.Unlock()

	return nil
}

func (i *memorySessionInstance) SetToken(token string) error {
	if i == nil || i.adapter == nil {
		return ErrSessionNotAvailable
	}

	if strings.TrimSpace(token) == "" {
		return ErrSessionNotAvailable
	}

	rec, ok := i.adapter.instanceRecord(i.id)
	if !ok {
		return ErrSessionNotAvailable
	}

	rec.mu.Lock()
	rec.token = token
	rec.updatedAt = time.Now()
	rec.mu.Unlock()

	return nil
}

func (i *memorySessionInstance) Clear() error {
	if i == nil || i.adapter == nil {
		return ErrSessionNotAvailable
	}

	rec, ok := i.adapter.instanceRecord(i.id)
	if !ok {
		return ErrSessionNotAvailable
	}

	rec.mu.Lock()
	rec.data = make(map[string]any)
	rec.updatedAt = time.Now()
	rec.mu.Unlock()

	return nil
}

func (i *memorySessionInstance) SetTTL(ttl time.Duration) {
	if i == nil || i.adapter == nil || ttl <= 0 {
		return
	}

	rec, ok := i.adapter.instanceRecord(i.id)
	if !ok {
		return
	}

	now := time.Now()

	rec.mu.Lock()
	rec.ttl = ttl
	rec.expireAt = now.Add(ttl)
	rec.updatedAt = now
	expireAt := rec.expireAt
	rec.mu.Unlock()

	i.adapter.indexRecord(i.id, expireAt)
}

func (i *memorySessionInstance) GetTTL() time.Duration {
	if i == nil || i.adapter == nil {
		return 0
	}

	rec, ok := i.adapter.instanceRecord(i.id)
	if !ok {
		return 0
	}

	rec.mu.RLock()
	ttl := rec.ttl
	rec.mu.RUnlock()

	return ttl
}
