package gox

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/textproto"
	"sort"
	"strconv"
	"strings"
	"time"
)

type FlowControlKind string

const (
	FlowDoneWith FlowControlKind = "doneWith"
	FlowRedirect FlowControlKind = "redirect"
	FlowInvalid  FlowControlKind = "invalid"
)

type FlowControl struct {
	Kind       FlowControlKind
	StatusCode int
	Component  Component
	Location   string
	Message    string
	ErrorKind  string
	File       string
	Line       int
}

type Response struct {
	runtimeState any
	Status       ResponseStatus
	Headers      ResponseHeaders
	Cookies      ResponseCookies
	Redirect     RedirectResponse
	ImportMap    ResponseImportMap
}

// SetRuntimeResponseState attaches opaque runtime-owned state to a response.
// It is intended for the GoX server runtime.
func SetRuntimeResponseState(response *Response, state any) {
	if response != nil {
		response.runtimeState = state
	}
}

// RuntimeResponseState returns opaque state attached by the GoX server runtime.
func RuntimeResponseState(response *Response) any {
	if response == nil {
		return nil
	}
	return response.runtimeState
}

func NewResponse() *Response {
	response := &Response{}
	ResetRuntimeResponse(response)
	return response
}

// ResetRuntimeResponse clears and reinitializes a response for reuse by the
// GoX server runtime. User-visible response state never survives the reset.
func ResetRuntimeResponse(response *Response) {
	if response == nil {
		return
	}
	*response = Response{}
	response.Status = ResponseStatus{response: response, code: 200, message: "OK"}
	response.Headers = ResponseHeaders{response: response}
	response.Cookies = ResponseCookies{response: response}
	response.Redirect = RedirectResponse{response: response}
	response.ImportMap = ResponseImportMap{response: response}
}

type ResponseStatus struct {
	response      *Response
	code          int
	message       string
	messageCustom bool
}

func (s *ResponseStatus) GetCode() int {
	if s == nil || s.code == 0 {
		return 200
	}
	return s.code
}

func (s *ResponseStatus) SetCode(code int) {
	if s == nil {
		return
	}
	if s.response != nil {
		s.response.ensureMutable()
	}
	s.code = code
	s.message = defaultStatusMessage(code)
	s.messageCustom = false
}

func (s *ResponseStatus) GetMessage() string {
	if s == nil {
		return "OK"
	}
	if s.message != "" {
		return s.message
	}
	return defaultStatusMessage(s.GetCode())
}

func (s *ResponseStatus) SetMessage(message string) {
	if s == nil {
		return
	}
	if s.response != nil {
		s.response.ensureMutable()
	}
	s.message = message
	s.messageCustom = true
}

func normalizeHeaderKey(key string) string {
	return textproto.CanonicalMIMEHeaderKey(key)
}

type ResponseHeaders struct {
	response *Response
	headers  map[string][]string
}

