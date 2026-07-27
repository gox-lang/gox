package gox

import (
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
)

// Channel is one active logical channel instance layered over WebSocket
// connections. A channel may contain many sockets and therefore Emit is a
// broadcast operation; server-side channel emissions intentionally have no
// Await operation.
type Channel struct {
	core *channelCore

	generation     atomic.Pointer[channelGeneration]
	candidate      *channelGeneration
	candidateState atomic.Uint32
	candidateClose atomic.Bool
}

// channelCore is the stable identity, membership and transport of one logical
// Channel. It survives every handler-generation replacement.
type channelCore struct {
	mu sync.RWMutex

	id   string
	name string
	key  string

	membersRuntime  func() []*Socket
	emitRuntime     func(event string, values []any) error
	leaveRuntime    func(socket *Socket) error
	closeRuntime    func() error
	isClosedRuntime func() bool
	closed          bool
}

// channelGeneration is an independently configurable, immutable-by-swap set
// of handlers. Its maps remain mutex protected because application code may
// register handlers while the factory is configuring the generation.
type channelGeneration struct {
	mu           sync.RWMutex
	version      string
	when         map[string]any
	on           map[string]any
	onSignatures map[string]WSEventSignature
}

const (
	channelCandidateStaging uint32 = iota
	channelCandidateCommitted
	channelCandidateAborted
)

// ChannelHandler configures one on-demand logical channel definition.
type ChannelHandler func(channel *Channel)

// NewChannel constructs a channel definition or active instance. It is used
// by generated GOX code and the host runtime.
func NewChannel(id, name, key string) *Channel {
	channel := &Channel{core: &channelCore{id: id, name: name, key: key}}
	channel.generation.Store(&channelGeneration{})
	return channel
}

// NewChannelGenerationCandidate creates an isolated handler generation backed
// by the same stable Channel core. Configuration performed on the candidate
// cannot emit, leave or close the live Channel before it is committed.
func NewChannelGenerationCandidate(channel *Channel, version string) *Channel {
	if channel == nil || channel.core == nil {
		return nil
	}
	return &Channel{core: channel.core, candidate: &channelGeneration{version: version}}
}

// ValidateChannelGenerationCandidate verifies that a candidate can be
// committed without changing either candidate or the live Channel.
func ValidateChannelGenerationCandidate(channel, candidate *Channel) error {
	if channel == nil || channel.core == nil {
		return fmt.Errorf("channel generation target is unavailable")
	}
	if candidate == nil || candidate.core != channel.core || candidate.candidate == nil {
		return fmt.Errorf("channel generation candidate does not belong to target")
	}
	if candidate.candidateState.Load() != channelCandidateStaging {
		return fmt.Errorf("channel generation candidate is no longer pending")
	}
	if candidate.candidateClose.Load() {
		return fmt.Errorf("channel generation candidate closed while being configured")
	}
	return nil
}

// CommitChannelGeneration atomically makes candidate the handler generation
// used by future dispatches. Dispatches that already captured the previous
// generation continue safely with it.
func CommitChannelGeneration(channel, candidate *Channel) error {
	if err := ValidateChannelGenerationCandidate(channel, candidate); err != nil {
		if candidate != nil && candidate.candidate != nil && candidate.candidateClose.Load() {
			candidate.candidateState.Store(channelCandidateAborted)
		}
		return err
	}
	if !candidate.candidateState.CompareAndSwap(channelCandidateStaging, channelCandidateCommitted) {
		return fmt.Errorf("channel generation candidate is no longer pending")
	}
	channel.generation.Store(candidate.candidate)
	return nil
}

// AbortChannelGeneration discards a pending candidate without changing the
// live Channel. It is safe to call more than once.
func AbortChannelGeneration(candidate *Channel) {
	if candidate == nil || candidate.candidate == nil {
		return
	}
	candidate.candidateState.CompareAndSwap(channelCandidateStaging, channelCandidateAborted)
}

