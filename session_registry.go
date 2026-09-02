package gox

import (
	"strings"
	"sync"

	"github.com/gox-lang/gox/server"
)

// gox:internal
// SessionRegistryBindings connects the public SDK facade to the host-owned
// session storage. Application code should not construct these bindings.
type SessionRegistryBindings struct {
	Get func(sessionID string) (*Session, error)
}

type sessionRegistryState struct {
	mu         sync.RWMutex
	generation uint64
	bindings   SessionRegistryBindings
}

// SessionRegistry provides access to sessions outside their owning request.
type SessionRegistry struct {
	state *sessionRegistryState
}

// Sessions exposes the sessions managed by the active GOX runtime.
var Sessions = newSessionRegistry()

func newSessionRegistry() *SessionRegistry {
	return &SessionRegistry{state: &sessionRegistryState{}}
}

// gox:internal
// BindSessionRegistry installs host runtime bindings and returns an
// identity-safe cleanup. A cleanup from an older runtime cannot unbind a newer
// runtime.
func BindSessionRegistry(bindings SessionRegistryBindings) func() {
	if Sessions == nil {
		return func() {}
	}
	state := Sessions.state
	state.mu.Lock()
	state.generation++
	generation := state.generation
	state.bindings = bindings
	state.mu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			state.mu.Lock()
			if state.generation == generation {
				state.bindings = SessionRegistryBindings{}
			}
			state.mu.Unlock()
		})
	}
}

func (s *SessionRegistry) bindings() SessionRegistryBindings {
	if s == nil || s.state == nil {
		return SessionRegistryBindings{}
	}
	s.state.mu.RLock()
	bindings := s.state.bindings
	s.state.mu.RUnlock()
	return bindings
}

// Get returns an existing session by ID. Changes must be persisted explicitly
// with Session.Save.
func (s *SessionRegistry) Get(sessionID string) (*Session, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, server.ErrSessionNotAvailable
	}
	get := s.bindings().Get
	if get == nil {
		return nil, server.ErrSessionNotAvailable
	}
	return get(sessionID)
}
