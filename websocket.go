package gox

import (
	"fmt"
	"reflect"
	"sync"
)

// WebSocket marks the WebSocket module entrypoint handled by the Gox compiler.
type WebSocket interface{}

// WebSocketHandler configures one socket connection.
type WebSocketHandler func(socket *Socket)

// EventReply marks an On handler as producing exactly one reply value.
// The GOX compiler uses this marker to expose the event request contract.
type EventReply = any

// WSEventSignature is the source-level contract recorded for an On handler.
type WSEventSignature struct {
	ParameterNames []string
	HasReply       bool
}

// WSCloseCode is a WebSocket close status code.
type WSCloseCode int

const (
	WSCloseDefault      WSCloseCode = 0
	WSCloseNormal       WSCloseCode = 1000
	WSCloseGoingAway    WSCloseCode = 1001
	WSCloseProtocol     WSCloseCode = 1002
	WSCloseUnsupported  WSCloseCode = 1003
	WSCloseAbnormal     WSCloseCode = 1006
	WSCloseInvalidData  WSCloseCode = 1007
	WSClosePolicy       WSCloseCode = 1008
	WSCloseMessageLarge WSCloseCode = 1009
	WSCloseInternal     WSCloseCode = 1011
	WSCloseRestart      WSCloseCode = 1012
	WSCloseTryLater     WSCloseCode = 1013

	WSClosePermanent        WSCloseCode = 4000
	WSCloseReconnect        WSCloseCode = 4001
	WSCloseSessionExpired   WSCloseCode = 4002
	WSCloseSessionDestroyed WSCloseCode = 4003
)

// WSCloseReason returns GOX's stable description for a close code.
func WSCloseReason(code WSCloseCode) string {
	if code == WSCloseDefault {
		code = WSCloseNormal
	}
	switch code {
	case WSCloseNormal:
		return "Normal closure"
	case WSCloseGoingAway:
		return "Going away"
	case WSCloseProtocol:
		return "Protocol error"
	case WSCloseUnsupported:
		return "Unsupported data"
	case WSCloseInvalidData:
		return "Invalid payload data"
	case WSClosePolicy:
		return "Policy violation"
	case WSCloseMessageLarge:
		return "Message too large"
	case WSCloseInternal:
		return "Internal server error"
	case WSCloseRestart:
		return "Server restarting"
	case WSCloseTryLater:
		return "Try again later"
	case WSClosePermanent:
		return "Permanent disconnection"
	case WSCloseReconnect:
		return "Reconnect requested"
	case WSCloseSessionExpired:
		return "Session expired"
	case WSCloseSessionDestroyed:
		return "Session destroyed"
	default:
		return "Connection closed"
	}
}

type WSDisconnectInitiator string

const (
	WSDisconnectInitiatorClient  WSDisconnectInitiator = "client"
	WSDisconnectInitiatorServer  WSDisconnectInitiator = "server"
	WSDisconnectInitiatorUnknown WSDisconnectInitiator = "unknown"
)

// WSDisconnectReason describes why a socket connection ended.
type WSDisconnectReason struct {
	Code          WSCloseCode
	Reason        string
	Clean         bool
	Initiator     WSDisconnectInitiator
	WillReconnect bool
}

// Socket represents one WebSocket connection between a client and the server.
type Socket struct {
	// Session is the same live session used by the HTTP upgrade request.
	Session Session

	// Handshake is an immutable snapshot of the HTTP upgrade request.
	Handshake SocketHandshake

	// Siblings dynamically represents every other live socket attached to this
	// socket's session. The current socket is never part of this collection.
	Siblings SocketCollection

	id            string
	identityBound bool
	instanceID    string
	local         bool

	mu                    sync.RWMutex
	when                  map[string]any
	on                    map[string]any
	onSignatures          map[string]WSEventSignature
	disconnectRuntime     func(code WSCloseCode, reason string)
	emitRuntime           func(event string, values []any) error
	packetRuntime         func(event string, packet []byte) error
	requestRuntime        func(event string, values []any, out any) error
	clusterRequestRuntime func(originInstanceID string, correlationID []byte, packet []byte) error
	clusterCancelRuntime  func(correlationID []byte) bool
	joinGroupRuntime      func(group string) error
	leaveGroupRuntime     func(group string) error
	leaveAllGroupsRuntime func() error
	groupsRuntime         func() []string
	emitToGroupRuntime    func(group string, event string, values []any)
	channelEmitRuntime    func(channelID string, event string, values []any) error
	channelLeaveRuntime   func(channelID string, reason ChannelDisconnectReason) error
	pendingEmits          []socketEmission
}