// GenerationVersion returns the version attached to the currently visible
// handler generation. An initial unversioned Channel returns an empty string.
func (channel *Channel) GenerationVersion() string {
	generation := channel.handlerGeneration()
	if generation == nil {
		return ""
	}
	generation.mu.RLock()
	version := generation.version
	generation.mu.RUnlock()
	return version
}

func (channel *Channel) handlerGeneration() *channelGeneration {
	if channel == nil {
		return nil
	}
	if channel.candidate != nil {
		return channel.candidate
	}
	return channel.generation.Load()
}

func (channel *Channel) candidateBlocksRuntime() bool {
	return channel != nil && channel.candidate != nil && channel.candidateState.Load() != channelCandidateCommitted
}

func (channel *Channel) ID() string {
	if channel == nil || channel.core == nil {
		return ""
	}
	channel.core.mu.RLock()
	id := channel.core.id
	channel.core.mu.RUnlock()
	return id
}

func (channel *Channel) Name() string {
	if channel == nil || channel.core == nil {
		return ""
	}
	channel.core.mu.RLock()
	name := channel.core.name
	channel.core.mu.RUnlock()
	return name
}

func (channel *Channel) Key() string {
	if channel == nil || channel.core == nil {
		return ""
	}
	channel.core.mu.RLock()
	key := channel.core.key
	channel.core.mu.RUnlock()
	return key
}

// ChannelRuntimeBindings connects an SDK channel object to its host-owned
// membership and transport. Application code should not construct it.
type ChannelRuntimeBindings struct {
	Members  func() []*Socket
	Emit     func(event string, values []any) error
	Leave    func(socket *Socket) error
	Close    func() error
	IsClosed func() bool
}

// BindChannelRuntime installs the active runtime operations for channel.
func BindChannelRuntime(channel *Channel, bindings ChannelRuntimeBindings) {
	if channel == nil || channel.core == nil {
		return
	}
	channel.core.mu.Lock()
	channel.core.membersRuntime = bindings.Members
	channel.core.emitRuntime = bindings.Emit
	channel.core.leaveRuntime = bindings.Leave
	channel.core.closeRuntime = bindings.Close
	channel.core.isClosedRuntime = bindings.IsClosed
	channel.core.mu.Unlock()
}

func (channel *Channel) Members() SocketCollection {
	return newSocketCollection(func() []*Socket {
		if channel == nil || channel.core == nil {
			return []*Socket{}
		}
		channel.core.mu.RLock()
		members := channel.core.membersRuntime
		channel.core.mu.RUnlock()
		if members == nil {
			return []*Socket{}
		}
		return members()
	})
}

func (channel *Channel) Count() int { return channel.Members().Count() }

func (channel *Channel) Has(socket *Socket) bool {
	if socket == nil {
		return false
	}
	for _, member := range channel.Members().All() {
		if member == socket || (member != nil && member.ID != "" && member.ID == socket.ID) {
			return true
		}
	}
	return false
}

func (channel *Channel) Emit(event string, values ...any) {
	if channel == nil || event == "" || channel.IsClosed() || channel.candidateBlocksRuntime() {
		return
	}
	channel.core.mu.RLock()
	emit := channel.core.emitRuntime
	channel.core.mu.RUnlock()
	if emit != nil {
		if err := emit(event, append([]any(nil), values...)); err != nil {
			channel.dispatchError(event, err)
		}
		return
	}
	for _, socket := range channel.Members().All() {
		socket.Emit(event, values...)
	}
}

func (channel *Channel) Leave(socket *Socket) error {
	if channel == nil || socket == nil {
		return nil
	}
	if channel.candidateBlocksRuntime() {
		return fmt.Errorf("channel generation candidate cannot change membership")
	}
	channel.core.mu.RLock()
	leave := channel.core.leaveRuntime
	channel.core.mu.RUnlock()
	if leave == nil {
		return fmt.Errorf("channel membership is not available")
	}
	return leave(socket)
}

