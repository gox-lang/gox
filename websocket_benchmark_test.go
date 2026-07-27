package gox

import "testing"

var benchmarkSocketReply any

func BenchmarkSocketDispatchOn(b *testing.B) {
	type payload struct {
		Text string
		ID   int
	}
	cases := []struct {
		name    string
		handler any
		args    []any
	}{
		{name: "NoArguments", handler: func() {}, args: nil},
		{name: "Text", handler: func(string) {}, args: []any{"hello websocket"}},
		{name: "Mixed", handler: func(string, int, bool) {}, args: []any{"hello websocket", 42, true}},
		{name: "Struct", handler: func(payload) {}, args: []any{payload{Text: "hello websocket", ID: 42}}},
		{name: "EventReply", handler: func(string) EventReply { return "ok" }, args: []any{"hello websocket"}},
	}
	for _, benchmark := range cases {
		b.Run(benchmark.name, func(b *testing.B) {
			socket := &Socket{}
			socket.On("event", benchmark.handler)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				value, handled, err := socket.DispatchOn("event", benchmark.args...)
				if err != nil || !handled {
					b.Fatalf("dispatch failed: handled=%v err=%v", handled, err)
				}
				benchmarkSocketReply = value
			}
		})
	}
}
