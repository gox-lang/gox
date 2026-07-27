package gox

import (
	"bytes"
	"sort"
)

type Request struct {
	method      string
	ip          string
	secure      bool
	contentType string

	URL     URL
	Headers Collection[string]
	Cookies Collection[string]
	Query   Collection[string]
	Form    FormData
	Route   Route
	Body    Body
}

type FormData struct {
	Fields Collection[string]
	Files  Collection[File]
}

type RequestInput struct {
	Method      string
	IP          string
	Secure      bool
	ContentType string

	URL     URL
	Headers Collection[string]
	Cookies Collection[string]
	Query   Collection[string]
	Form    Collection[string]
	Files   Collection[File]
	Route   Route
	Body    Body
}

func NewRequest(input RequestInput) Request {
	return Request{
		method:      input.Method,
		ip:          input.IP,
		secure:      input.Secure,
		contentType: input.ContentType,
		URL:         input.URL,
		Headers:     ensureStringCollection(input.Headers),
		Cookies:     ensureStringCollection(input.Cookies),
		Query:       ensureStringCollection(input.Query),
		Form: FormData{
			Fields: ensureStringCollection(input.Form),
			Files:  ensureFileCollection(input.Files),
		},
		Route: input.Route,
		Body:  input.Body,
	}
}

func (r Request) Method() string      { return r.method }
func (r Request) IP() string          { return r.ip }
func (r Request) IsSecure() bool      { return r.secure }
func (r Request) ContentType() string { return r.contentType }

type URL struct {
	explicit    string
	scheme      string
	host        string
	hostname    string
	port        string
	path        string
	rawPath     string
	queryString string
}

type URLInput struct {
	String      string
	Scheme      string
	Host        string
	Hostname    string
	Port        string
	Path        string
	RawPath     string
	QueryString string
}

func NewURL(input URLInput) URL {
	return URL{
		explicit:    input.String,
		scheme:      input.Scheme,
		host:        input.Host,
		hostname:    input.Hostname,
		port:        input.Port,
		path:        input.Path,
		rawPath:     input.RawPath,
		queryString: input.QueryString,
	}
}

func (u URL) String() string {
	if u.explicit != "" {
		return u.explicit
	}

	path := u.path
	if u.rawPath != "" {
		path = u.rawPath
	}

	prefixLen := len(u.scheme) + len(u.host)
	if u.scheme != "" {
		prefixLen += 3
	}
	if prefixLen == 0 {
		return path + u.queryString
	}

	var b bytes.Buffer
	b.Grow(prefixLen + len(path) + len(u.queryString))
	if u.scheme != "" {
		b.WriteString(u.scheme)
		b.WriteString("://")
	}
	b.WriteString(u.host)
	b.WriteString(path)
	b.WriteString(u.queryString)
	return b.String()
}
func (u URL) Scheme() string      { return u.scheme }
func (u URL) Host() string        { return u.host }
func (u URL) Hostname() string    { return u.hostname }
func (u URL) Port() string        { return u.port }
func (u URL) Path() string        { return u.path }
func (u URL) RawPath() string     { return u.rawPath }
func (u URL) QueryString() string { return u.queryString }

func (u URL) Origin() string {
	origin := u.scheme + "://" + u.hostname

	if u.port != "" {
		origin += ":" + u.port
	}

	return origin
}

type Route struct {
	matched     bool
	original    string
	target      string
	pattern     string
	method      string
	static      bool
	action      bool
	clientAsset bool

	Params Collection[string]
}

type RouteInput struct {
	Matched       bool
	Original      string
	Target        string
	Pattern       string
	Method        string
	IsStatic      bool
	IsAction      bool
	IsClientAsset bool
	Params        Collection[string]
}

func NewRoute(input RouteInput) Route {
	isStatic := input.IsStatic
	if input.IsClientAsset {
		isStatic = true
	}

	return Route{
		matched:     input.Matched,
		original:    input.Original,
		target:      input.Target,
		pattern:     input.Pattern,
		method:      input.Method,
		static:      isStatic,
		action:      input.IsAction,
		clientAsset: input.IsClientAsset,
		Params:      ensureStringCollection(input.Params),
	}
}

