package gox

import "testing"

func TestResponseHeadersSetUsesSingleValueStorage(t *testing.T) {
	res := NewResponse()
	res.Headers.Set("Content-Type", "text/html; charset=utf-8")

	if got := responseHeaderValue(&res.Headers, "Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("unexpected header value: %q", got)
	}
	if got := responseHeaderValues(&res.Headers, "Content-Type"); len(got) != 1 || got[0] != "text/html; charset=utf-8" {
		t.Fatalf("unexpected stored values: %#v", got)
	}
}

func TestResponseHeadersContentTypeAppendPreservesValues(t *testing.T) {
	res := NewResponse()
	res.Headers.Set("content-type", "text/html")
	res.Headers.Append("Content-Type", "application/json")

	got := responseHeaderValues(&res.Headers, "content-type")
	if len(got) != 2 || got[0] != "text/html" || got[1] != "application/json" {
		t.Fatalf("unexpected Content-Type values: %#v", got)
	}
	if got := responseHeaderValue(&res.Headers, "Content-Type"); got != "text/html" {
		t.Fatalf("unexpected first Content-Type value: %q", got)
	}
}

func TestResponseHeadersAppendPromotesSingleValue(t *testing.T) {
	res := NewResponse()
	res.Headers.Set("Vary", "Accept-Encoding")
	res.Headers.Append("Vary", "Origin")

	got := responseHeaderValues(&res.Headers, "Vary")
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

	if got := responseCookieValue(&res.Cookies, "sid"); got.Value != "abc" || got.Path != "/" {
		t.Fatalf("unexpected cookie: %#v", got)
	}
	if res.Cookies.cookies == nil || len(res.Cookies.cookies["sid"]) != 1 || res.Cookies.cookies["sid"][0].Value != "abc" {
		t.Fatalf("Set should use one stored value: %#v", res.Cookies)
	}
	if got := res.Cookies.Count(); got != 1 {
		t.Fatalf("cookie count = %d, want 1", got)
	}
}

func TestResponseCookiesAppendPreservesCookiesWithSameName(t *testing.T) {
	res := NewResponse()
	res.Cookies.Append(Cookie{Name: "sid", Value: "root", Path: "/"})
	res.Cookies.Append(Cookie{Name: "sid", Value: "admin", Path: "/admin"})

	got := responseCookieValues(&res.Cookies, "sid")
	if len(got) != 2 {
		t.Fatalf("cookie count for sid = %d, want 2", len(got))
	}
	if got[0].Value != "root" || got[0].Path != "/" {
		t.Fatalf("unexpected first cookie: %#v", got[0])
	}
	if got[1].Value != "admin" || got[1].Path != "/admin" {
		t.Fatalf("unexpected second cookie: %#v", got[1])
	}
	if res.Cookies.Count() != 1 {
		t.Fatalf("Count = %d, want 1 distinct name", res.Cookies.Count())
	}
}

func TestResponseCookiesSetReplacesAllCookiesWithSameName(t *testing.T) {
	res := NewResponse()
	res.Cookies.Append(Cookie{Name: "sid", Value: "root", Path: "/"})
	res.Cookies.Append(Cookie{Name: "sid", Value: "admin", Path: "/admin"})
	res.Cookies.Set(Cookie{Name: "sid", Value: "replacement", Path: "/new"})

	got := responseCookieValues(&res.Cookies, "sid")
	if len(got) != 1 || got[0].Value != "replacement" || got[0].Path != "/new" {
		t.Fatalf("Set should replace every cookie with the same name: %#v", got)
	}
}

