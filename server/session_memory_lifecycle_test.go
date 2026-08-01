package server

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMemorySessionTouchRenewsActiveSession(t *testing.T) {
	runtime := newMemorySessionRuntime(func() memorySessionOptions {
		return memorySessionOptions{StorageName: "memory-test", TTL: time.Minute}
	})
	t.Cleanup(func() { _ = runtime.Close() })

	now := time.Now()
	instance, err := runtime.CreateSession(context.Background(), &SessionMeta{
		ID: "session-touch", Token: "token", TTL: time.Minute,
		CreatedAt: now, UpdatedAt: now, ExpireAt: now.Add(time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := runtime.TouchSession(context.Background(), instance.ID(), 2*time.Hour); err != nil {
		t.Fatalf("TouchSession() error = %v", err)
	}
	if ttl := instance.GetTTL(); ttl != 2*time.Hour {
		t.Fatalf("configured TTL = %s, want 2h", ttl)
	}
}

func TestMemorySessionOpenPreservesConfiguredTTL(t *testing.T) {
	runtime := newMemorySessionRuntime(func() memorySessionOptions {
		return memorySessionOptions{StorageName: "memory-test", TTL: time.Hour}
	})
	t.Cleanup(func() { _ = runtime.Close() })

	now := time.Now()
	instance, err := runtime.CreateSession(context.Background(), &SessionMeta{
		ID: "custom-ttl", Token: "token", TTL: 15 * time.Minute,
		CreatedAt: now, UpdatedAt: now, ExpireAt: now.Add(15 * time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := runtime.OpenSession(context.Background(), SessionOpenRequest{
		ID: instance.ID(), Token: instance.Token(), TTL: time.Hour, Now: now.Add(time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Opened() {
		t.Fatalf("OpenSession() status = %v, want SessionOpenOpened", result.Status)
	}
	if ttl := result.Instance.GetTTL(); ttl != 15*time.Minute {
		t.Fatalf("configured TTL = %s, want 15m", ttl)
	}

	rec, ok := runtime.record(instance.ID())
	if !ok {
		t.Fatal("session record is missing")
	}
	rec.mu.RLock()
	expireAt := rec.expireAt
	rec.mu.RUnlock()
	if want := now.Add(16 * time.Minute); !expireAt.Equal(want) {
		t.Fatalf("ExpireAt = %s, want %s", expireAt, want)
	}
}

func TestMemorySessionTouchRejectsExpiredAndMissingSessions(t *testing.T) {
	runtime := newMemorySessionRuntime(nil)
	t.Cleanup(func() { _ = runtime.Close() })

	now := time.Now()
	_, err := runtime.CreateSession(context.Background(), &SessionMeta{
		ID: "expired", Token: "token", TTL: time.Minute,
		CreatedAt: now.Add(-2 * time.Minute), UpdatedAt: now.Add(-2 * time.Minute), ExpireAt: now.Add(-time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := runtime.TouchSession(context.Background(), "expired", time.Hour); !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("expired TouchSession() error = %v, want ErrSessionExpired", err)
	}
	if err := runtime.TouchSession(context.Background(), "missing", time.Hour); !errors.Is(err, ErrSessionNotAvailable) {
		t.Fatalf("missing TouchSession() error = %v, want ErrSessionNotAvailable", err)
	}
}

func TestMemorySessionPublishesDestroyedEvent(t *testing.T) {
	runtime := newMemorySessionRuntime(nil)
	t.Cleanup(func() { _ = runtime.Close() })

	now := time.Now()
	_, err := runtime.CreateSession(context.Background(), &SessionMeta{
		ID: "destroyed", Token: "token", TTL: time.Hour,
		CreatedAt: now, UpdatedAt: now, ExpireAt: now.Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}

	events, cancel := subscribeMemorySessionEvents(t, runtime)
	defer cancel()

	if err := runtime.DeleteSession(context.Background(), "destroyed"); err != nil {
		t.Fatal(err)
	}

	event := receiveSessionEvent(t, events)
	if event.Kind != SessionEventDestroyed || event.SessionID != "destroyed" || event.Session != nil {
		t.Fatalf("destroyed event = %#v", event)
	}
}

func TestMemorySessionPublishesExpiredEventWithSnapshot(t *testing.T) {
	runtime := newMemorySessionRuntime(nil)
	t.Cleanup(func() { _ = runtime.Close() })

	now := time.Now()
	instance, err := runtime.CreateSession(context.Background(), &SessionMeta{
		ID: "expired-event", Token: "token", TTL: time.Minute,
		CreatedAt: now.Add(-2 * time.Minute), UpdatedAt: now.Add(-time.Minute), ExpireAt: now.Add(-time.Second),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Set("user", "Javier"); err != nil {
		t.Fatal(err)
	}

	events, cancel := subscribeMemorySessionEvents(t, runtime)
	defer cancel()
	// The expiration index intentionally rounds entries up to the next minute.
	// Advance the explicit test clock past that bucket without sleeping.
	runtime.expireDueSessions(now.Add(time.Minute))

	event := receiveSessionEvent(t, events)
	if event.Kind != SessionEventExpired || event.SessionID != "expired-event" || event.Session == nil {
		t.Fatalf("expired event = %#v", event)
	}
	var user string
	if ok, err := event.Session.Get("user", &user); err != nil || !ok || user != "Javier" {
		t.Fatalf("expired snapshot user = %q, ok=%v, err=%v", user, ok, err)
	}
}

func subscribeMemorySessionEvents(t *testing.T, runtime *memorySessionRuntime) (<-chan SessionEvent, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	events := make(chan SessionEvent, 1)
	go func() {
		_ = runtime.SubscribeSessionEvents(ctx, func(event SessionEvent) { events <- event })
	}()
	deadline := time.Now().Add(time.Second)
	for {
		runtime.eventMu.RLock()
		ready := runtime.eventFn != nil
		runtime.eventMu.RUnlock()
		if ready {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("session event subscription did not become ready")
		}
		time.Sleep(time.Millisecond)
	}
	return events, cancel
}

func receiveSessionEvent(t *testing.T, events <-chan SessionEvent) SessionEvent {
	t.Helper()
	select {
	case event := <-events:
		return event
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for session event")
		return SessionEvent{}
	}
}

func TestDetachSessionPreservesStorageAndRemovesRequestRotation(t *testing.T) {
	runtime := newMemorySessionRuntime(nil)
	t.Cleanup(func() { _ = runtime.Close() })
	now := time.Now()
	instance, err := runtime.CreateSession(context.Background(), &SessionMeta{
		ID: "detached", Token: "token", TTL: time.Hour,
		CreatedAt: now, UpdatedAt: now, ExpireAt: now.Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	original := NewSessionWithRotate(instance, func() (string, error) { return "rotated", nil })
	if original.ID() != "detached" || original.Token() != "token" {
		t.Fatalf("session identity = %q/%q", original.ID(), original.Token())
	}
	if err := original.Rotate(); err != nil {
		t.Fatal(err)
	}
	if original.ID() != "detached" || original.Token() != "rotated" {
		t.Fatalf("rotated session identity = %q/%q", original.ID(), original.Token())
	}
	detached := DetachSession(original)
	if err := detached.Set("name", "Javier"); err != nil {
		t.Fatal(err)
	}
	var name string
	if ok, err := original.Get("name", &name); err != nil || !ok || name != "Javier" {
		t.Fatalf("shared value = %q, ok=%v, err=%v", name, ok, err)
	}
	if err := detached.Rotate(); !errors.Is(err, ErrSessionNotAvailable) {
		t.Fatalf("detached Rotate() error = %v, want ErrSessionNotAvailable", err)
	}
}

func TestSessionDestroyIsIdempotentAndRunsRequestCleanupOnce(t *testing.T) {
	runtime := newMemorySessionRuntime(nil)
	t.Cleanup(func() { _ = runtime.Close() })
	now := time.Now()
	instance, err := runtime.CreateSession(context.Background(), &SessionMeta{
		ID: "destroy-api", Token: "token", TTL: time.Hour,
		CreatedAt: now, UpdatedAt: now, ExpireAt: now.Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	destroyCalls := 0
	cleanupCalls := 0
	session := NewSessionWithLifecycle(instance, nil, func() error {
		destroyCalls++
		return runtime.DeleteSession(context.Background(), instance.ID())
	}, func() { cleanupCalls++ })

	if err := session.Destroy(); err != nil {
		t.Fatal(err)
	}
	if err := session.Destroy(); err != nil {
		t.Fatal(err)
	}
	if destroyCalls != 1 || cleanupCalls != 1 || !session.IsDestroyed() {
		t.Fatalf("destroy calls=%d cleanup calls=%d destroyed=%v", destroyCalls, cleanupCalls, session.IsDestroyed())
	}
	if err := session.Set("after", true); !errors.Is(err, ErrSessionNotAvailable) {
		t.Fatalf("Set after Destroy error = %v, want ErrSessionNotAvailable", err)
	}
}

func TestDetachedSessionDestroyDoesNotRetainRequestCleanup(t *testing.T) {
	runtime := newMemorySessionRuntime(nil)
	t.Cleanup(func() { _ = runtime.Close() })
	now := time.Now()
	instance, err := runtime.CreateSession(context.Background(), &SessionMeta{
		ID: "socket-destroy", Token: "token", TTL: time.Hour,
		CreatedAt: now, UpdatedAt: now, ExpireAt: now.Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	cleanupCalls := 0
	httpSession := NewSessionWithLifecycle(instance, nil, func() error {
		return runtime.DeleteSession(context.Background(), instance.ID())
	}, func() { cleanupCalls++ })
	socketSession := DetachSession(httpSession)
	if err := socketSession.Destroy(); err != nil {
		t.Fatal(err)
	}
	if cleanupCalls != 0 {
		t.Fatalf("detached session retained request cleanup: %d calls", cleanupCalls)
	}
	if !httpSession.IsDestroyed() {
		t.Fatal("HTTP and WebSocket copies did not share lifecycle state")
	}
}
