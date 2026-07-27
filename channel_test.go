package gox

import (
	"errors"
	"sync"
	"testing"
)

func TestChannelIdentityLifecycleAndBroadcastContract(t *testing.T) {
	channel := NewChannel("Chat#venezuela", "Chat", "venezuela")
	first := &Socket{ID: "first"}
	second := &Socket{ID: "second"}
	members := []*Socket{first}
	emitted := ""
	left := ""
	closed := false
	BindChannelRuntime(channel, ChannelRuntimeBindings{
		Members: func() []*Socket { return members },
		Emit:    func(event string, _ []any) error { emitted = event; return nil },
		Leave:   func(socket *Socket) error { left = socket.ID; return nil },
		Close:   func() error { closed = true; return nil },
	})
	if channel.ID() != "Chat#venezuela" || channel.Name() != "Chat" || channel.Key() != "venezuela" {
		t.Fatal("channel identity is incorrect")
	}
	if channel.Count() != 1 || !channel.Has(first) || channel.Has(second) {
		t.Fatal("channel membership is incorrect")
	}
	members = append(members, second)
	if channel.Count() != 2 {
		t.Fatal("channel members are not dynamic")
	}
	channel.Emit("message", 42)
	if emitted != "message" {
		t.Fatalf("emitted event = %q", emitted)
	}
	if err := channel.Leave(second); err != nil || left != "second" {
		t.Fatalf("Leave: left=%q err=%v", left, err)
	}
	if err := channel.Close(); err != nil || !closed || !channel.IsClosed() {
		t.Fatalf("Close: closed=%t err=%v", closed, err)
	}
}

func TestChannelValidateIsStrictBoolAndErrorsAreIsolated(t *testing.T) {
	channel := NewChannel("Chat", "Chat", "")
	socket := &Socket{ID: "socket"}
	channel.When("validate", func(received *Socket, value string) bool { return received == socket && value == "token" })
	result, handled, err := channel.DispatchWhen("validate", socket, "token")
	if err != nil || !handled || result != true {
		t.Fatalf("validate: value=%v handled=%t err=%v", result, handled, err)
	}
	channel.When("validate", func(*Socket) string { return "wrong" })
	if _, _, err := channel.DispatchWhen("validate", socket); err == nil {
		t.Fatal("non-bool validate result was accepted")
	}

	received := make(chan ChannelError, 1)
	channel.When("error", func(err ChannelError) { received <- err })
	cause := errors.New("publish failed")
	BindChannelRuntime(channel, ChannelRuntimeBindings{Emit: func(string, []any) error { return cause }})
	channel.Emit("message")
	got := <-received
	if got.ChannelID != "Chat" || got.Event != "message" || !errors.Is(got, cause) {
		t.Fatalf("channel error = %#v", got)
	}
}

func TestChannelRegistryIsZeroSafeAndGenerationSafe(t *testing.T) {
	old := NewChannel("old", "old", "")
	current := NewChannel("current", "current", "")
	cleanupOld := BindChannelRegistry(ChannelRegistryBindings{List: func() []*Channel { return []*Channel{old} }})
	cleanupCurrent := BindChannelRegistry(ChannelRegistryBindings{
		Get: func(id string) (*Channel, bool) {
			if id == "current" {
				return current, true
			}
			return nil, false
		},
		List: func() []*Channel { return []*Channel{current} },
	})
	cleanupOld()
	if !Channels.Has("current") || Channels.Count() != 1 {
		t.Fatal("old cleanup replaced current channel registry")
	}
	cleanupCurrent()
	if Channels.Count() != 0 || Channels.List() == nil || Channels.Has("current") {
		t.Fatal("unbound channel registry is not safely empty")
	}
}