// ID uniquely identifies this connection for its complete lifetime. It is
// assigned by the GOX server after a successful WebSocket upgrade.
func (s *Socket) ID() string {
	if s == nil {
		return ""
	}
	s.mu.RLock()
	id := s.id
	s.mu.RUnlock()
	return id
}

// BindSocketIdentity reserves the immutable host-owned socket identifier and
// returns its one-time assignment function. It is reserved for the GOX runtime
// when constructing real sockets and remote proxies. Rebinding returns nil.
func BindSocketIdentity(s *Socket) func(string) {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	if s.identityBound {
		s.mu.Unlock()
		return nil
	}
	s.identityBound = true
	s.mu.Unlock()

	return func(id string) {
		if id == "" {
			return
		}
		s.mu.Lock()
		if s.id == "" {
			s.id = id
		}
		s.mu.Unlock()
	}
}

// SocketChannelTransport is reserved for the GOX Channel runtime.
type SocketChannelTransport struct {
	Emit  func(channelID string, event string, values []any) error
	Leave func(channelID string, reason ChannelDisconnectReason) error
}

func BindSocketChannelTransport(socket *Socket, transport SocketChannelTransport) {
	if socket == nil {
		return
	}
	socket.mu.Lock()
	socket.channelEmitRuntime = transport.Emit
	socket.channelLeaveRuntime = transport.Leave
	socket.mu.Unlock()
}

// EmitSocketChannelEvent writes one namespaced Channel event. It is reserved
// for host runtimes; application code emits through channel.Emit.
func EmitSocketChannelEvent(socket *Socket, channelID, event string, values []any) error {
	if socket == nil {
		return nil
	}
	socket.mu.RLock()
	emit := socket.channelEmitRuntime
	socket.mu.RUnlock()
	if emit == nil {
		return fmt.Errorf("socket Channel transport is not available")
	}
	return emit(channelID, event, append([]any(nil), values...))
}

func LeaveSocketChannel(socket *Socket, channelID string, reason ChannelDisconnectReason) error {
	if socket == nil {
		return nil
	}
	socket.mu.RLock()
	leave := socket.channelLeaveRuntime
	socket.mu.RUnlock()
	if leave == nil {
		return fmt.Errorf("socket Channel transport is not available")
	}
	return leave(channelID, reason)
}

// Join adds this socket to an ephemeral group. Joining the same group more
// than once is safe and has no additional effect.
func (s *Socket) Join(group string) error {
	if err := ValidateGroupName(group); err != nil {
		return err
	}
	if s == nil {
		return nil
	}
	s.mu.RLock()
	join := s.joinGroupRuntime
	s.mu.RUnlock()
	if join == nil {
		return fmt.Errorf("socket group membership is not available")
	}
	return join(group)
}

// Leave removes this socket from one group. Leaving a group that is not joined
// is safe and has no effect.
func (s *Socket) Leave(group string) error {
	if err := ValidateGroupName(group); err != nil {
		return err
	}
	if s == nil {
		return nil
	}
	s.mu.RLock()
	leave := s.leaveGroupRuntime
	s.mu.RUnlock()
	if leave == nil {
		return fmt.Errorf("socket group membership is not available")
	}
	return leave(group)
}

