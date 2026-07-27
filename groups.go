package gox

import (
	"fmt"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

const MaxGroupNameBytes = 128

// ValidateGroupName validates a group name without changing it. Group names
// are case-sensitive and are never normalized by GOX.
func ValidateGroupName(name string) error {
	if name == "" {
		return fmt.Errorf("group name cannot be empty")
	}
	if len(name) > MaxGroupNameBytes {
		return fmt.Errorf("group name exceeds %d bytes", MaxGroupNameBytes)
	}
	if !utf8.ValidString(name) {
		return fmt.Errorf("group name must contain valid UTF-8")
	}
	if strings.TrimSpace(name) != name {
		return fmt.Errorf("group name cannot start or end with whitespace")
	}
	for _, value := range name {
		if unicode.IsControl(value) {
			return fmt.Errorf("group name cannot contain control characters")
		}
	}
	return nil
}

type GroupRegistryBindings struct {
	List         func() []string
	Members      func(group string) []*Socket
	Count        func() int
	CountMembers func(group string) int
	EmitTo       func(group string, event string, values []any)
}

type groupRegistryState struct {
	mu         sync.RWMutex
	generation uint64
	bindings   GroupRegistryBindings
}

type GroupRegistry struct{ state *groupRegistryState }

// Groups exposes ephemeral socket groups known to the active GOX runtime.
var Groups = GroupRegistry{state: &groupRegistryState{}}

func BindGroups(bindings GroupRegistryBindings) func() {
	state := Groups.state
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
				state.bindings = GroupRegistryBindings{}
			}
			state.mu.Unlock()
		})
	}
}

func (groups *GroupRegistry) bindings() GroupRegistryBindings {
	if groups == nil || groups.state == nil {
		return GroupRegistryBindings{}
	}
	groups.state.mu.RLock()
	bindings := groups.state.bindings
	groups.state.mu.RUnlock()
	return bindings
}

func (groups *GroupRegistry) List() []string {
	if list := groups.bindings().List; list != nil {
		return append([]string(nil), list()...)
	}
	return []string{}
}

func (groups *GroupRegistry) Has(group string) bool {
	if ValidateGroupName(group) != nil {
		return false
	}
	return groups.CountMembers(group) > 0
}

func (groups *GroupRegistry) Count() int {
	if count := groups.bindings().Count; count != nil {
		return count()
	}
	return len(groups.List())
}

func (groups *GroupRegistry) Members(group string) SocketCollection {
	resolve := func() []*Socket {
		if ValidateGroupName(group) != nil {
			return []*Socket{}
		}
		if members := groups.bindings().Members; members != nil {
			return members(group)
		}
		return []*Socket{}
	}
	return newSocketCollectionWithEmit(resolve, func(event string, values []any) {
		groups.EmitTo(group, event, values...)
	})
}

func (groups *GroupRegistry) CountMembers(group string) int {
	if ValidateGroupName(group) != nil {
		return 0
	}
	if count := groups.bindings().CountMembers; count != nil {
		return count(group)
	}
	return groups.Members(group).Count()
}

func (groups *GroupRegistry) EmitTo(group string, event string, values ...any) {
	if ValidateGroupName(group) != nil || event == "" {
		return
	}
	if emit := groups.bindings().EmitTo; emit != nil {
		emit(group, event, append([]any(nil), values...))
		return
	}
	for _, socket := range groups.Members(group).All() {
		socket.Emit(event, values...)
	}
}
