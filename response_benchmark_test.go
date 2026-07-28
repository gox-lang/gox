package gox

import "testing"

var (
	benchmarkHeaderString string
	benchmarkHeaderValues []string
	benchmarkHeaderBool   bool
	benchmarkHeaderCount  int
)

func BenchmarkResponseHeadersSetLowercase(b *testing.B) {
	response := NewResponse()
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		response.Headers.Set("content-type", "text/html")
	}
}

func BenchmarkResponseHeadersSetCanonical(b *testing.B) {
	response := NewResponse()
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		response.Headers.Set("Content-Type", "text/html")
	}
}

func BenchmarkResponseHeadersAppendLifecycle(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		response := NewResponse()
		response.Headers.Append("vary", "Accept-Encoding")
		response.Headers.Append("vary", "Origin")
		response.Headers.Append("vary", "User-Agent")
	}
}

func BenchmarkResponseHeadersAppendFreshStorage(b *testing.B) {
	response := NewResponse()
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		response.Headers.headers = nil
		response.Headers.Append("vary", "Accept-Encoding")
		response.Headers.Append("vary", "Origin")
		response.Headers.Append("vary", "User-Agent")
	}
}

func BenchmarkResponseHeadersAppendExistingLowercase(b *testing.B) {
	response := NewResponse()
	response.Headers.headers = map[string][]string{
		"Vary": make([]string, 1, 4),
	}
	response.Headers.headers["Vary"][0] = "Accept-Encoding"
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		response.Headers.headers["Vary"] = response.Headers.headers["Vary"][:1]
		response.Headers.Append("vary", "Origin")
	}
}

func BenchmarkResponseHeadersAppendExistingCanonical(b *testing.B) {
	response := NewResponse()
	response.Headers.headers = map[string][]string{
		"Vary": make([]string, 1, 4),
	}
	response.Headers.headers["Vary"][0] = "Accept-Encoding"
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		response.Headers.headers["Vary"] = response.Headers.headers["Vary"][:1]
		response.Headers.Append("Vary", "Origin")
	}
}

func BenchmarkResponseHeadersGetLowercase(b *testing.B) {
	response := NewResponse()
	response.Headers.Set("content-type", "text/html")
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		benchmarkHeaderString = response.Headers.Get("content-type")
	}
}

func BenchmarkResponseHeadersGetCanonical(b *testing.B) {
	response := NewResponse()
	response.Headers.Set("content-type", "text/html")
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		benchmarkHeaderString = response.Headers.Get("Content-Type")
	}
}

func BenchmarkResponseHeadersGetAll(b *testing.B) {
	response := NewResponse()
	response.Headers.Append("vary", "Accept-Encoding")
	response.Headers.Append("vary", "Origin")
	response.Headers.Append("vary", "User-Agent")
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		benchmarkHeaderValues = response.Headers.GetAll("vary")
	}
}

func BenchmarkResponseHeadersHas(b *testing.B) {
	response := NewResponse()
	response.Headers.Set("x-request-id", "123")
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		benchmarkHeaderBool = response.Headers.Has("X-Request-ID")
	}
}

func BenchmarkResponseHeadersRange(b *testing.B) {
	response := NewResponse()
	response.Headers.Set("content-type", "text/html")
	response.Headers.Set("cache-control", "no-store")
	response.Headers.Append("vary", "Accept-Encoding")
	response.Headers.Append("vary", "Origin")
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		count := 0
		response.Headers.Range(func(_ string, _ string, _ bool) bool {
			count++
			return true
		})
		benchmarkHeaderCount = count
	}
}