// LeaveAll removes this socket from every group it currently belongs to.
func (s *Socket) LeaveAll() error {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	leaveAll := s.leaveAllGroupsRuntime
	s.mu.RUnlock()
	if leaveAll == nil {
		return fmt.Errorf("socket group membership is not available")
	}
	return leaveAll()
}

// Groups returns a sorted snapshot of the groups joined by this socket.
func (s *Socket) Groups() []string {
	if s == nil {
		return []string{}
	}
	s.mu.RLock()
	groups := s.groupsRuntime
	s.mu.RUnlock()
	if groups == nil {
		return []string{}
	}
	return append([]string(nil), groups()...)
}

// EmitTo emits a normal socket event to every current member of group. The
// sender does not need to belong to the group.
func (s *Socket) EmitTo(group string, event string, values ...any) {
	if s == nil || event == "" || ValidateGroupName(group) != nil {
		return
	}
	s.mu.RLock()
	emitTo := s.emitToGroupRuntime
	s.mu.RUnlock()
	if emitTo != nil {
		emitTo(group, event, append([]any(nil), values...))
	}
}

// SocketGroupBindings attaches group membership and group emission to a
// socket. It is reserved for the GOX runtime.
type SocketGroupBindings struct {
	Join     func(group string) error
	Leave    func(group string) error
	LeaveAll func() error
	Groups   func() []string
	EmitTo   func(group string, event string, values []any)
}

func BindSocketGroups(s *Socket, bindings SocketGroupBindings) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.joinGroupRuntime = bindings.Join
	s.leaveGroupRuntime = bindings.Leave
	s.leaveAllGroupsRuntime = bindings.LeaveAll
	s.groupsRuntime = bindings.Groups
	s.emitToGroupRuntime = bindings.EmitTo
	s.mu.Unlock()
}

// IsLocal reports whether this socket's physical connection belongs to the
// current GOX server instance. It never changes during the socket lifetime.
func (s *Socket) IsLocal() bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	local := s.local
	s.mu.RUnlock()
	return local
}

// InstanceID identifies the GOX server instance that owns this socket.
func (s *Socket) InstanceID() string {
	if s == nil {
		return ""
	}
	s.mu.RLock()
	instanceID := s.instanceID
	s.mu.RUnlock()
	return instanceID
}

// BindSocketLocation assigns immutable host location metadata. It is reserved
// for the GOX runtime when constructing real sockets and remote proxies.
func BindSocketLocation(s *Socket, instanceID string, local bool) {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.instanceID == "" {
		s.instanceID = instanceID
		s.local = local
	}
	s.mu.Unlock()
}

// SocketCollection is a dynamic view over a group of live sockets. A zero
// collection is valid and behaves like an empty collection.
type SocketCollection struct {
	state *socketCollectionState
}

type socketCollectionState struct {
	mu         sync.RWMutex
	resolve    func() []*Socket
	emit       func(event string, values []any)
	disconnect func(code WSCloseCode)
}

func newSocketCollection(resolve func() []*Socket) SocketCollection {
	return SocketCollection{state: &socketCollectionState{resolve: resolve}}
}

func newSocketCollectionWithEmit(resolve func() []*Socket, emit func(event string, values []any)) SocketCollection {
	return SocketCollection{state: &socketCollectionState{resolve: resolve, emit: emit}}
}

func newSocketCollectionWithTransport(resolve func() []*Socket, emit func(event string, values []any), disconnect func(code WSCloseCode)) SocketCollection {
	return SocketCollection{state: &socketCollectionState{resolve: resolve, emit: emit, disconnect: disconnect}}
}

// BindSocketSiblings attaches the runtime resolver used by socket.Siblings.
// It is intended only for use by the GOX WebSocket runtime.
func BindSocketSiblings(socket *Socket, resolve func() []*Socket) {
	if socket == nil || resolve == nil {
		return
	}
	socket.Siblings = newSocketCollection(resolve)
}

