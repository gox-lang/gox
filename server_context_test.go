package gox

import "testing"

func TestSocketRegistryAreStableAndZeroSafe(t *testing.T) {
	if Sockets == nil {
		t.Fatal("gox.Sockets must always be available")
	}
	cleanup := BindSocketRegistry(SocketRegistryBindings{})
	defer cleanup()
	if Sockets.Count() != 0 || Sockets.All() == nil {
		t.Fatal("unbound server sockets must behave as an empty collection")
	}
	if socket, found := Sockets.GetByID("missing"); found || socket != nil {
		t.Fatal("unbound server sockets found an unknown ID")
	}
	if collection := Sockets.FindBySessionID("missing"); collection.Count() != 0 || collection.All() == nil {
		t.Fatal("unknown session must return a safe empty collection")
	}
}

func TestSocketRegistryBindFindEmitDisconnectAndRemainDynamic(t *testing.T) {
	first := &Socket{ID: "first", Session: Session{ID: "session-1"}}
	second := &Socket{ID: "second", Session: Session{ID: "session-1"}}
	third := &Socket{ID: "third", Session: Session{ID: "session-2"}}
	current := []*Socket{first, third}

	cleanup := BindSocketRegistry(SocketRegistryBindings{
		All: func() []*Socket { return current },
		FindByID: func(id string) (*Socket, bool) {
			for _, socket := range current {
				if socket.ID == id {
					return socket, true
				}
			}
			return nil, false
		},
		FindBySessionID: func(sessionID string) []*Socket {
			result := []*Socket{}
			for _, socket := range current {
				if socket.Session.ID == sessionID {
					result = append(result, socket)
				}
			}
			return result
		},
	})
	defer cleanup()

	session := Sockets.FindBySessionID("session-1")
	if session.Count() != 1 || session.All()[0] != first {
		t.Fatal("initial dynamic session collection is incorrect")
	}
	current = append(current, second)
	if session.Count() != 2 || Sockets.Count() != 3 {
		t.Fatal("server collections did not reflect a later connection")
	}
	if socket, found := Sockets.GetByID("second"); !found || socket != second {
		t.Fatal("FindByID did not return the requested socket")
	}

	emitted := make(chan string, 3)
	disconnected := make(chan WSCloseCode, 3)
	for _, socket := range current {
		BindSocketEmit(socket, func(event string, _ []any) error {
			emitted <- event
			return nil
		})
		BindSocketDisconnect(socket, func(code WSCloseCode, _ string) {
			disconnected <- code
		})
	}
	Sockets.Emit("broadcast", 42)
	Sockets.Disconnect(WSClosePermanent)
	for index := 0; index < 3; index++ {
		if event := <-emitted; event != "broadcast" {
			t.Fatalf("event = %q", event)
		}
		if code := <-disconnected; code != WSClosePermanent {
			t.Fatalf("close code = %d", code)
		}
	}
}

func TestSocketRegistryOldCleanupCannotUnbindNewRuntime(t *testing.T) {
	oldSocket := &Socket{ID: "old"}
	newSocket := &Socket{ID: "new"}
	cleanupOld := BindSocketRegistry(SocketRegistryBindings{All: func() []*Socket { return []*Socket{oldSocket} }})
	cleanupNew := BindSocketRegistry(SocketRegistryBindings{All: func() []*Socket { return []*Socket{newSocket} }})
	defer cleanupNew()

	cleanupOld()
	if sockets := Sockets.All(); len(sockets) != 1 || sockets[0] != newSocket {
		t.Fatal("an old runtime cleanup removed the current binding")
	}
}

func TestSocketRegistryUsesHostEmitterWhenBound(t *testing.T) {
	received := make(chan string, 1)
	cleanup := BindSocketRegistry(SocketRegistryBindings{
		Emit: func(event string, values []any) {
			received <- event
		},
	})
	defer cleanup()
	Sockets.Emit("distributed", 42)
	if event := <-received; event != "distributed" {
		t.Fatalf("event = %q", event)
	}
}

func TestServerSessionCollectionUsesDistributedEmitter(t *testing.T) {
	received := make(chan string, 1)
	cleanup := BindSocketRegistry(SocketRegistryBindings{
		FindBySessionID: func(string) []*Socket { return []*Socket{} },
		EmitSession: func(sessionID, event string, values []any) {
			received <- sessionID + ":" + event
		},
	})
	defer cleanup()
	Sockets.FindBySessionID("session-1").Emit("updated", true)
	if value := <-received; value != "session-1:updated" {
		t.Fatalf("distributed session event = %q", value)
	}
}

func TestSocketRegistryDistributedDisconnectUsesOneRuntimeOperation(t *testing.T) {
	all := make(chan WSCloseCode, 1)
	session := make(chan string, 1)
	cleanup := BindSocketRegistry(SocketRegistryBindings{
		Disconnect:        func(code WSCloseCode) { all <- code },
		DisconnectSession: func(sessionID string, code WSCloseCode) { session <- sessionID },
	})
	defer cleanup()
	Sockets.Disconnect(WSClosePermanent)
	if code := <-all; code != WSClosePermanent {
		t.Fatalf("all close code = %d", code)
	}
	Sockets.FindBySessionID("session-1").Disconnect(WSClosePolicy)
	if id := <-session; id != "session-1" {
		t.Fatalf("session disconnect = %q", id)
	}
}

func TestSocketRegistryGetByIDReturnsOneSocketWhileCollectionsRemainBroadcastOnly(t *testing.T) {
	selected := &Socket{ID: "remote-socket"}
	cleanup := BindSocketRegistry(SocketRegistryBindings{
		FindByID: func(id string) (*Socket, bool) {
			if id == selected.ID {
				return selected, true
			}
			return nil, false
		},
	})
	defer cleanup()
	if socket, found := Sockets.GetByID("remote-socket"); !found || socket != selected {
		t.Fatalf("selected socket = %#v", socket)
	}
	if socket, found := Sockets.GetByID(""); found || socket != nil {
		t.Fatal("empty socket ID returned a target")
	}
}