func (h *ResponseHeaders) Get(key string) string {
	if h == nil {
		return ""
	}
	key = normalizeHeaderKey(key)
	if key == "" {
		return ""
	}
	values := h.headers[key]
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (h *ResponseHeaders) GetAll(key string) []string {
	if h == nil {
		return nil
	}
	key = normalizeHeaderKey(key)
	if key == "" {
		return nil
	}
	values := h.headers[key]
	if len(values) == 0 {
		return nil
	}
	return cloneSlice(values)
}

func (h *ResponseHeaders) Set(key string, value string) {
	if h == nil {
		return
	}
	key = normalizeHeaderKey(key)
	if key == "" {
		return
	}
	h.ensureMutable()
	if h.headers == nil {
		h.headers = make(map[string][]string)
	}
	h.headers[key] = []string{value}
}

func (h *ResponseHeaders) Append(key string, value string) {
	if h == nil {
		return
	}
	key = normalizeHeaderKey(key)
	if key == "" {
		return
	}
	h.ensureMutable()
	if h.headers == nil {
		h.headers = make(map[string][]string)
	}
	h.headers[key] = append(h.headers[key], value)
}

func (h *ResponseHeaders) Delete(key string) {
	if h == nil {
		return
	}
	key = normalizeHeaderKey(key)
	if key == "" {
		return
	}
	h.ensureMutable()
	delete(h.headers, key)
}

func (h *ResponseHeaders) Contains(key string, value string) bool {
	key = normalizeHeaderKey(key)
	if h == nil || key == "" || value == "" {
		return false
	}
	for _, current := range h.GetAll(key) {
		if containsFold(current, value) {
			return true
		}
	}
	return false
}

func containsFold(s string, substr string) bool {
	if substr == "" {
		return true
	}
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if strings.EqualFold(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

func (h *ResponseHeaders) Has(key string) bool {
	if h == nil {
		return false
	}
	key = normalizeHeaderKey(key)
	if key == "" {
		return false
	}
	values, ok := h.headers[key]
	return ok && len(values) > 0
}

func (h *ResponseHeaders) IsEmpty() bool { return h.Count() == 0 }

func (h *ResponseHeaders) Count() int {
	if h == nil {
		return 0
	}
	count := 0
	for _, values := range h.headers {
		if len(values) > 0 {
			count++
		}
	}
	return count
}

func (h *ResponseHeaders) Keys() []string {
	if h == nil || h.Count() == 0 {
		return nil
	}
	keys := make([]string, 0, h.Count())
	for key, values := range h.headers {
		if key == "" || len(values) == 0 {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// Range visits the stored header values without cloning maps or slices.
// first is true for the first value of each key.
func (h *ResponseHeaders) Range(fn func(key string, value string, first bool) bool) {
	if h == nil || fn == nil {
		return
	}
	for key, values := range h.headers {
		if key == "" || len(values) == 0 {
			continue
		}
		for i, value := range values {
			if !fn(key, value, i == 0) {
				return
			}
		}
	}
}

func (h *ResponseHeaders) Values() map[string][]string {
	if h == nil || h.Count() == 0 {
		return nil
	}
	out := make(map[string][]string, h.Count())
	for key, values := range h.headers {
		if key == "" || len(values) == 0 {
			continue
		}
		out[key] = cloneSlice(values)
	}
	return out
}

func (h *ResponseHeaders) Entries() []Entry[string] {
	keys := h.Keys()
	entries := make([]Entry[string], 0, len(keys))
	for _, key := range keys {
		entries = append(entries, Entry[string]{Key: key, Value: h.GetAll(key)})
	}
	return entries
}

func (h *ResponseHeaders) ensureMutable() {
	if h != nil && h.response != nil {
		h.response.ensureMutable()
	}
}

type Cookie struct {
	Name     string
	Value    string
	Path     string
	Domain   string
	MaxAge   int
	Expires  time.Time
	HttpOnly bool
	Secure   bool
	SameSite SameSite
}

type SameSite int

const (
	SameSiteDefault SameSite = iota
	SameSiteLax
	SameSiteStrict
	SameSiteNone
)

type ResponseCookies struct {
	response *Response
	single   map[string]Cookie
	deleted  map[string]Cookie
}

func (c *ResponseCookies) Get(key string) Cookie {
	if c == nil || key == "" || c.single == nil {
		return Cookie{}
	}
	return c.single[key]
}

func (c *ResponseCookies) GetAll(key string) []Cookie {
	if c == nil || key == "" || c.single == nil {
		return nil
	}
	cookie, ok := c.single[key]
	if !ok {
		return nil
	}
	return []Cookie{cookie}
}

func (c *ResponseCookies) Set(cookie Cookie) {
	if c == nil || cookie.Name == "" {
		return
	}
	c.ensureMutable()
	if c.single == nil {
		c.single = make(map[string]Cookie)
	}
	if c.deleted != nil {
		delete(c.deleted, cookie.Name)
	}
	c.single[cookie.Name] = cookie
}

func (c *ResponseCookies) Delete(key string) {
	if c == nil || key == "" {
		return
	}
	c.ensureMutable()
	if c.single != nil {
		delete(c.single, key)
	}
	if c.deleted == nil {
		c.deleted = make(map[string]Cookie)
	}
	c.deleted[key] = Cookie{Name: key, Path: "/", MaxAge: -1, Expires: time.Unix(0, 0)}
}

func (c *ResponseCookies) Has(key string) bool {
	if c == nil || key == "" || c.single == nil {
		return false
	}
	_, ok := c.single[key]
	return ok
}

func (c *ResponseCookies) IsEmpty() bool { return c.Count() == 0 }

func (c *ResponseCookies) Count() int {
	if c == nil {
		return 0
	}
	return len(c.single)
}

func (c *ResponseCookies) Keys() []string {
	if c == nil || len(c.single) == 0 {
		return nil
	}
	keys := make([]string, 0, len(c.single))
	for key := range c.single {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (c *ResponseCookies) Values() map[string][]Cookie {
	if c == nil || len(c.single) == 0 {
		return nil
	}
	out := make(map[string][]Cookie, len(c.single))
	for key, cookie := range c.single {
		out[key] = []Cookie{cookie}
	}
	return out
}

// PendingValues returns the cookies that must be written to the HTTP response,
// including deletion tombstones. It is intended for the runtime bridge.
func (c *ResponseCookies) PendingValues() map[string][]Cookie {
	if c == nil || (len(c.single) == 0 && len(c.deleted) == 0) {
		return nil
	}
	out := make(map[string][]Cookie, len(c.single)+len(c.deleted))
	for key, cookie := range c.single {
		out[key] = []Cookie{cookie}
	}
	for key, cookie := range c.deleted {
		if key == "" {
			continue
		}
		out[key] = []Cookie{cookie}
	}
	return out
}

func (c *ResponseCookies) Entries() []Entry[Cookie] {
	keys := c.Keys()
	entries := make([]Entry[Cookie], 0, len(keys))
	for _, key := range keys {
		if cookie, ok := c.single[key]; ok {
			entries = append(entries, Entry[Cookie]{Key: key, Value: []Cookie{cookie}})
		}
	}
	return entries
}

func (c *ResponseCookies) ensureMutable() {
	if c != nil && c.response != nil {
		c.response.ensureMutable()
	}
}

func (r *Response) DoneWith(status int, component ...Component) {
	if r == nil {
		return
	}
	r.ensureMutable()
	r.Status.setCodeInternal(status)

	var selected Component
	if len(component) > 0 {
		selected = component[0]
	}

	file, line := responseFlowCallsite()
	panic(FlowControl{
		Kind:       FlowDoneWith,
		StatusCode: status,
		Component:  selected,
		File:       file,
		Line:       line,
	})
}

func (r *Response) redirect(status int, location string) {
	if r == nil {
		return
	}
	r.ensureMutable()
	location = strings.TrimSpace(location)
	file, line := responseFlowCallsite()
	if location == "" {
		panic(FlowControl{
			Kind:       FlowInvalid,
			StatusCode: 500,
			Message:    "ctx.Response.Redirect requires a non-empty location",
			ErrorKind:  "invalid-redirect",
			File:       file,
			Line:       line,
		})
	}

	r.Status.setCodeInternal(status)
	r.Headers.Set("Location", location)
	panic(FlowControl{
		Kind:       FlowRedirect,
		StatusCode: status,
		Location:   location,
		File:       file,
		Line:       line,
	})
}

type RedirectResponse struct {
	response *Response
}

func (r RedirectResponse) Permanent(location string) {
	r.response.redirect(308, location)
}

func (r RedirectResponse) Temporary(location string) {
	r.response.redirect(307, location)
}

func (r RedirectResponse) SeeOther(location string) {
	r.response.redirect(303, location)
}

func (s *ResponseStatus) setCodeInternal(code int) {
	if s == nil {
		return
	}
	s.code = code
	s.message = defaultStatusMessage(code)
	s.messageCustom = false
}

func (r *Response) ensureMutable() {
	if r == nil || runtimeResponseMutationGuard == nil {
		return
	}
	if runtimeResponseMutationGuard(r) {
		return
	}
	file, line := responseFlowCallsite()
	panic(FlowControl{
		Kind:       FlowInvalid,
		StatusCode: 500,
		Message:    "ctx.Response cannot be modified after the response writer has started",
		ErrorKind:  "invalid-response-mutation-after-write",
		File:       file,
		Line:       line,
	})
}

func defaultStatusMessage(code int) string {
	switch code {
	case 100:
		return "Continue"
	case 101:
		return "Switching Protocols"
	case 102:
		return "Processing"
	case 103:
		return "Early Hints"
	case 200:
		return "OK"
	case 201:
		return "Created"
	case 202:
		return "Accepted"
	case 203:
		return "Non-Authoritative Information"
	case 204:
		return "No Content"
	case 205:
		return "Reset Content"
	case 206:
		return "Partial Content"
	case 207:
		return "Multi-Status"
	case 208:
		return "Already Reported"
	case 226:
		return "IM Used"
	case 300:
		return "Multiple Choices"
	case 301:
		return "Moved Permanently"
	case 302:
		return "Found"
	case 303:
		return "See Other"
	case 304:
		return "Not Modified"
	case 305:
		return "Use Proxy"
	case 307:
		return "Temporary Redirect"
	case 308:
		return "Permanent Redirect"
	case 400:
		return "Bad Request"
	case 401:
		return "Unauthorized"
	case 402:
		return "Payment Required"
	case 403:
		return "Forbidden"
	case 404:
		return "Not Found"
	case 405:
		return "Method Not Allowed"
	case 406:
		return "Not Acceptable"
	case 407:
		return "Proxy Authentication Required"
	case 408:
		return "Request Timeout"
	case 409:
		return "Conflict"
	case 410:
		return "Gone"
	case 411:
		return "Length Required"
	case 412:
		return "Precondition Failed"
	case 413:
		return "Content Too Large"
	case 414:
		return "URI Too Long"
	case 415:
		return "Unsupported Media Type"
	case 416:
		return "Range Not Satisfiable"
	case 417:
		return "Expectation Failed"
	case 418:
		return "I'm a teapot"
	case 421:
		return "Misdirected Request"
	case 422:
		return "Unprocessable Content"
	case 423:
		return "Locked"
	case 424:
		return "Failed Dependency"
	case 425:
		return "Too Early"
	case 426:
		return "Upgrade Required"
	case 428:
		return "Precondition Required"
	case 429:
		return "Too Many Requests"
	case 431:
		return "Request Header Fields Too Large"
	case 451:
		return "Unavailable For Legal Reasons"
	case 500:
		return "Internal Server Error"
	case 501:
		return "Not Implemented"
	case 502:
		return "Bad Gateway"
	case 503:
		return "Service Unavailable"
	case 504:
		return "Gateway Timeout"
	case 505:
		return "HTTP Version Not Supported"
	case 506:
		return "Variant Also Negotiates"
	case 507:
		return "Insufficient Storage"
	case 508:
		return "Loop Detected"
	case 510:
		return "Not Extended"
	case 511:
		return "Network Authentication Required"
	default:
		return ""
	}
}

var runtimeFlowCallsiteResolver func() (file string, line int)

// SetRuntimeFlowCallsiteResolver configures source-location capture for generated/runtime execution.
func SetRuntimeFlowCallsiteResolver(resolver func() (file string, line int)) {
	runtimeFlowCallsiteResolver = resolver
}

func responseFlowCallsite() (string, int) {
	if runtimeFlowCallsiteResolver == nil {
		return "", 0
	}
	return runtimeFlowCallsiteResolver()
}

var runtimeResponseMutationGuard func(*Response) bool

// SetRuntimeResponseMutationGuard configures runtime response mutability checks.
func SetRuntimeResponseMutationGuard(guard func(*Response) bool) {
	runtimeResponseMutationGuard = guard
}

type ResponseImportMap struct {
	response       *Response
	imports        map[string]string
	nonce          string
	modified       bool
	emitted        bool
	securitySource string
}

func (m *ResponseImportMap) Nonce() string {
	if m == nil {
		return ""
	}
	if m.nonce != "" {
		return m.nonce
	}
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return ""
	}
	m.nonce = base64.RawURLEncoding.EncodeToString(raw[:])
	return m.nonce
}

func (m *ResponseImportMap) Set(specifier string, url string) {
	if m == nil {
		return
	}
	specifier = strings.TrimSpace(specifier)
	url = strings.TrimSpace(url)
	if specifier == "" || url == "" {
		return
	}
	m.ensureMutable()
	m.modified = true
	if m.imports == nil {
		m.imports = make(map[string]string, 1)
	}
	m.imports[specifier] = url
}

func (m *ResponseImportMap) Get(specifier string) string {
	if m == nil || m.imports == nil {
		return ""
	}
	return m.imports[specifier]
}

func (m *ResponseImportMap) Has(specifier string) bool {
	if m == nil || m.imports == nil {
		return false
	}
	_, ok := m.imports[specifier]
	return ok
}

func (m *ResponseImportMap) Delete(specifier string) {
	if m == nil {
		return
	}
	m.ensureMutable()
	m.modified = true
	if m.imports != nil {
		delete(m.imports, specifier)
	}
}

func (m *ResponseImportMap) Len() int {
	if m == nil {
		return 0
	}
	return len(m.imports)
}

func (m *ResponseImportMap) Print() []byte {
	if m == nil || len(m.imports) == 0 {
		return nil
	}
	data, err := json.Marshal(m.imports)
	if err != nil || len(data) < 2 {
		return nil
	}
	return data[1 : len(data)-1]
}

func (m *ResponseImportMap) WriteEntries(w Writer, leadingComma bool) {
	if m == nil || len(m.imports) == 0 || w == nil {
		return
	}
	if leadingComma {
		w.WriteByte(',')
	}
	data := m.Print()
	w.WriteRawBytes(data)
}

func (m *ResponseImportMap) Modified() bool {
	return m != nil && m.modified
}

func (m *ResponseImportMap) SecuritySource() string {
	if m == nil || !m.emitted {
		return ""
	}
	return m.securitySource
}

type responseImportMapPayload struct {
	Static  [][2]string
	Dynamic map[string]string
}

func (p responseImportMapPayload) MarshalJSON() ([]byte, error) {
	buf := make([]byte, 0, 32+(len(p.Static)+len(p.Dynamic))*32)
	buf = append(buf, `{"imports":{`...)
	first := true
	for _, entry := range p.Static {
		specifier, url := entry[0], entry[1]
		if _, overridden := p.Dynamic[specifier]; overridden {
			continue
		}
		if !first {
			buf = append(buf, ',')
		}
		first = false
		buf = strconv.AppendQuote(buf, specifier)
		buf = append(buf, ':')
		buf = strconv.AppendQuote(buf, url)
	}
	for specifier, url := range p.Dynamic {
		if !first {
			buf = append(buf, ',')
		}
		first = false
		buf = strconv.AppendQuote(buf, specifier)
		buf = append(buf, ':')
		buf = strconv.AppendQuote(buf, url)
	}
	buf = append(buf, '}', '}')
	return buf, nil
}

func (m *ResponseImportMap) ApplyCSP(staticHash string) {
	if m == nil || m.response == nil {
		return
	}
	if m.response.Headers.Get("Content-Security-Policy") != "" {
		return
	}
	scriptSrc := "'self'"
	if m.modified {
		n := m.Nonce()
		if n != "" {
			m.securitySource = "'nonce-" + n + "'"
		}
	} else if staticHash != "" {
		m.securitySource = staticHash
	}
	if m.securitySource != "" {
		scriptSrc += " " + m.securitySource
	}
	m.response.Headers.Set("Content-Security-Policy", "default-src 'self'; script-src "+scriptSrc+"; style-src 'self'; img-src 'self' data:; font-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'self'")
}

// WriteCompiled writes exactly one import map. The static path is a single
// precompiled write. The dynamic path combines compiled and request entries,
// generates one nonce, and uses the existing Writer JSON serialization.
func (m *ResponseImportMap) WriteCompiled(w Writer, staticEntries [][2]string, staticScript []byte, staticHash string) {
	if m == nil || w == nil {
		return
	}
	if !m.modified {
		if len(staticScript) == 0 {
			return
		}
		w.WriteRawBytes(staticScript)
		m.emitted = true
		return
	}

	if len(staticEntries) == 0 && len(m.imports) == 0 {
		return
	}
	nonce := m.Nonce()
	if nonce == "" {
		return
	}
	w.WriteRawString(`<script type="importmap" nonce="`)
	w.WriteRawString(nonce)
	w.WriteRawString(`">`)
	w.WriteRawAny(responseImportMapPayload{Static: staticEntries, Dynamic: m.imports})
	w.WriteRawString(`</script>`)
	m.emitted = true
}

func (m *ResponseImportMap) ensureMutable() {
	if m != nil && m.response != nil {
		m.response.ensureMutable()
	}
}
