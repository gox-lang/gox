package gox

import "testing"

func TestResponseHeadersSetUsesUnifiedStorage(t *testing.T) {
	res := NewResponse()
	res.Headers.Set("Content-Type", "text/html; charset=utf-8")

	if got := res.Headers.Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("unexpected header value: %q", got)
	}
	values := res.Headers.GetAll("Content-Type")
	if len(values) != 1 || values[0] != "text/html; charset=utf-8" {
		t.Fatalf("unexpected stored values: %#v", values)
	}
}

func TestResponseHeadersContentTypeAppendPreservesValues(t *testing.T) {
	res := NewResponse()
	res.Headers.Set("content-type", "text/html")
	res.Headers.Append("Content-Type", "application/json")
	res.Headers.Append("CONTENT-TYPE", "text/plain")

	got := res.Headers.GetAll("content-type")
	if len(got) != 3 || got[0] != "text/html" || got[1] != "application/json" || got[2] != "text/plain" {
		t.Fatalf("unexpected Content-Type values: %#v", got)
	}
	if got := res.Headers.Get("Content-Type"); got != "text/html" {
		t.Fatalf("unexpected first Content-Type value: %q", got)
	}
}

func TestResponseHeadersAppendPromotesSingleValue(t *testing.T) {
	res := NewResponse()
	res.Headers.Set("Vary", "Accept-Encoding")
	res.Headers.Append("Vary", "Origin")

	got := res.Headers.GetAll("Vary")
	if len(got) != 2 || got[0] != "Accept-Encoding" || got[1] != "Origin" {
		t.Fatalf("unexpected promoted values: %#v", got)
	}
	if res.Headers.Has("Vary") != true || res.Headers.Count() != 1 {
		t.Fatalf("unexpected header state: has=%v count=%d", res.Headers.Has("Vary"), res.Headers.Count())
	}
}

func TestResponseCookiesSetUsesSingleValueStorage(t *testing.T) {
	res := NewResponse()
	res.Cookies.Set(Cookie{Name: "sid", Value: "abc", Path: "/"})

	if got := res.Cookies.Get("sid"); got.Value != "abc" || got.Path != "/" {
		t.Fatalf("unexpected cookie: %#v", got)
	}
	if res.Cookies.single == nil || res.Cookies.single["sid"].Value != "abc" {
		t.Fatalf("Set should use single-value storage: %#v", res.Cookies)
	}
	if got := res.Cookies.Count(); got != 1 {
		t.Fatalf("cookie count = %d, want 1", got)
	}
}

func TestResponseCookiesValuesPreservePublicShape(t *testing.T) {
	res := NewResponse()
	res.Cookies.Set(Cookie{Name: "sid", Value: "abc"})

	values := res.Cookies.Values()
	if len(values["sid"]) != 1 || values["sid"][0].Value != "abc" {
		t.Fatalf("unexpected public values: %#v", values)
	}

	pending := res.Cookies.PendingValues()
	if len(pending["sid"]) != 1 || pending["sid"][0].Value != "abc" {
		t.Fatalf("unexpected pending values: %#v", pending)
	}
}

func TestResponseImportMapSetGetHasDeletePrint(t *testing.T) {
	res := NewResponse()
	res.ImportMap.Set("@app/foo", "/foo.js")
	res.ImportMap.Set("@app/bar", "/bar.js")

	if !res.ImportMap.Has("@app/foo") {
		t.Fatal("expected import map to have @app/foo")
	}
	if got := res.ImportMap.Get("@app/foo"); got != "/foo.js" {
		t.Fatalf("unexpected import map value: %q", got)
	}
	if got := string(res.ImportMap.Print()); got != `"@app/bar":"/bar.js","@app/foo":"/foo.js"` {
		t.Fatalf("unexpected import map print: %s", got)
	}

	res.ImportMap.Delete("@app/foo")
	if res.ImportMap.Has("@app/foo") {
		t.Fatal("expected @app/foo to be deleted")
	}
	if got := res.ImportMap.Len(); got != 1 {
		t.Fatalf("unexpected import map len: %d", got)
	}
}

func TestResponseHeadersRangeWithoutChangingPublicValues(t *testing.T) {
	res := NewResponse()
	res.Headers.Set("Content-Type", "text/html")
	res.Headers.Append("Vary", "Accept-Encoding")
	res.Headers.Append("Vary", "Origin")

	got := make(map[string][]string)
	res.Headers.Range(func(key string, value string, first bool) bool {
		if first {
			got[key] = got[key][:0]
		}
		got[key] = append(got[key], value)
		return true
	})

	if values := got["Content-Type"]; len(values) != 1 || values[0] != "text/html" {
		t.Fatalf("unexpected Content-Type values: %#v", values)
	}
	if values := got["Vary"]; len(values) != 2 || values[0] != "Accept-Encoding" || values[1] != "Origin" {
		t.Fatalf("unexpected Vary values: %#v", values)
	}
}

func TestResponseHeadersNormalizeKeysAcrossOperations(t *testing.T) {
	res := NewResponse()
	res.Headers.Set("x-request-id", "first")

	if got := res.Headers.Get("X-Request-ID"); got != "first" {
		t.Fatalf("Get should be case-insensitive: %q", got)
	}
	if !res.Headers.Has("X-REQUEST-ID") {
		t.Fatal("Has should be case-insensitive")
	}
	keys := res.Headers.Keys()
	if len(keys) != 1 || keys[0] != "X-Request-Id" {
		t.Fatalf("header key was not canonicalized: %#v", keys)
	}

	res.Headers.Append("x-REQUEST-id", "second")
	values := res.Headers.GetAll("X-Request-Id")
	if len(values) != 2 || values[0] != "first" || values[1] != "second" {
		t.Fatalf("mixed-case values = %#v", values)
	}

	res.Headers.Delete("X-REQUEST-id")
	if res.Headers.Has("x-request-id") {
		t.Fatal("Delete should be case-insensitive")
	}
}

func TestResponseHeadersNormalizeContentTypeKey(t *testing.T) {
	res := NewResponse()
	res.Headers.Set("content-type", "text/plain")
	res.Headers.Append("CONTENT-TYPE", "application/json")

	got := res.Headers.GetAll("Content-Type")
	if len(got) != 2 || got[0] != "text/plain" || got[1] != "application/json" {
		t.Fatalf("mixed-case Content-Type values = %#v", got)
	}
	keys := res.Headers.Keys()
	if len(keys) != 1 || keys[0] != "Content-Type" {
		t.Fatalf("canonical Content-Type keys = %#v", keys)
	}

	res.Headers.Delete("Content-TYPE")
	if res.Headers.Has("content-type") {
		t.Fatal("Content-Type should be deleted case-insensitively")
	}
}

func TestResponseHeadersValuesAndRangeExposeCanonicalKeys(t *testing.T) {
	res := NewResponse()
	res.Headers.Set("X-Custom-Header", "value")
	res.Headers.Set("Content-Type", "text/plain")

	values := res.Headers.Values()
	if values["X-Custom-Header"][0] != "value" || values["Content-Type"][0] != "text/plain" {
		t.Fatalf("Values did not expose canonical keys: %#v", values)
	}

	seen := make(map[string]bool)
	res.Headers.Range(func(key string, value string, first bool) bool {
		seen[key] = true
		return true
	})
	if !seen["X-Custom-Header"] || !seen["Content-Type"] {
		t.Fatalf("Range did not expose canonical keys: %#v", seen)
	}
}
