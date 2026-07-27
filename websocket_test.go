package gox

import (
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestSocketEmitQueuesUntilRuntimeBinding(t *testing.T) {
	socket := &Socket{}
	socket.Emit("first", 1)
	socket.Emit("second", "two")
	var events []string
	BindSocketEmit(socket, func(event string, values []any) error {
		events = append(events, event)
		return nil
	})
	socket.Emit("third", true)
	if !reflect.DeepEqual(events, []string{"first", "second", "third"}) {
		t.Fatalf("events = %v", events)
	}
}

func TestSocketLocationIsImmutableAndNilSafe(t *testing.T) {
	var missing *Socket
	if missing.IsLocal() || missing.InstanceID() != "" {
		t.Fatal("nil socket exposed a location")
	}
	socket := &Socket{}
	BindSocketLocation(socket, "instance-a", true)
	BindSocketLocation(socket, "instance-b", false)
	if !socket.IsLocal() || socket.InstanceID() != "instance-a" {
		t.Fatalf("socket location = local:%v instance:%q", socket.IsLocal(), socket.InstanceID())
	}
}

func TestSocketEmitAwaitUsesTypedDestination(t *testing.T) {
	socket := &Socket{}
	BindSocketRequest(socket, func(event string, values []any, out any) error {
		if event != "get-theme" || !reflect.DeepEqual(values, []any{7}) {
			t.Fatalf("request = %q %#v", event, values)
		}
		*(out.(*string)) = "dark"
		return nil
	})
	var theme string
	if err := socket.EmitAwait(&theme, "get-theme", 7); err != nil {
		t.Fatal(err)
	}
	if theme != "dark" {
		t.Fatalf("theme = %q", theme)
	}
}

func TestSocketEmitAwaitRejectsInvalidDestination(t *testing.T) {
	socket := &Socket{}
	if err := socket.EmitAwait(nil, "event"); err == nil {
		t.Fatal("nil destination was accepted")
	}
	if err := socket.EmitAwait("value", "event"); err == nil {
		t.Fatal("non-pointer destination was accepted")
	}
	var destination *string
	if err := socket.EmitAwait(destination, "event"); err == nil {
		t.Fatal("nil pointer destination was accepted")
	}
}

func TestSocketOnParameterTypes(t *testing.T) {
	socket := &Socket{}
	socket.On("user", func(string, int, bool) {})
	types, handled, err := socket.OnParameterTypes("user")
	if err != nil || !handled || len(types) != 3 {
		t.Fatalf("types = %v, handled = %v, err = %v", types, handled, err)
	}
	if types[0].Kind() != reflect.String || types[1].Kind() != reflect.Int || types[2].Kind() != reflect.Bool {
		t.Fatalf("types = %v", types)
	}
}

func TestSocketOnNamedPreservesCompilerParameterNames(t *testing.T) {
	socket := &Socket{}
	socket.OnRegistered("profile", []string{"name", "age", "avatar"}, true, func(string, int, File) EventReply { return nil })
	names, handled := socket.OnParameterNames("profile")
	if !handled || !reflect.DeepEqual(names, []string{"name", "age", "avatar"}) {
		t.Fatalf("names = %v, handled = %v", names, handled)
	}
	names[0] = "changed"
	again, _ := socket.OnParameterNames("profile")
	if again[0] != "name" {
		t.Fatalf("parameter names were not copied: %v", again)
	}
	signature, _ := socket.OnSignature("profile")
	if !signature.HasReply {
		t.Fatal("reply contract was not preserved")
	}
}

func TestSocketOnRegisteredValidatesReplyContract(t *testing.T) {
	socket := &Socket{}
	socket.OnRegistered("profile", nil, true, func(string) {})
	_, handled, err := socket.DispatchOn("profile", "profile-42")
	if !handled || err == nil || !strings.Contains(err.Error(), "reply contract") {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
}

func TestSocketOnRegisteredPreservesZeroParameterSignatureAsEmptySlice(t *testing.T) {
	socket := &Socket{}
	socket.OnRegistered("ready", []string{}, true, func() EventReply { return true })
	signature, ok := socket.OnSignature("ready")
	if !ok {
		t.Fatal("signature is missing")
	}
	if signature.ParameterNames == nil || len(signature.ParameterNames) != 0 {
		t.Fatalf("parameter names = %#v, want non-nil empty slice", signature.ParameterNames)
	}
}

type socketTestMessage struct {
	Text string
}

func TestSocketDispatchWhenReturnsHandlerValue(t *testing.T) {
	socket := &Socket{}
	ctx := &Context{}
	socket.When("handshake", func(received *Context) bool {
		return received == ctx
	})

	value, handled, err := socket.DispatchWhen("handshake", ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !handled {
		t.Fatal("handshake was not handled")
	}
	allowed, ok := value.(bool)
	if !ok || !allowed {
		t.Fatalf("handshake value = %#v", value)
	}
}

func TestGoxSocketErrorImplementsErrorAndUnwrapsCause(t *testing.T) {
	cause := errors.New("native failure")
	err := NewGoxSocketError(
		GoxSocketTransportError,
		"WebSocket transport failed",
		GoxSocketPhaseTransport,
		"read",
		true,
		cause,
	)
	var standard error = err
	if standard.Error() != "WebSocket transport failed" {
		t.Fatalf("message = %q", standard.Error())
	}
	if !errors.Is(standard, cause) {
		t.Fatal("GoxSocketError does not unwrap its cause")
	}
	if err.Code != GoxSocketTransportError || err.Phase != GoxSocketPhaseTransport || err.Event != "read" || !err.Retryable {
		t.Fatalf("unexpected socket error: %#v", err)
	}
}

func TestSocketDisconnectUsesBoundRuntime(t *testing.T) {
	socket := &Socket{}
	var code WSCloseCode
	var reason string
	BindSocketDisconnect(socket, func(receivedCode WSCloseCode, receivedReason string) {
		code = receivedCode
		reason = receivedReason
	})
	socket.Disconnect(WSClosePermanent)
	if code != WSClosePermanent || reason != WSCloseReason(WSClosePermanent) {
		t.Fatalf("disconnect = (%d, %q)", code, reason)
	}
}

func TestSocketDisconnectZeroUsesNormalClosure(t *testing.T) {
	socket := &Socket{}
	var code WSCloseCode
	var reason string
	BindSocketDisconnect(socket, func(receivedCode WSCloseCode, receivedReason string) {
		code = receivedCode
		reason = receivedReason
	})
	socket.Disconnect(0)
	if code != WSCloseNormal || reason != "Normal closure" {
		t.Fatalf("disconnect zero = (%d, %q)", code, reason)
	}
}

func TestSocketDispatchOnUsesStrictTypedPayload(t *testing.T) {
	socket := &Socket{}
	socket.On("message", func(message socketTestMessage) string {
		return "received:" + message.Text
	})

	value, handled, err := socket.DispatchOn("message", socketTestMessage{Text: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if !handled || value != "received:hello" {
		t.Fatalf("value=%#v handled=%v", value, handled)
	}

	_, handled, err = socket.DispatchOn("message", map[string]string{"Text": "hello"})
	if !handled || err == nil || !strings.Contains(err.Error(), "expected gox.socketTestMessage") {
		t.Fatalf("expected strict payload type error, handled=%v err=%v", handled, err)
	}
}

func TestSocketDispatchUnknownEvent(t *testing.T) {
	socket := &Socket{}
	value, handled, err := socket.DispatchWhen("unknown")
	if value != nil || handled || err != nil {
		t.Fatalf("value=%#v handled=%v err=%v", value, handled, err)
	}
}

func TestSocketRegistrationReplacesSameEvent(t *testing.T) {
	socket := &Socket{}
	socket.When("connect", func(bool) string { return "first" })
	socket.When("connect", func(bool) string { return "second" })

	value, handled, err := socket.DispatchWhen("connect", false)
	if err != nil || !handled || value != "second" {
		t.Fatalf("value=%#v handled=%v err=%v", value, handled, err)
	}
}

func TestSocketDispatchReleasesLockBeforeHandler(t *testing.T) {
	socket := &Socket{}
	socket.When("connect", func() {
		socket.When("disconnect", func() {})
		socket.On("message", func(string) {})
	})

	_, handled, err := socket.DispatchWhen("connect")
	if err != nil || !handled {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
	if _, ok := socket.whenHandler("disconnect"); !ok {
		t.Fatal("handler could not register lifecycle event during dispatch")
	}
	if _, ok := socket.onHandler("message"); !ok {
		t.Fatal("handler could not register communication event during dispatch")
	}
}

func TestSocketDispatchRecoversHandlerPanic(t *testing.T) {
	socket := &Socket{}
	socket.On("explode", func() {
		panic("boom")
	})

	value, handled, err := socket.DispatchOn("explode")
	if value != nil || !handled || err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("value=%#v handled=%v err=%v", value, handled, err)
	}
}

func TestSocketDispatchValidatesArgumentsAndReturnCount(t *testing.T) {
	socket := &Socket{}
	socket.When("connect", func(bool) {})
	_, handled, err := socket.DispatchWhen("connect")
	if !handled || err == nil || !strings.Contains(err.Error(), "expects 1 arguments") {
		t.Fatalf("handled=%v err=%v", handled, err)
	}

	socket.On("invalid-return", func() (string, error) {
		return "value", errors.New("error")
	})
	_, handled, err = socket.DispatchOn("invalid-return")
	if !handled || err == nil || !strings.Contains(err.Error(), "zero or one value") {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
}

func TestSocketDispatchAcceptsTypedNil(t *testing.T) {
	socket := &Socket{}
	socket.When("handshake", func(ctx *Context) bool { return ctx == nil })

	value, handled, err := socket.DispatchWhen("handshake", nil)
	if err != nil || !handled || value != true {
		t.Fatalf("value=%#v handled=%v err=%v", value, handled, err)
	}
}

func TestNilSocketDispatchIsUnhandled(t *testing.T) {
	var socket *Socket
	value, handled, err := socket.DispatchOn("message", "hello")
	if value != nil || handled || err != nil {
		t.Fatalf("value=%#v handled=%v err=%v", value, handled, err)
	}
}

func TestSocketConcurrentRegistrationAndDispatch(t *testing.T) {
	socket := &Socket{}
	socket.On("message", func(value int) int { return value })

	var wg sync.WaitGroup
	errs := make(chan error, 64)
	for i := 0; i < 64; i++ {
		value := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			socket.On("message", func(received int) int { return received })
			result, handled, err := socket.DispatchOn("message", value)
			if err != nil {
				errs <- err
				return
			}
			if !handled || result != value {
				errs <- errors.New("unexpected concurrent dispatch result")
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func TestSocketCollectionIsDynamicAndZeroSafe(t *testing.T) {
	var zero SocketCollection
	if zero.Count() != 0 || zero.All() == nil {
		t.Fatal("zero SocketCollection must behave as an empty collection")
	}

	first := &Socket{}
	second := &Socket{}
	current := []*Socket{first}
	collection := newSocketCollection(func() []*Socket {
		return current
	})
	if collection.Count() != 1 || collection.All()[0] != first {
		t.Fatal("collection did not resolve its current sockets")
	}
	current = append(current, second)
	if collection.Count() != 2 {
		t.Fatal("collection is a frozen snapshot instead of a dynamic view")
	}

	snapshot := collection.All()
	snapshot[0] = nil
	if collection.All()[0] != first {
		t.Fatal("All exposed the resolver slice")
	}
}

func TestSocketCollectionEmitsAndDisconnectsEveryCurrentSocket(t *testing.T) {
	first := &Socket{}
	second := &Socket{}
	type emission struct {
		event  string
		values []any
	}
	emissions := make(chan emission, 2)
	disconnections := make(chan WSCloseCode, 2)
	for _, socket := range []*Socket{first, second} {
		BindSocketEmit(socket, func(event string, values []any) error {
			emissions <- emission{event: event, values: values}
			return nil
		})
		BindSocketDisconnect(socket, func(code WSCloseCode, _ string) {
			disconnections <- code
		})
	}

	collection := newSocketCollection(func() []*Socket {
		return []*Socket{first, nil, second}
	})
	collection.Emit("updated", 42)
	collection.Disconnect(WSClosePermanent)
	for index := 0; index < 2; index++ {
		got := <-emissions
		if got.event != "updated" || len(got.values) != 1 || got.values[0] != 42 {
			t.Fatalf("unexpected emission: %#v", got)
		}
		if code := <-disconnections; code != WSClosePermanent {
			t.Fatalf("disconnect code = %d", code)
		}
	}
}