func (r Route) IsMatched() bool     { return r.matched }
func (r Route) Original() string    { return r.original }
func (r Route) Target() string      { return r.target }
func (r Route) Pattern() string     { return r.pattern }
func (r Route) Method() string      { return r.method }
func (r Route) IsStatic() bool      { return r.static }
func (r Route) IsAction() bool      { return r.action }
func (r Route) IsClientAsset() bool { return r.clientAsset }

type Entry[T any] struct {
	Key   string
	Value []T
}

type Collection[T any] interface {
	Get(key string) (T, bool)
	GetAll(key string) ([]T, bool)
	Has(key string) bool
	IsEmpty() bool
	Count() int
	Keys() []string
	Values() map[string][]T
	Entries() []Entry[T]
}

type collection[T any] struct {
	values map[string][]T
}

func NewCollection[T any](values map[string][]T) Collection[T] {
	return collection[T]{values: cloneCollectionMap(values)}
}

// NewCollectionView creates a read-only collection view over values without
// cloning the map or its slices. Callers must not mutate values while the
// collection is in use.
func NewCollectionView[T any](values map[string][]T) Collection[T] {
	return collection[T]{values: values}
}

func (c collection[T]) Get(key string) (T, bool) {
	values, ok := c.values[key]
	if !ok || len(values) == 0 {
		var zero T
		return zero, false
	}

	return values[0], true
}

func (c collection[T]) GetAll(key string) ([]T, bool) {
	values, ok := c.values[key]
	if !ok || len(values) == 0 {
		return nil, false
	}

	return cloneSlice(values), true
}

func (c collection[T]) Has(key string) bool {
	values, ok := c.values[key]
	return ok && len(values) > 0
}

func (c collection[T]) IsEmpty() bool {
	return c.Count() == 0
}

func (c collection[T]) Count() int {
	if len(c.values) == 0 {
		return 0
	}
	count := 0
	for _, values := range c.values {
		if len(values) > 0 {
			count++
		}
	}
	return count
}

func (c collection[T]) Keys() []string {
	keys := make([]string, 0, len(c.values))
	for key, values := range c.values {
		if len(values) > 0 {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func (c collection[T]) Values() map[string][]T {
	return cloneCollectionMap(c.values)
}

func (c collection[T]) Entries() []Entry[T] {
	keys := c.Keys()
	entries := make([]Entry[T], 0, len(keys))
	for _, key := range keys {
		entries = append(entries, Entry[T]{Key: key, Value: cloneSlice(c.values[key])})
	}
	return entries
}

func cloneCollectionMap[T any](values map[string][]T) map[string][]T {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string][]T, len(values))
	for key, value := range values {
		out[key] = cloneSlice(value)
	}
	return out
}

func cloneSlice[T any](value []T) []T {
	if len(value) == 0 {
		return nil
	}
	out := make([]T, len(value))
	copy(out, value)
	return out
}

func ensureStringCollection(value Collection[string]) Collection[string] {
	if value != nil {
		return value
	}
	return NewCollection[string](nil)
}

func ensureFileCollection(value Collection[File]) Collection[File] {
	if value != nil {
		return value
	}
	return NewCollection[File](nil)
}

// NewHeaders is kept for compatibility. Prefer NewCollection[string].
func NewHeaders(values map[string][]string) Collection[string] {
	return NewCollection[string](values)
}

// NewCookies is kept for compatibility. Prefer NewCollection[string].
func NewCookies(values map[string]string) Collection[string] {
	if len(values) == 0 {
		return NewCollection[string](nil)
	}
	out := make(map[string][]string, len(values))
	for key, value := range values {
		out[key] = []string{value}
	}
	return NewCollection[string](out)
}

// SetRuntimeRequestIP updates the resolved client IP without rebuilding Request.
// It is intended for the GoX server runtime.
func SetRuntimeRequestIP(request *Request, ip string) {
	if request != nil {
		request.ip = ip
	}
}

// SetRuntimeRequestRoute updates route metadata without rebuilding Request.
// It is intended for the GoX server runtime.
func SetRuntimeRequestRoute(request *Request, route Route) {
	if request != nil {
		request.Route = route
	}
}