// BindSocketSiblingsEmit attaches the distributed emitter used by
// socket.Siblings while preserving its dynamic local resolver.
func BindSocketSiblingsEmit(socket *Socket, resolve func() []*Socket, emit func(event string, values []any)) {
	if socket == nil || resolve == nil || emit == nil {
		return
	}
	socket.Siblings = newSocketCollectionWithEmit(resolve, emit)
}

// BindSocketSiblingsDisconnect attaches the distributed disconnect operation
// while preserving the collection's current resolver and emitter.
func BindSocketSiblingsDisconnect(socket *Socket, disconnect func(code WSCloseCode)) {
	if socket == nil || disconnect == nil || socket.Siblings.state == nil {
		return
	}
	socket.Siblings.state.mu.Lock()
	socket.Siblings.state.disconnect = disconnect
	socket.Siblings.state.mu.Unlock()
}

// All returns a snapshot of the sockets currently in the collection.
func (collection SocketCollection) All() []*Socket {
	if collection.state == nil {
		return []*Socket{}
	}
	collection.state.mu.RLock()
	resolve := collection.state.resolve
	collection.state.mu.RUnlock()
	if resolve == nil {
		return []*Socket{}
	}
	resolved := resolve()
	result := make([]*Socket, 0, len(resolved))
	for _, socket := range resolved {
		if socket != nil {
			result = append(result, socket)
		}
	}
	return result
}

// Count returns the number of sockets currently in the collection.
func (collection SocketCollection) Count() int {
	return len(collection.All())
}

// Emit sends an event to every socket currently in the collection.
func (collection SocketCollection) Emit(event string, values ...any) {
	if collection.state != nil {
		collection.state.mu.RLock()
		emit := collection.state.emit
		collection.state.mu.RUnlock()
		if emit != nil {
			emit(event, append([]any(nil), values...))
			return
		}
	}
	for _, socket := range collection.All() {
		socket.Emit(event, values...)
	}
}

// BindSocketPacket attaches the already-encoded packet writer used by the
// cluster runtime. Application code should continue using Emit.
func BindSocketPacket(s *Socket, write func(event string, packet []byte) error) {
	if s == nil || write == nil {
		return
	}
	s.mu.Lock()
	s.packetRuntime = write
	s.mu.Unlock()
}

// WriteSocketPacket writes one validated GOX protocol packet without encoding
// it again. It is intended only for host and cluster runtimes.
func WriteSocketPacket(s *Socket, event string, packet []byte) error {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	write := s.packetRuntime
	s.mu.RUnlock()
	if write == nil {
		return fmt.Errorf("socket packet transport is not ready")
	}
	return write(event, packet)
}

// Disconnect closes every socket currently in the collection.
func (collection SocketCollection) Disconnect(code WSCloseCode) {
	if collection.state != nil {
		collection.state.mu.RLock()
		disconnect := collection.state.disconnect
		collection.state.mu.RUnlock()
		if disconnect != nil {
			disconnect(code)
			return
		}
	}
	for _, socket := range collection.All() {
		socket.Disconnect(code)
	}
}

type socketEmission struct {
	event  string
	values []any
}

// Disconnect closes this connection using GOX's standard reason for code.
// Code zero is an alias for WSCloseNormal.
func (s *Socket) Disconnect(code WSCloseCode) {
	if s == nil {
		return
	}
	s.mu.RLock()
	disconnect := s.disconnectRuntime
	s.mu.RUnlock()
	if disconnect != nil {
		if code == WSCloseDefault {
			code = WSCloseNormal
		}
		disconnect(code, WSCloseReason(code))
	}
}

// BindSocketDisconnect attaches the transport used by Socket.Disconnect.
// It is intended only for use by the GOX WebSocket runtime.
func BindSocketDisconnect(s *Socket, disconnect func(code WSCloseCode, reason string)) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.disconnectRuntime = disconnect
	s.mu.Unlock()
}

