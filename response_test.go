package gox

import "testing"

func TestResponseHeadersSetUsesSingleValueStorage(t *testing.T) {
	res := NewResponse()
	res.Headers.Set("Content-Type", "text/html; charset=utf-8")

	if got := res.Headers.Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("unexpected header value: %q", got)
	}
	if res.Headers.contentType != "text/html; charset=utf-8" {
		t.Fatalf("Content-Type should use inline storage: %#v", res.Headers)
	}
	if _, ok := res.Headers.multi["Content-Type"]; ok {
		t.Fatalf("Set should not use multi-value storage: %#v", res.Headers.multi)
	}
}

func TestResponseHeadersContentTypeAppendPreservesValues(t *testing.T) {
	res := NewResponse()
	res.Headers.Set("content-type", "text/html")
	res.Headers.Append("Content-Type", "application/json")

	got := res.Headers.GetAll("content-type")
	if len(got) != 2 || got[0] != "text/html" || got[1] != "application/json" {
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

	if values := got["content-type"]; len(values) != 1 || values[0] != "text/html" {
		t.Fatalf("unexpected Content-Type values: %#v", values)
	}
	if values := got["vary"]; len(values) != 2 || values[0] != "Accept-Encoding" || values[1] != "Origin" {
		t.Fatalf("unexpected Vary values: %#v", values)
	}
}

func TestResponseHeadersNormalizeKeysAcrossOperations(t *testing.T) {
	res := NewResponse()
	res.Headers.Set("X-Request-ID", "first")

	if got := res.Headers.Get("x-request-id"); got != "first" {
		t.Fatalf("Get with lowercase key = %q, want first", got)
	}
	if !res.Headers.Has("X-REQUEST-ID") {
		t.Fatal("Has should be case-insensitive")
	}
	if _, ok := res.Headers.single["x-request-id"]; !ok {
		t.Fatalf("header key was not normalized internally: %#v", res.Headers.single)
	}
	if _, ok := res.Headers.single["X-Request-ID"]; ok {
		t.Fatalf("original header casing should not remain internally: %#v", res.Headers.single)
	}

	res.Headers.Append("x-REQUEST-id", "second")
	got := res.Headers.GetAll("X-Request-Id")
	if len(got) != 2 || got[0] != "first" || got[1] != "second" {
		t.Fatalf("GetAll after mixed-case Append = %#v", got)
	}
	if res.Headers.Count() != 1 {
		t.Fatalf("Count after mixed-case operations = %d, want 1", res.Headers.Count())
	}

	res.Headers.Delete("X-REQUEST-ID")
	if res.Headers.Has("x-request-id") {
		t.Fatal("Delete should be case-insensitive")
	}
}

func TestResponseHeadersNormalizeContentTypeKey(t *testing.T) {
	res := NewResponse()
	res.Headers.Set("CONTENT-TYPE", "text/plain")
	res.Headers.Append("content-Type", "application/json")

	got := res.Headers.GetAll("Content-Type")
	if len(got) != 2 || got[0] != "text/plain" || got[1] != "application/json" {
		t.Fatalf("mixed-case Content-Type values = %#v", got)
	}
	keys := res.Headers.Keys()
	if len(keys) != 1 || keys[0] != "content-type" {
		t.Fatalf("normalized Content-Type keys = %#v", keys)
	}

	res.Headers.Delete("Content-TYPE")
	if res.Headers.Has("content-type") {
		t.Fatal("Content-Type should be deleted regardless of casing")
	}
}

func TestResponseHeadersValuesAndRangeExposeNormalizedKeys(t *testing.T) {
	res := NewResponse()
	res.Headers.Set("X-Custom-Header", "value")
	res.Headers.Set("Content-Type", "text/plain")

	values := res.Headers.Values()
	if values["x-custom-header"][0] != "value" || values["content-type"][0] != "text/plain" {
		t.Fatalf("Values did not expose normalized keys: %#v", values)
	}

	seen := make(map[string]bool)
	res.Headers.Range(func(key string, value string, first bool) bool {
		seen[key] = true
		return true
	})
	if !seen["x-custom-header"] || !seen["content-type"] {
		t.Fatalf("Range did not expose normalized keys: %#v", seen)
	}
}

func TestResponseHeadersContainsSearchesEveryValue(t *testing.T) {
	res := NewResponse()
	res.Headers.Append("Vary", "Accept-Encoding")
	res.Headers.Append("Vary", "Origin")

	if !res.Headers.Contains("VARY", "origin") {
		t.Fatal("Contains should inspect every value and compare case-insensitively")
	}
	if res.Headers.Contains("Vary", "authorization") {
		t.Fatal("Contains reported a value that is not present")
	}
}

func TestResponseRedirectChecksMutabilityOnce(t *testing.T) {
	checks := 0
	SetRuntimeResponseMutationGuard(func(*Response) bool {
		checks++
		return true
	})
	defer SetRuntimeResponseMutationGuard(nil)

	res := NewResponse()
	defer func() {
		recovered := recover()
		flow, ok := recovered.(FlowControl)
		if !ok || flow.Kind != FlowRedirect {
			t.Fatalf("unexpected redirect panic: %#v", recovered)
		}
		if checks != 1 {
			t.Fatalf("mutation guard checks = %d, want 1", checks)
		}
		if res.Status.GetCode() != 307 {
			t.Fatalf("redirect status = %d, want 307", res.Status.GetCode())
		}
		if got := res.Headers.Get("Location"); got != "/next" {
			t.Fatalf("redirect location = %q, want /next", got)
		}
	}()

	res.Redirect.Temporary(" /next ")
}

func TestResponseCookiesSetReplacesCookieByName(t *testing.T) {
	res := NewResponse()
	res.Cookies.Set(Cookie{Name: "sid", Value: "old", Path: "/old"})
	res.Cookies.Set(Cookie{Name: "sid", Value: "new", Path: "/new"})

	if got := res.Cookies.Count(); got != 1 {
		t.Fatalf("cookie count = %d, want 1", got)
	}
	got := res.Cookies.Get("sid")
	if got.Value != "new" || got.Path != "/new" {
		t.Fatalf("unexpected replacement cookie: %#v", got)
	}
	pending := res.Cookies.PendingValues()["sid"]
	if len(pending) != 1 || pending[0].Value != "new" {
		t.Fatalf("unexpected pending replacement: %#v", pending)
	}
}

func TestResponseCookiesDeleteCreatesSingleTombstone(t *testing.T) {
	res := NewResponse()
	res.Cookies.Set(Cookie{Name: "sid", Value: "active", Path: "/private"})
	res.Cookies.Delete("sid")
	res.Cookies.Delete("sid")

	if res.Cookies.Has("sid") {
		t.Fatal("deleted cookie should not remain active")
	}
	if got := res.Cookies.Count(); got != 0 {
		t.Fatalf("active cookie count = %d, want 0", got)
	}
	pending := res.Cookies.PendingValues()["sid"]
	if len(pending) != 1 {
		t.Fatalf("pending deletion count = %d, want 1: %#v", len(pending), pending)
	}
	if pending[0].MaxAge != -1 || pending[0].Path != "/" || pending[0].Expires.Unix() != 0 {
		t.Fatalf("unexpected deletion tombstone: %#v", pending[0])
	}
}

func TestResponseCookiesSetAfterDeleteClearsTombstone(t *testing.T) {
	res := NewResponse()
	res.Cookies.Delete("sid")
	res.Cookies.Set(Cookie{Name: "sid", Value: "restored", Path: "/"})

	pending := res.Cookies.PendingValues()["sid"]
	if len(pending) != 1 || pending[0].Value != "restored" || pending[0].MaxAge == -1 {
		t.Fatalf("unexpected pending cookie after restore: %#v", pending)
	}
}

func TestResponseCookieNamesRemainCaseSensitive(t *testing.T) {
	res := NewResponse()
	res.Cookies.Set(Cookie{Name: "Session", Value: "upper"})
	res.Cookies.Set(Cookie{Name: "session", Value: "lower"})

	if got := res.Cookies.Count(); got != 2 {
		t.Fatalf("cookie count = %d, want 2", got)
	}
	if res.Cookies.Get("Session").Value != "upper" || res.Cookies.Get("session").Value != "lower" {
		t.Fatalf("cookie names should remain case-sensitive: %#v", res.Cookies.Values())
	}
}

func TestResponseCookiesSetPreservesExpiredFlag(t *testing.T) {
	res := NewResponse()
	res.Cookies.Set(Cookie{Name: "session", Value: "active", Expired: true})

	got := res.Cookies.Get("session")
	if !got.Expired {
		t.Fatal("Get should preserve Expired=true")
	}
	if got.Value != "active" {
		t.Fatalf("Set should not transform the cookie value: %q", got.Value)
	}

	pending := res.Cookies.PendingValues()["session"]
	if len(pending) != 1 || !pending[0].Expired {
		t.Fatalf("PendingValues should preserve Expired=true: %#v", pending)
	}
}

func TestCookieExpiredDefaultsToFalse(t *testing.T) {
	cookie := Cookie{Name: "session"}
	if cookie.Expired {
		t.Fatal("Expired zero value should be false")
	}
}
