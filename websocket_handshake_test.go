package gox

import "testing"

func TestSocketHandshakeSnapshotsUpgradeRequest(t *testing.T) {
	headers := map[string][]string{
		"Origin":     {"https://app.example.com"},
		"User-Agent": {"GOX Browser"},
		"X-Trace-ID": {"trace-1"},
	}
	cookies := map[string][]string{"theme": {"dark"}}
	query := map[string][]string{"room": {"general"}}
	request := NewRequest(RequestInput{
		Method:  "GET",
		IP:      "203.0.113.10",
		Secure:  true,
		URL:     NewURL(URLInput{Scheme: "https", Host: "app.example.com", Path: "/_gox/websocket/handshake", QueryString: "?room=general"}),
		Headers: NewCollection[string](headers),
		Cookies: NewCollection[string](cookies),
		Query:   NewCollection[string](query),
	})

	handshake := NewSocketHandshake(request)
	if handshake.Method() != "GET" || handshake.IP() != "203.0.113.10" || !handshake.IsSecure() {
		t.Fatalf("unexpected handshake identity: %#v", handshake)
	}
	if handshake.Origin() != "https://app.example.com" || handshake.UserAgent() != "GOX Browser" {
		t.Fatal("origin or user agent was not preserved")
	}
	room, ok := handshake.Query.Get("room")
	if !ok || room != "general" {
		t.Fatal("URL or query was not preserved")
	}
	theme, ok := handshake.Cookies.Get("theme")
	if !ok || theme != "dark" {
		t.Fatal("cookies were not preserved")
	}
	traceID, ok := handshake.Headers.Get("X-Trace-ID")
	if !ok || traceID != "trace-1" {
		t.Fatal("headers were not preserved")
	}

	request.Headers = NewCollection[string](map[string][]string{"X-Trace-ID": {"changed"}})
	request.Cookies = NewCollection[string](map[string][]string{"theme": {"light"}})
	request.Query = NewCollection[string](map[string][]string{"room": {"private"}})

	traceID, traceOK := handshake.Headers.Get("X-Trace-ID")
	theme, themeOK := handshake.Cookies.Get("theme")
	room, roomOK := handshake.Query.Get("room")

	if !traceOK || traceID != "trace-1" ||
		!themeOK || theme != "dark" ||
		!roomOK || room != "general" {
		t.Fatal("handshake snapshot changed with the request")
	}
}

func TestZeroSocketHandshakeHasSafeEmptyCollections(t *testing.T) {
	handshake := NewSocketHandshake(Request{})
	if handshake.Headers == nil || handshake.Cookies == nil || handshake.Query == nil {
		t.Fatal("handshake collections must always be available")
	}
	if !handshake.Headers.IsEmpty() || !handshake.Cookies.IsEmpty() || !handshake.Query.IsEmpty() {
		t.Fatal("zero request produced non-empty handshake collections")
	}
}