// Emit sends one event to the client. Calls made before the transport is ready
// are preserved in order and flushed when the runtime binds the connection.
func (s *Socket) Emit(event string, values ...any) {
	if s == nil || event == "" {
		return
	}
	args := append([]any(nil), values...)
	s.mu.Lock()
	emit := s.emitRuntime
	if emit == nil {
		s.pendingEmits = append(s.pendingEmits, socketEmission{event: event, values: args})
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()
	if err := emit(event, args); err != nil {
		s.dispatchEmitError(event, err)
	}
}

// BindSocketEmit attaches the event writer and flushes queued emissions. It is
// intended only for use by the GOX WebSocket runtime.
func BindSocketEmit(s *Socket, emit func(event string, values []any) error) {
	if s == nil || emit == nil {
		return
	}
	s.mu.Lock()
	s.emitRuntime = emit
	pending := s.pendingEmits
	s.pendingEmits = nil
	s.mu.Unlock()
	for _, emission := range pending {
		if err := emit(emission.event, emission.values); err != nil {
			s.dispatchEmitError(emission.event, err)
		}
	}
}

// EmitAwait sends one request event and waits for its single reply to be
// assigned to out. GOX generates this call from Emit(...).Await(&out).
func (s *Socket) EmitAwait(out any, event string, values ...any) error {
	if err := validateSocketReplyDestination(out); err != nil {
		return err
	}
	if s == nil {
		return nil
	}
	if event == "" {
		_, _, _ = s.DispatchWhen("error", NewGoxSocketError(GoxSocketInvalidState, "Socket request event cannot be empty", GoxSocketPhaseProtocol, "", false, nil))
		return nil
	}
	s.mu.RLock()
	request := s.requestRuntime
	s.mu.RUnlock()
	if request == nil {
		_, _, _ = s.DispatchWhen("error", NewGoxSocketError(GoxSocketInvalidState, "Socket request transport is not ready", GoxSocketPhaseConnection, event, true, nil))
		return nil
	}
	return request(event, append([]any(nil), values...), out)
}

// BindSocketRequest attaches the request/reply transport used by generated
// Emit(...).Await(&out) calls.
func BindSocketRequest(s *Socket, request func(event string, values []any, out any) error) {
	if s == nil || request == nil {
		return
	}
	s.mu.Lock()
	s.requestRuntime = request
	s.mu.Unlock()
}

// BindSocketClusterRequest attaches the host-owned transport that routes one
// already validated distributed request through this individual socket.
func BindSocketClusterRequest(s *Socket, request func(originInstanceID string, correlationID []byte, packet []byte) error) {
	if s == nil || request == nil {
		return
	}
	s.mu.Lock()
	s.clusterRequestRuntime = request
	s.mu.Unlock()
}

// RouteSocketClusterRequest is reserved for the GOX cluster runtime. It does
// not exist on SocketCollection, because a request must have exactly one
// browser and exactly one reply.
func RouteSocketClusterRequest(s *Socket, originInstanceID string, correlationID []byte, packet []byte) error {
	if s == nil {
		return fmt.Errorf("distributed socket target is unavailable")
	}
	s.mu.RLock()
	request := s.clusterRequestRuntime
	s.mu.RUnlock()
	if request == nil {
		return fmt.Errorf("distributed socket request transport is not ready")
	}
	return request(originInstanceID, correlationID, packet)
}

// BindSocketClusterCancel attaches cancellation for a distributed request
// that was previously routed through this individual socket.
func BindSocketClusterCancel(s *Socket, cancel func(correlationID []byte) bool) {
	if s == nil || cancel == nil {
		return
	}
	s.mu.Lock()
	s.clusterCancelRuntime = cancel
	s.mu.Unlock()
}

// CancelSocketClusterRequest marks an in-flight distributed request as
// canceled. A later browser reply is consumed but never republished.
func CancelSocketClusterRequest(s *Socket, correlationID []byte) bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	cancel := s.clusterCancelRuntime
	s.mu.RUnlock()
	return cancel != nil && cancel(correlationID)
}