func (channel *Channel) Close() error {
	if channel == nil {
		return nil
	}
	if channel.candidateBlocksRuntime() {
		if channel.candidateState.Load() == channelCandidateStaging {
			channel.candidateClose.Store(true)
		}
		return nil
	}
	if channel.core == nil {
		return nil
	}
	channel.core.mu.RLock()
	if channel.core.closed {
		channel.core.mu.RUnlock()
		return nil
	}
	closeRuntime := channel.core.closeRuntime
	channel.core.mu.RUnlock()
	if closeRuntime != nil {
		if err := closeRuntime(); err != nil {
			return err
		}
	}
	channel.core.mu.Lock()
	channel.core.closed = true
	channel.core.mu.Unlock()
	return nil
}

func (channel *Channel) IsClosed() bool {
	if channel == nil || channel.core == nil {
		return true
	}
	if channel.candidate != nil && channel.candidateState.Load() == channelCandidateAborted {
		return true
	}
	if channel.candidateBlocksRuntime() && channel.candidateClose.Load() {
		return true
	}
	channel.core.mu.RLock()
	closed := channel.core.closed
	isClosed := channel.core.isClosedRuntime
	channel.core.mu.RUnlock()
	return closed || (isClosed != nil && isClosed())
}

// When registers a lifecycle handler: validate, connected, disconnected or
// error. Validation handlers return bool; false is a normal rejection.
func (channel *Channel) When(event string, handler any) {
	if channel == nil || event == "" || handler == nil {
		return
	}
	generation := channel.handlerGeneration()
	if generation == nil {
		return
	}
	generation.mu.Lock()
	if generation.when == nil {
		generation.when = make(map[string]any)
	}
	generation.when[event] = handler
	generation.mu.Unlock()
}

func (channel *Channel) On(event string, handler any) {
	channel.OnRegistered(event, nil, handlerReturnsValue(handler), handler)
}

// OnNamed registers source parameter names when generated code does not have
// the complete event signature available yet.
func (channel *Channel) OnNamed(event string, parameterNames []string, handler any) {
	channel.OnRegistered(event, parameterNames, handlerReturnsValue(handler), handler)
}

func (channel *Channel) OnRegistered(event string, parameterNames []string, hasReply bool, handler any) {
	if channel == nil || event == "" || handler == nil {
		return
	}
	generation := channel.handlerGeneration()
	if generation == nil {
		return
	}
	generation.mu.Lock()
	if generation.on == nil {
		generation.on = make(map[string]any)
	}
	if generation.onSignatures == nil {
		generation.onSignatures = make(map[string]WSEventSignature)
	}
	generation.on[event] = handler
	generation.onSignatures[event] = WSEventSignature{ParameterNames: append([]string{}, parameterNames...), HasReply: hasReply}
	generation.mu.Unlock()
}

func (channel *Channel) DispatchWhen(event string, args ...any) (any, bool, error) {
	handler, ok := channel.whenHandler(event)
	if !ok {
		return nil, false, nil
	}
	value, err := dispatchChannelHandler("When", event, handler, args)
	if err == nil && event == "validate" {
		if _, valid := value.(bool); !valid {
			return nil, true, fmt.Errorf(`channel.When("validate") must return bool`)
		}
	}
	return value, true, err
}

func (channel *Channel) DispatchOn(event string, args ...any) (any, bool, error) {
	handler, signature, ok := channel.onHandlerAndSignature(event)
	if !ok {
		return nil, false, nil
	}
	if signature != nil {
		fn := reflect.ValueOf(handler)
		if fn.IsValid() && fn.Kind() == reflect.Func {
			hasReply := fn.Type().NumOut() == 1
			if hasReply != signature.HasReply {
				return nil, true, fmt.Errorf("channel.On(%q) reply contract does not match its handler", event)
			}
		}
	}
	returnValue, err := dispatchChannelHandler("On", event, handler, args)
	return returnValue, true, err
}