func TestChannelEmitUsesNamespacedSocketTransport(t *testing.T) {
	socket := &Socket{ID: "socket-1"}
	received := ""
	BindSocketChannelTransport(socket, SocketChannelTransport{Emit: func(channelID, event string, values []any) error {
		received = channelID + ":" + event
		return nil
	}})
	channel := NewChannel("Chat#room", "Chat", "room")
	BindChannelRuntime(channel, ChannelRuntimeBindings{Members: func() []*Socket { return []*Socket{socket} }, Emit: func(event string, values []any) error {
		return EmitSocketChannelEvent(socket, channel.ID(), event, values)
	}})
	channel.Emit("message", "hello")
	if received != "Chat#room:message" {
		t.Fatalf("namespaced emission = %q", received)
	}
}

func TestChannelGenerationCommitPreservesCoreAndInFlightDispatch(t *testing.T) {
	channel := NewChannel("Chat#room", "Chat", "room")
	socket := &Socket{ID: "socket-1"}
	members := []*Socket{socket}
	emitted := 0
	BindChannelRuntime(channel, ChannelRuntimeBindings{
		Members: func() []*Socket { return members },
		Emit:    func(string, []any) error { emitted++; return nil },
	})

	started := make(chan struct{})
	release := make(chan struct{})
	channel.On("version", func(*Socket) EventReply {
		close(started)
		<-release
		return "old"
	})

	var oldResult any
	var oldErr error
	var wait sync.WaitGroup
	wait.Add(1)
	go func() {
		defer wait.Done()
		oldResult, _, oldErr = channel.DispatchOn("version", socket)
	}()
	<-started

	candidate := NewChannelGenerationCandidate(channel, "v2")
	if candidate == nil {
		t.Fatal("generation candidate is nil")
	}
	if candidate.ID() != channel.ID() || candidate.Name() != channel.Name() || candidate.Key() != channel.Key() {
		t.Fatal("candidate did not preserve Channel identity")
	}
	if candidate.Count() != 1 || !candidate.Has(socket) {
		t.Fatal("candidate did not preserve Channel membership")
	}
	candidate.Emit("must-not-publish")
	if emitted != 0 {
		t.Fatal("staging generation emitted before commit")
	}
	candidate.On("version", func(*Socket) EventReply { return "new" })
	if err := CommitChannelGeneration(channel, candidate); err != nil {
		t.Fatal(err)
	}
	if channel.GenerationVersion() != "v2" {
		t.Fatalf("generation version = %q", channel.GenerationVersion())
	}

	newResult, handled, err := channel.DispatchOn("version", socket)
	if err != nil || !handled || newResult != "new" {
		t.Fatalf("new dispatch = %#v handled=%t err=%v", newResult, handled, err)
	}
	close(release)
	wait.Wait()
	if oldErr != nil || oldResult != "old" {
		t.Fatalf("in-flight dispatch = %#v err=%v", oldResult, oldErr)
	}

	candidate.Emit("after-commit")
	if emitted != 1 {
		t.Fatalf("committed candidate transport calls = %d", emitted)
	}
}

func TestChannelGenerationAbortAndCandidateCloseCannotMutateLiveChannel(t *testing.T) {
	channel := NewChannel("Chat", "Chat", "")
	channel.On("version", func() EventReply { return "stable" })

	aborted := NewChannelGenerationCandidate(channel, "aborted")
	aborted.On("version", func() EventReply { return "wrong" })
	AbortChannelGeneration(aborted)
	if err := CommitChannelGeneration(channel, aborted); err == nil {
		t.Fatal("aborted generation was committed")
	}
	value, _, err := channel.DispatchOn("version")
	if err != nil || value != "stable" {
		t.Fatalf("abort changed live generation: value=%v err=%v", value, err)
	}

	closed := NewChannelGenerationCandidate(channel, "closed")
	if err := closed.Close(); err != nil {
		t.Fatal(err)
	}
	if channel.IsClosed() {
		t.Fatal("closing candidate closed live Channel")
	}
	if err := CommitChannelGeneration(channel, closed); err == nil {
		t.Fatal("closed generation was committed")
	}
	if channel.IsClosed() {
		t.Fatal("failed candidate commit closed live Channel")
	}
}