func validateSocketReplyDestination(out any) error {
	if out == nil {
		return fmt.Errorf("socket Await destination cannot be nil")
	}
	value := reflect.ValueOf(out)
	if value.Kind() != reflect.Pointer {
		return fmt.Errorf("socket Await destination must be a pointer, received %T", out)
	}
	if value.IsNil() {
		return fmt.Errorf("socket Await destination pointer cannot be nil")
	}
	return nil
}

func (s *Socket) dispatchEmitError(event string, err error) {
	socketErr, ok := err.(GoxSocketError)
	if !ok {
		socketErr = NewGoxSocketError(GoxSocketEncodeFailed, "WebSocket event could not be encoded", GoxSocketPhaseProtocol, event, false, err)
	}
	_, _, _ = s.DispatchWhen("error", socketErr)
}

// When registers a lifecycle event handler for this socket.
func (s *Socket) When(event string, handler any) {
	if s == nil || event == "" || handler == nil {
		return
	}

	s.mu.Lock()
	if s.when == nil {
		s.when = make(map[string]any)
	}
	s.when[event] = handler
	s.mu.Unlock()
}

// On registers a communication event handler for this socket.
func (s *Socket) On(event string, handler any) {
	s.OnRegistered(event, nil, handlerReturnsValue(handler), handler)
}

// OnNamed registers a communication handler together with source parameter
// names. It remains available for runtimes that do not provide full metadata.
func (s *Socket) OnNamed(event string, parameterNames []string, handler any) {
	s.OnRegistered(event, parameterNames, handlerReturnsValue(handler), handler)
}

// OnRegistered registers the complete compiler-provided event contract.
// Application code uses On; this method is reserved for generated GOX code.
func (s *Socket) OnRegistered(event string, parameterNames []string, hasReply bool, handler any) {
	if s == nil || event == "" || handler == nil {
		return
	}

	s.mu.Lock()
	if s.on == nil {
		s.on = make(map[string]any)
	}
	s.on[event] = handler
	if s.onSignatures == nil {
		s.onSignatures = make(map[string]WSEventSignature)
	}
	// Preserve an explicitly empty signature as [] instead of nil. The internal
	// signature protocol requires a JSON array even when an event has no args.
	s.onSignatures[event] = WSEventSignature{ParameterNames: append([]string{}, parameterNames...), HasReply: hasReply}
	s.mu.Unlock()
}

// DispatchWhen executes a registered lifecycle handler.
// It is intended for use by the Gox WebSocket runtime.
func (s *Socket) DispatchWhen(event string, args ...any) (value any, handled bool, err error) {
	handler, handled := s.whenHandler(event)
	if !handled {
		return nil, false, nil
	}
	value, err = dispatchSocketHandler("When", event, handler, args)
	return value, true, err
}

// DispatchOn executes a registered communication event handler.
// It is intended for use by the Gox WebSocket runtime.
func (s *Socket) DispatchOn(event string, args ...any) (value any, handled bool, err error) {
	handler, handled := s.onHandler(event)
	if !handled {
		return nil, false, nil
	}
	if signature, ok := s.OnSignature(event); ok {
		fn := reflect.ValueOf(handler)
		if fn.IsValid() && fn.Kind() == reflect.Func {
			hasReply := fn.Type().NumOut() == 1
			if signature.HasReply != hasReply {
				return nil, true, fmt.Errorf("socket.On(%q) reply contract does not match its handler", event)
			}
		}
	}
	value, err = dispatchSocketHandler("On", event, handler, args)
	return value, true, err
}

