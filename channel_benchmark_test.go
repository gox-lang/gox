package gox

import "testing"

var benchmarkChannelReply any

func BenchmarkChannelDispatchOn(b *testing.B) {
	type payload struct {
		Text string
		ID   int
	}
	socket := &Socket{ID: "benchmark-socket"}
	cases := []struct {
		name    string
		handler any
		args    []any
	}{
		{name: "SocketOnly", handler: func(*Socket) {}, args: []any{socket}},
		{name: "Text", handler: func(*Socket, string) {}, args: []any{socket, "hello websocket"}},
		{name: "Mixed", handler: func(*Socket, string, int, bool) {}, args: []any{socket, "hello websocket", 42, true}},
		{name: "Struct", handler: func(*Socket, payload) {}, args: []any{socket, payload{Text: "hello websocket", ID: 42}}},
		{name: "EventReply", handler: func(*Socket, string) EventReply { return "ok" }, args: []any{socket, "hello websocket"}},
	}
	for _, benchmark := range cases {
		b.Run(benchmark.name, func(b *testing.B) {
			channel := NewChannel("Chat#benchmark", "Chat", "benchmark")
			channel.On("event", benchmark.handler)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				value, handled, err := channel.DispatchOn("event", benchmark.args...)
				if err != nil || !handled {
					b.Fatalf("dispatch failed: handled=%v err=%v", handled, err)
				}
				benchmarkChannelReply = value
			}
		})
	}
}
