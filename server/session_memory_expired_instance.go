package server

import (
	"strings"
	"time"
)

// memoryExpiredSessionInstance is a detached, read-only snapshot. Expiration
// callbacks may safely inspect it after the live session record is deleted.
type memoryExpiredSessionInstance struct {
	id    string
	token string
	ttl   time.Duration
	data  map[string]any
}

func (i *memoryExpiredSessionInstance) ID() string {
	if i == nil {
		return ""
	}
	return i.id
}

func (i *memoryExpiredSessionInstance) Token() string {
	if i == nil {
		return ""
	}
	return i.token
}

func (i *memoryExpiredSessionInstance) Get(key string, out any) (bool, error) {
	if i == nil || i.id == "" {
		return false, ErrSessionNotAvailable
	}
	if strings.TrimSpace(key) == "" {
		return false, ErrSessionInvalidKey
	}
	if out == nil {
		return false, ErrSessionInvalidOut
	}
	value, ok := i.data[key]
	if !ok {
		return false, nil
	}
	return true, assign(out, value)
}

func (i *memoryExpiredSessionInstance) Has(key string) bool {
	if i == nil || strings.TrimSpace(key) == "" {
		return false
	}
	_, ok := i.data[key]
	return ok
}

func (i *memoryExpiredSessionInstance) GetAll() map[string]any {
	if i == nil {
		return nil
	}
	return cloneMap(i.data)
}

func (*memoryExpiredSessionInstance) Set(string, any) error {
	return ErrSessionNotAvailable
}

func (*memoryExpiredSessionInstance) Delete(string) error {
	return ErrSessionNotAvailable
}

func (*memoryExpiredSessionInstance) SetToken(string) error {
	return ErrSessionNotAvailable
}

func (*memoryExpiredSessionInstance) Clear() error {
	return ErrSessionNotAvailable
}

func (*memoryExpiredSessionInstance) SetTTL(time.Duration) {}

func (i *memoryExpiredSessionInstance) GetTTL() time.Duration {
	if i == nil {
		return 0
	}
	return i.ttl
}

var _ SessionStorageInstance = (*memoryExpiredSessionInstance)(nil)
