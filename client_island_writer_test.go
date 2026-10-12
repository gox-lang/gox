package gox

import (
	"errors"
	"testing"
)

type islandByteWriter struct {
	Writer
	failAt int
	calls  int
	err    error
}

func (*islandByteWriter) WriteRawString(string)  {}
func (*islandByteWriter) WriteTextString(string) {}
func (*islandByteWriter) WriteTextBytes([]byte)  {}
func (w *islandByteWriter) WriteByte(byte) error {
	w.calls++
	if w.calls == w.failAt {
		return w.err
	}
	return nil
}

type byteTestIslandRuntime struct{}

func (byteTestIslandRuntime) ResolveClientIsland(string) (ClientIslandSnapshot, bool) {
	return ClientIslandSnapshot{ScriptURL: "/menu.js"}, true
}
func (byteTestIslandRuntime) ClientRuntimeHTML() []byte { return nil }

func TestClientIslandByteFailuresPropagate(t *testing.T) {
	for _, failAt := range []int{1, 2} {
		t.Run(string(rune('0'+failAt)), func(t *testing.T) {
			sentinel := errors.New("transport failed")
			w := &islandByteWriter{failAt: failAt, err: sentinel}
			ctx := &Context{Writer: w}
			BindClientIslandRuntime(ctx, byteTestIslandRuntime{})
			children := false
			defer func() {
				if got := recover(); got != sentinel {
					t.Errorf("panic = %v, want original transport error", got)
				}
				if children {
					t.Error("children executed after failed write")
				}
				if w.calls != failAt {
					t.Errorf("byte calls = %d, want %d", w.calls, failAt)
				}
			}()
			WriteClientIsland(ctx, "menu", nil, func(*Context) { children = true })
		})
	}
}

func TestClientPropsByteFailurePropagates(t *testing.T) {
	sentinel := errors.New("transport failed")
	w := &islandByteWriter{failAt: 1, err: sentinel}
	defer func() {
		if got := recover(); got != sentinel {
			t.Errorf("panic = %v, want original transport error", got)
		}
	}()
	WriteClientPropsAttribute(w, map[string]any{"name": "GOX"})
}

func TestClientIslandSuccessfulBytes(t *testing.T) {
	w := &islandByteWriter{}
	ctx := &Context{Writer: w}
	BindClientIslandRuntime(ctx, byteTestIslandRuntime{})
	children := false
	WriteClientIsland(ctx, "menu", nil, func(*Context) { children = true })
	WriteClientPropsAttribute(w, map[string]any{"name": "GOX"})
	if !children || w.calls != 3 {
		t.Fatalf("children=%v, byte calls=%d", children, w.calls)
	}
}