func TestResponseCookiesGetAllReturnsIndependentCopy(t *testing.T) {
	res := NewResponse()
	res.Cookies.Append(Cookie{Name: "sid", Value: "first"})
	res.Cookies.Append(Cookie{Name: "sid", Value: "second"})

	got := responseCookieValues(&res.Cookies, "sid")
	got[0].Value = "changed"

	if stored := responseCookieValue(&res.Cookies, "sid"); stored.Value != "first" {
		t.Fatalf("GetAll exposed internal storage: %#v", stored)
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

func TestResponseImportMapSetGetHasDeleteLen(t *testing.T) {
	res := NewResponse()
	res.ImportMap.Set("@app/foo", "/foo.js")
	res.ImportMap.Set("@app/bar", "/bar.js")

	if !res.ImportMap.Has("@app/foo") {
		t.Fatal("expected import map to have @app/foo")
	}
	if got, ok := res.ImportMap.Get("@app/foo"); !ok || got != "/foo.js" {
		t.Fatalf("unexpected import map value: %q, %t", got, ok)
	}

	res.ImportMap.Delete("@app/foo")
	if res.ImportMap.Has("@app/foo") {
		t.Fatal("expected @app/foo to be deleted")
	}
	if got := res.ImportMap.Len(); got != 1 {
		t.Fatalf("unexpected import map len: %d", got)
	}
}

func TestResponseImportMapSpecifierNormalization(t *testing.T) {
	res := NewResponse()
	res.ImportMap.Set("  @app/foo  ", "  /foo.js  ")

	if got, ok := res.ImportMap.Get("  @app/foo  "); !ok || got != "/foo.js" {
		t.Fatalf("unexpected normalized import map value: %q, %t", got, ok)
	}
	if !res.ImportMap.Has("  @app/foo  ") {
		t.Fatal("expected normalized specifier to exist")
	}

	res.ImportMap.Delete("  @app/foo  ")
	if res.ImportMap.Has("@app/foo") {
		t.Fatal("expected normalized specifier to be deleted")
	}
}

func TestResponseImportMapGetReportsMissingSpecifier(t *testing.T) {
	res := NewResponse()

	if got, ok := res.ImportMap.Get("missing"); ok || got != "" {
		t.Fatalf("unexpected missing import map value: %q, %t", got, ok)
	}
	if got, ok := res.ImportMap.Get("   "); ok || got != "" {
		t.Fatalf("unexpected blank import map value: %q, %t", got, ok)
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

	if got := responseHeaderValue(&res.Headers, "x-request-id"); got != "first" {
		t.Fatalf("Get with lowercase key = %q, want first", got)
	}
	if !res.Headers.Has("X-REQUEST-ID") {
		t.Fatal("Has should be case-insensitive")
	}
	if _, ok := res.Headers.headers["x-request-id"]; !ok {
		t.Fatalf("header key was not canonicalized internally: %#v", res.Headers.headers)
	}

	res.Headers.Append("x-REQUEST-id", "second")
	got := responseHeaderValues(&res.Headers, "X-Request-Id")
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

	got := responseHeaderValues(&res.Headers, "Content-Type")
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
		if got := responseHeaderValue(&res.Headers, "Location"); got != "/next" {
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
	got := responseCookieValue(&res.Cookies, "sid")
	if got.Value != "new" || got.Path != "/new" {
		t.Fatalf("unexpected replacement cookie: %#v", got)
	}
	pending := res.Cookies.PendingValues()["sid"]
	if len(pending) != 1 || pending[0].Value != "new" {
		t.Fatalf("unexpected pending replacement: %#v", pending)
	}
}

func TestResponseCookiesSetThenDeleteLeavesNoPendingCookie(t *testing.T) {
	res := NewResponse()
	res.Cookies.Set(Cookie{Name: "sid", Value: "active", Path: "/private"})
	res.Cookies.Delete("sid")

	if res.Cookies.Has("sid") {
		t.Fatal("deleted cookie should not remain pending")
	}
	if got := res.Cookies.Count(); got != 0 {
		t.Fatalf("cookie count = %d, want 0", got)
	}
	if pending := res.Cookies.PendingValues(); pending != nil {
		t.Fatalf("PendingValues = %#v, want nil", pending)
	}
}

func TestResponseCookiesDeleteMissingCookieIsNoOp(t *testing.T) {
	res := NewResponse()
	res.Cookies.Delete("sid")

	if pending := res.Cookies.PendingValues(); pending != nil {
		t.Fatalf("PendingValues = %#v, want nil", pending)
	}
}

func TestResponseCookiesDeleteThenSetLeavesOnePendingCookie(t *testing.T) {
	res := NewResponse()
	res.Cookies.Delete("sid")
	res.Cookies.Set(Cookie{Name: "sid", Value: "restored", Path: "/"})

	pending := res.Cookies.PendingValues()["sid"]
	if len(pending) != 1 || pending[0].Value != "restored" {
		t.Fatalf("unexpected pending cookie: %#v", pending)
	}
}

func TestResponseCookiesSetNormalThenExpiredReplacesPendingCookie(t *testing.T) {
	res := NewResponse()
	res.Cookies.Set(Cookie{Name: "sid", Value: "active"})
	res.Cookies.Set(Cookie{Name: "sid", Expired: true})

	pending := res.Cookies.PendingValues()["sid"]
	if len(pending) != 1 || !pending[0].Expired {
		t.Fatalf("unexpected pending expired cookie: %#v", pending)
	}
}

func TestResponseCookieNamesRemainCaseSensitive(t *testing.T) {
	res := NewResponse()
	res.Cookies.Set(Cookie{Name: "Session", Value: "upper"})
	res.Cookies.Set(Cookie{Name: "session", Value: "lower"})

	if got := res.Cookies.Count(); got != 2 {
		t.Fatalf("cookie count = %d, want 2", got)
	}
	if responseCookieValue(&res.Cookies, "Session").Value != "upper" || responseCookieValue(&res.Cookies, "session").Value != "lower" {
		t.Fatalf("cookie names should remain case-sensitive: %#v", res.Cookies.Values())
	}
}

func TestResponseCookiesSetPreservesExpiredFlag(t *testing.T) {
	res := NewResponse()
	res.Cookies.Set(Cookie{Name: "session", Value: "active", Expired: true})

	got := responseCookieValue(&res.Cookies, "session")
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

func TestResponseHeadersGetDistinguishesMissingFromEmptyValue(t *testing.T) {
	res := NewResponse()

	if value, ok := res.Headers.Get("X-Empty"); ok || value != "" {
		t.Fatalf("missing header = (%q, %v), want (\"\", false)", value, ok)
	}

	res.Headers.Set("X-Empty", "")
	if value, ok := res.Headers.Get("X-Empty"); !ok || value != "" {
		t.Fatalf("empty header = (%q, %v), want (\"\", true)", value, ok)
	}
}

func TestResponseHeadersGetAllDistinguishesMissingFromEmptyValue(t *testing.T) {
	res := NewResponse()

	if values, ok := res.Headers.GetAll("X-Empty"); ok || values != nil {
		t.Fatalf("missing header values = (%v, %v), want (nil, false)", values, ok)
	}

	res.Headers.Set("X-Empty", "")
	values, ok := res.Headers.GetAll("X-Empty")
	if !ok || len(values) != 1 || values[0] != "" {
		t.Fatalf("empty header values = (%v, %v), want ([\"\"], true)", values, ok)
	}
}

func TestResponseCookiesGetDistinguishesMissingFromEmptyValue(t *testing.T) {
	res := NewResponse()

	if cookie, ok := res.Cookies.Get("empty"); ok || cookie != (Cookie{}) {
		t.Fatalf("missing cookie = (%+v, %v), want (zero, false)", cookie, ok)
	}

	res.Cookies.Set(Cookie{Name: "empty", Value: ""})
	cookie, ok := res.Cookies.Get("empty")
	if !ok || cookie.Name != "empty" || cookie.Value != "" {
		t.Fatalf("empty cookie = (%+v, %v), want named empty cookie and true", cookie, ok)
	}
}

func TestResponseCookiesGetAllDistinguishesMissingFromEmptyValue(t *testing.T) {
	res := NewResponse()

	if cookies, ok := res.Cookies.GetAll("empty"); ok || cookies != nil {
		t.Fatalf("missing cookies = (%v, %v), want (nil, false)", cookies, ok)
	}

	res.Cookies.Set(Cookie{Name: "empty", Value: ""})
	cookies, ok := res.Cookies.GetAll("empty")
	if !ok || len(cookies) != 1 || cookies[0].Name != "empty" || cookies[0].Value != "" {
		t.Fatalf("empty cookies = (%v, %v), want one named empty cookie and true", cookies, ok)
	}
}
