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

	if values := got["Content-Type"]; len(values) != 1 || values[0] != "text/html" {
		t.Fatalf("unexpected Content-Type values: %#v", values)
	}
	if values := got["Vary"]; len(values) != 2 || values[0] != "Accept-Encoding" || values[1] != "Origin" {
		t.Fatalf("unexpected Vary values: %#v", values)
	}
}
