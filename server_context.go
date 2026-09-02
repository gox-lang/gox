package gox

import "sync"

// gox:internal
// SocketRegistryBindings connects the public SDK facade to the host-owned live
// socket registry. Application code should not construct these bindings.
type SocketRegistryBindings struct {
	All               func() []*Socket
	Count             func() int
	FindByID          func(id string) (*Socket, bool)
	FindBySessionID   func(sessionID string) []*Socket
	Emit              func(event string, values []any)
	EmitSession       func(sessionID string, event string, values []any)
	Disconnect        func(code WSCloseCode)
	DisconnectSession func(sessionID string, code WSCloseCode)
}

type socketRegistryState struct {
	mu         sync.RWMutex
	generation uint64
	bindings   SocketRegistryBindings
}

// SocketRegistry exposes every live socket owned by the active GOX runtime.
type SocketRegistry struct {
	state *socketRegistryState
}

// Sockets exposes every live socket known to the active GOX runtime. When a
// cluster is configured, this includes sockets owned by every instance.
var Sockets = newSocketRegistry()

func newSocketRegistry() *SocketRegistry {
	return &SocketRegistry{state: &socketRegistryState{}}
}

// gox:internal
// BindSocketRegistry installs host runtime bindings and returns an identity-safe
// cleanup. A cleanup from an older runtime cannot unbind a newer runtime.
func BindSocketRegistry(bindings SocketRegistryBindings) func() {
	if Sockets == nil {
		return func() {}
	}
	state := Sockets.state
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
				state.bindings = SocketRegistryBindings{}
			}
			state.mu.Unlock()
		})
	}
}

func (s *SocketRegistry) bindings() SocketRegistryBindings {
	if s == nil || s.state == nil {
		return SocketRegistryBindings{}
	}
	s.state.mu.RLock()
	bindings := s.state.bindings
	s.state.mu.RUnlock()
	return bindings
}

func (s *SocketRegistry) resolveAll() []*Socket {
	resolve := s.bindings().All
	if resolve == nil {
		return []*Socket{}
	}
	return resolve()
}

// All returns a snapshot of every currently connected socket.
func (s *SocketRegistry) All() []*Socket {
	resolved := s.resolveAll()
	result := make([]*Socket, 0, len(resolved))
	for _, socket := range resolved {
		if socket != nil {
			result = append(result, socket)
		}
	}
	return result
}

func (s *SocketRegistry) Count() int {
	if count := s.bindings().Count; count != nil {
		return count()
	}
	return len(s.All())
}

func (s *SocketRegistry) Emit(event string, values ...any) {
	if emit := s.bindings().Emit; emit != nil {
		emit(event, append([]any(nil), values...))
		return
	}
	for _, socket := range s.All() {
		socket.Emit(event, values...)
	}
}

func (s *SocketRegistry) Disconnect(code WSCloseCode) {
	if disconnect := s.bindings().Disconnect; disconnect != nil {
		disconnect(code)
		return
	}
	for _, socket := range s.All() {
		socket.Disconnect(code)
	}
}

// GetByID returns one live socket regardless of which GOX instance owns its
// physical connection.
func (s *SocketRegistry) GetByID(id string) (*Socket, bool) {
	if id == "" {
		return nil, false
	}
	find := s.bindings().FindByID
	if find == nil {
		return nil, false
	}
	return find(id)
}

// FindBySessionID returns a dynamic collection containing every socket in the
// session. Unlike socket.Siblings, it does not exclude a current socket.
func (s *SocketRegistry) FindBySessionID(sessionID string) SocketCollection {
	resolve := func() []*Socket {
		if sessionID == "" {
			return []*Socket{}
		}
		find := s.bindings().FindBySessionID
		if find == nil {
			return []*Socket{}
		}
		return find(sessionID)
	}
	return newSocketCollectionWithTransport(resolve, func(event string, values []any) {
		bindings := s.bindings()
		if bindings.EmitSession != nil && sessionID != "" {
			bindings.EmitSession(sessionID, event, values)
			return
		}
		for _, socket := range resolve() {
			socket.Emit(event, values...)
		}
	}, func(code WSCloseCode) {
		bindings := s.bindings()
		if bindings.DisconnectSession != nil && sessionID != "" {
			bindings.DisconnectSession(sessionID, code)
			return
		}
		for _, socket := range resolve() {
			socket.Disconnect(code)
		}
	})
}
