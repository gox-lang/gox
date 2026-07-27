package gox

import "net/textproto"

// SocketHandshake is an immutable snapshot of the HTTP request that upgraded
// into a WebSocket connection. It intentionally excludes request-scoped
// objects such as Context, Body, Form, Files and Writer.
type SocketHandshake struct {
	method string
	ip     string
	secure bool

	URL     URL
	Headers Collection[string]
	Cookies Collection[string]
	Query   Collection[string]
}

// NewSocketHandshake snapshots the durable connection metadata from request.
// Every collection is cloned so the result remains valid after the HTTP
// context and its backing request are released or reused.
func NewSocketHandshake(request Request) SocketHandshake {
	return SocketHandshake{
		method:  request.Method(),
		ip:      request.IP(),
		secure:  request.IsSecure(),
		URL:     request.URL,
		Headers: snapshotHeaderCollection(request.Headers),
		Cookies: snapshotStringCollection(request.Cookies),
		Query:   snapshotStringCollection(request.Query),
	}
}

type socketHandshakeHeaders struct {
	Collection[string]
}

func (headers socketHandshakeHeaders) Get(key string) (string, bool) {
	return headers.Collection.Get(textproto.CanonicalMIMEHeaderKey(key))
}

func (headers socketHandshakeHeaders) GetAll(key string) ([]string, bool) {
	return headers.Collection.GetAll(textproto.CanonicalMIMEHeaderKey(key))
}

func (headers socketHandshakeHeaders) Has(key string) bool {
	return headers.Collection.Has(textproto.CanonicalMIMEHeaderKey(key))
}

func snapshotHeaderCollection(collection Collection[string]) Collection[string] {
	if collection == nil {
		return socketHandshakeHeaders{Collection: NewCollection[string](nil)}
	}
	values := collection.Values()
	normalized := make(map[string][]string, len(values))
	for key, entries := range values {
		canonical := textproto.CanonicalMIMEHeaderKey(key)
		normalized[canonical] = append(normalized[canonical], entries...)
	}
	return socketHandshakeHeaders{Collection: NewCollection[string](normalized)}
}

func snapshotStringCollection(collection Collection[string]) Collection[string] {
	if collection == nil {
		return NewCollection[string](nil)
	}
	return NewCollection[string](collection.Values())
}

func (handshake SocketHandshake) Method() string {
	return handshake.method
}

func (handshake SocketHandshake) IP() string {
	return handshake.ip
}

func (handshake SocketHandshake) IsSecure() bool {
	return handshake.secure
}

func (handshake SocketHandshake) Origin() string {
	value, _ := handshake.Headers.Get("Origin")
	return value
}

func (handshake SocketHandshake) UserAgent() string {
	value, _ := handshake.Headers.Get("User-Agent")
	return value
}