// OnParameterTypes returns the strict parameter contract of an On handler. It
// is intended for use by the GOX WebSocket runtime binder.
func (s *Socket) OnParameterTypes(event string) ([]reflect.Type, bool, error) {
	handler, handled := s.onHandler(event)
	if !handled {
		return nil, false, nil
	}
	fn := reflect.ValueOf(handler)
	if !fn.IsValid() || fn.Kind() != reflect.Func || fn.IsNil() {
		return nil, true, fmt.Errorf("socket.On(%q) handler must be a non-nil function", event)
	}
	typeOf := fn.Type()
	if typeOf.IsVariadic() {
		return nil, true, fmt.Errorf("socket.On(%q) handler cannot be variadic", event)
	}
	types := make([]reflect.Type, typeOf.NumIn())
	for index := range types {
		types[index] = typeOf.In(index)
	}
	return types, true, nil
}

// OnParameterNames returns the source-level parameter names recorded by the
// GOX compiler for an On handler.
func (s *Socket) OnParameterNames(event string) ([]string, bool) {
	signature, ok := s.OnSignature(event)
	if !ok {
		return nil, false
	}
	return signature.ParameterNames, true
}

// OnSignature returns the compiler-recorded contract for an On handler.
func (s *Socket) OnSignature(event string) (WSEventSignature, bool) {
	if s == nil || event == "" {
		return WSEventSignature{}, false
	}
	s.mu.RLock()
	signature, ok := s.onSignatures[event]
	s.mu.RUnlock()
	if !ok {
		return WSEventSignature{}, false
	}
	signature.ParameterNames = append([]string{}, signature.ParameterNames...)
	return signature, true
}

func handlerReturnsValue(handler any) bool {
	fn := reflect.ValueOf(handler)
	return fn.IsValid() && fn.Kind() == reflect.Func && !fn.IsNil() && fn.Type().NumOut() == 1
}

func (s *Socket) whenHandler(event string) (any, bool) {
	if s == nil || event == "" {
		return nil, false
	}
	s.mu.RLock()
	handler, ok := s.when[event]
	s.mu.RUnlock()
	return handler, ok
}

func (s *Socket) onHandler(event string) (any, bool) {
	if s == nil || event == "" {
		return nil, false
	}
	s.mu.RLock()
	handler, ok := s.on[event]
	s.mu.RUnlock()
	return handler, ok
}

func dispatchSocketHandler(kind string, event string, handler any, args []any) (value any, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			value = nil
			err = fmt.Errorf("socket.%s(%q) handler panic: %v", kind, event, recovered)
		}
	}()

	fn := reflect.ValueOf(handler)
	if !fn.IsValid() || fn.Kind() != reflect.Func || fn.IsNil() {
		return nil, fmt.Errorf("socket.%s(%q) handler must be a non-nil function", kind, event)
	}

	fnType := fn.Type()
	if fnType.IsVariadic() {
		return nil, fmt.Errorf("socket.%s(%q) handler cannot be variadic", kind, event)
	}
	if fnType.NumIn() != len(args) {
		return nil, fmt.Errorf("socket.%s(%q) handler expects %d arguments, received %d", kind, event, fnType.NumIn(), len(args))
	}
	if fnType.NumOut() > 1 {
		return nil, fmt.Errorf("socket.%s(%q) handler must return zero or one value", kind, event)
	}

	input := make([]reflect.Value, len(args))
	for i, arg := range args {
		expected := fnType.In(i)
		if arg == nil {
			if !socketHandlerAcceptsNil(expected) {
				return nil, fmt.Errorf("socket.%s(%q) argument %d cannot be nil; expected %s", kind, event, i, expected)
			}
			input[i] = reflect.Zero(expected)
			continue
		}

		actual := reflect.ValueOf(arg)
		if !actual.Type().AssignableTo(expected) {
			return nil, fmt.Errorf("socket.%s(%q) argument %d has type %s; expected %s", kind, event, i, actual.Type(), expected)
		}
		input[i] = actual
	}

	output := fn.Call(input)
	if len(output) == 1 {
		return output[0].Interface(), nil
	}
	return nil, nil
}

func socketHandlerAcceptsNil(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return true
	default:
		return false
	}
}
