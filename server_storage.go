package gox

import (
	"fmt"
	"reflect"
	"sync"
)

type ServerStorage interface {
	Set(key string, value any)
	Get(key string, out any) (bool, error)
	Has(key string) bool
	Delete(key string)
}

// Storage is the process-wide application storage provided by the active GOX
// runtime. It is safe for concurrent use.
var Storage ServerStorage = newMemoryServerStorage()

type memoryServerStorage struct {
	mu     sync.RWMutex
	values map[string]any
}

func newMemoryServerStorage() *memoryServerStorage {
	return &memoryServerStorage{values: make(map[string]any)}
}

func (s *memoryServerStorage) Set(key string, value any) {
	s.mu.Lock()
	s.values[key] = value
	s.mu.Unlock()
}

func (s *memoryServerStorage) Get(key string, out any) (bool, error) {
	target, err := serverStorageTarget(out)
	if err != nil {
		return false, err
	}

	s.mu.RLock()
	value, ok := s.values[key]
	s.mu.RUnlock()
	if !ok {
		return false, nil
	}

	if value == nil {
		if !serverStorageCanSetNil(target) {
			return false, fmt.Errorf("gox server storage: key %q stores nil, target %s cannot receive nil", key, target.Type())
		}
		target.Set(reflect.Zero(target.Type()))
		return true, nil
	}

	stored := reflect.ValueOf(value)
	if !stored.Type().AssignableTo(target.Type()) {
		return false, fmt.Errorf("gox server storage: key %q has type %s, target expects %s", key, stored.Type(), target.Type())
	}

	target.Set(stored)
	return true, nil
}

func (s *memoryServerStorage) Has(key string) bool {
	s.mu.RLock()
	_, ok := s.values[key]
	s.mu.RUnlock()
	return ok
}

func (s *memoryServerStorage) Delete(key string) {
	s.mu.Lock()
	delete(s.values, key)
	s.mu.Unlock()
}

func serverStorageTarget(out any) (reflect.Value, error) {
	if out == nil {
		return reflect.Value{}, fmt.Errorf("gox server storage: out must be a non-nil pointer")
	}
	v := reflect.ValueOf(out)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return reflect.Value{}, fmt.Errorf("gox server storage: out must be a non-nil pointer")
	}
	return v.Elem(), nil
}

func serverStorageCanSetNil(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return true
	default:
		return false
	}
}