func (channel *Channel) OnSignature(event string) (WSEventSignature, bool) {
	if channel == nil || event == "" {
		return WSEventSignature{}, false
	}
	generation := channel.handlerGeneration()
	if generation == nil {
		return WSEventSignature{}, false
	}
	generation.mu.RLock()
	signature, ok := generation.onSignatures[event]
	generation.mu.RUnlock()
	signature.ParameterNames = append([]string{}, signature.ParameterNames...)
	return signature, ok
}

// OnParameterTypes exposes the strict input contract used by the runtime
// binder. Channel handlers receive the originating socket as their first
// parameter, followed by the client payload values.
func (channel *Channel) OnParameterTypes(event string) ([]reflect.Type, bool, error) {
	handler, handled := channel.onHandler(event)
	if !handled {
		return nil, false, nil
	}
	fn := reflect.ValueOf(handler)
	if !fn.IsValid() || fn.Kind() != reflect.Func || fn.IsNil() {
		return nil, true, fmt.Errorf("channel.On(%q) handler must be a non-nil function", event)
	}
	typeOf := fn.Type()
	if typeOf.IsVariadic() {
		return nil, true, fmt.Errorf("channel.On(%q) handler cannot be variadic", event)
	}
	types := make([]reflect.Type, typeOf.NumIn())
	for index := range types {
		types[index] = typeOf.In(index)
	}
	return types, true, nil
}

func (channel *Channel) OnParameterNames(event string) ([]string, bool) {
	signature, ok := channel.OnSignature(event)
	if !ok {
		return nil, false
	}
	return signature.ParameterNames, true
}

// WhenParameterTypes exposes a strict lifecycle handler contract to the host
// runtime. It is primarily used to bind arbitrary Channel validation data.
func (channel *Channel) WhenParameterTypes(event string) ([]reflect.Type, bool, error) {
	handler, handled := channel.whenHandler(event)
	if !handled {
		return nil, false, nil
	}
	fn := reflect.ValueOf(handler)
	if !fn.IsValid() || fn.Kind() != reflect.Func || fn.IsNil() {
		return nil, true, fmt.Errorf("channel.When(%q) handler must be a non-nil function", event)
	}
	typeOf := fn.Type()
	if typeOf.IsVariadic() {
		return nil, true, fmt.Errorf("channel.When(%q) handler cannot be variadic", event)
	}
	types := make([]reflect.Type, typeOf.NumIn())
	for index := range types {
		types[index] = typeOf.In(index)
	}
	return types, true, nil
}

func (channel *Channel) whenHandler(event string) (any, bool) {
	if channel == nil || event == "" {
		return nil, false
	}
	generation := channel.handlerGeneration()
	if generation == nil {
		return nil, false
	}
	generation.mu.RLock()
	handler, ok := generation.when[event]
	generation.mu.RUnlock()
	return handler, ok
}

func (channel *Channel) onHandler(event string) (any, bool) {
	if channel == nil || event == "" {
		return nil, false
	}
	generation := channel.handlerGeneration()
	if generation == nil {
		return nil, false
	}
	generation.mu.RLock()
	handler, ok := generation.on[event]
	generation.mu.RUnlock()
	return handler, ok
}

// onHandlerAndSignature reads both values from one generation so a concurrent
// commit can never pair an old handler with a new signature.
func (channel *Channel) onHandlerAndSignature(event string) (any, *WSEventSignature, bool) {
	if channel == nil || event == "" {
		return nil, nil, false
	}
	generation := channel.handlerGeneration()
	if generation == nil {
		return nil, nil, false
	}
	generation.mu.RLock()
	handler, ok := generation.on[event]
	signature, hasSignature := generation.onSignatures[event]
	generation.mu.RUnlock()
	if !ok {
		return nil, nil, false
	}
	if !hasSignature {
		return handler, nil, true
	}
	signature.ParameterNames = append([]string{}, signature.ParameterNames...)
	return handler, &signature, true
}

