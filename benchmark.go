package gox

import "sync"

var benchmarkHandlerSection struct {
	mu sync.RWMutex
	fn func(func())
}

func SetBenchmarkHandlerSectionHook(fn func(func())) func() {
	benchmarkHandlerSection.mu.Lock()
	previous := benchmarkHandlerSection.fn
	benchmarkHandlerSection.fn = fn
	benchmarkHandlerSection.mu.Unlock()
	return func() {
		benchmarkHandlerSection.mu.Lock()
		benchmarkHandlerSection.fn = previous
		benchmarkHandlerSection.mu.Unlock()
	}
}

func BenchmarkHandlerSection(fn func()) {
	if fn == nil {
		return
	}
	benchmarkHandlerSection.mu.RLock()
	hook := benchmarkHandlerSection.fn
	benchmarkHandlerSection.mu.RUnlock()
	if hook == nil {
		fn()
		return
	}
	hook(fn)
}