func (channel *Channel) dispatchError(event string, cause error) {
	err, ok := cause.(ChannelError)
	if !ok {
		err = ChannelError{Code: ChannelTransportError, ChannelID: channel.ID(), Event: event, Message: "Channel event could not be emitted", Retryable: true, Cause: cause}
	}
	_, _, _ = channel.DispatchWhen("error", err)
}

func dispatchChannelHandler(kind, event string, handler any, args []any) (value any, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			value = nil
			err = fmt.Errorf("channel.%s(%q) handler panic: %v", kind, event, recovered)
		}
	}()
	fn := reflect.ValueOf(handler)
	if !fn.IsValid() || fn.Kind() != reflect.Func || fn.IsNil() {
		return nil, fmt.Errorf("channel.%s(%q) handler must be a non-nil function", kind, event)
	}
	typeOf := fn.Type()
	if typeOf.IsVariadic() {
		return nil, fmt.Errorf("channel.%s(%q) handler cannot be variadic", kind, event)
	}
	if typeOf.NumIn() != len(args) {
		return nil, fmt.Errorf("channel.%s(%q) handler expects %d arguments, received %d", kind, event, typeOf.NumIn(), len(args))
	}
	if typeOf.NumOut() > 1 {
		return nil, fmt.Errorf("channel.%s(%q) handler must return zero or one value", kind, event)
	}
	input := make([]reflect.Value, len(args))
	for index, arg := range args {
		expected := typeOf.In(index)
		if arg == nil {
			if !socketHandlerAcceptsNil(expected) {
				return nil, fmt.Errorf("channel.%s(%q) argument %d cannot be nil", kind, event, index)
			}
			input[index] = reflect.Zero(expected)
			continue
		}
		actual := reflect.ValueOf(arg)
		if !actual.Type().AssignableTo(expected) {
			return nil, fmt.Errorf("channel.%s(%q) argument %d has type %s, expected %s", kind, event, index, actual.Type(), expected)
		}
		input[index] = actual
	}
	output := fn.Call(input)
	if len(output) == 1 {
		return output[0].Interface(), nil
	}
	return nil, nil
}

type channelRegistryState struct {
	mu         sync.RWMutex
	generation uint64
	bindings   ChannelRegistryBindings
}

type ChannelRegistryBindings struct {
	Get   func(id string) (*Channel, bool)
	List  func() []*Channel
	Count func() int
}

type ChannelRegistry struct{ state *channelRegistryState }

// Channels exposes active logical channel instances known to GOX. With a
// cluster configured it represents the global directory.
var Channels = &ChannelRegistry{state: &channelRegistryState{}}

func BindChannelRegistry(bindings ChannelRegistryBindings) func() {
	state := Channels.state
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
				state.bindings = ChannelRegistryBindings{}
			}
			state.mu.Unlock()
		})
	}
}

func (registry *ChannelRegistry) bindings() ChannelRegistryBindings {
	if registry == nil || registry.state == nil {
		return ChannelRegistryBindings{}
	}
	registry.state.mu.RLock()
	bindings := registry.state.bindings
	registry.state.mu.RUnlock()
	return bindings
}

func (registry *ChannelRegistry) Get(id string) (*Channel, bool) {
	if id == "" {
		return nil, false
	}
	if get := registry.bindings().Get; get != nil {
		return get(id)
	}
	return nil, false
}

func (registry *ChannelRegistry) Has(id string) bool { _, found := registry.Get(id); return found }

func (registry *ChannelRegistry) List() []*Channel {
	if list := registry.bindings().List; list != nil {
		return append([]*Channel(nil), list()...)
	}
	return []*Channel{}
}

func (registry *ChannelRegistry) Count() int {
	if count := registry.bindings().Count; count != nil {
		return count()
	}
	return len(registry.List())
}
