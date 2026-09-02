package gox

import (
	"crypto/rand"
	"encoding/base64"
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

// gox:internal
// SetRuntimeResponseState attaches opaque runtime-owned state to a response.
// It is intended for the GoX server runtime.
func SetRuntimeResponseState(response *Response, state any) {
	if response != nil {
		response.runtimeState = state
	}
}

// gox:internal
// RuntimeResponseState returns opaque state attached by the GoX server runtime.
func RuntimeResponseState(response *Response) any {
	if response == nil {
		return nil
	}
	return response.runtimeState
}

// gox:internal
func NewResponse() *Response {
	response := &Response{}
	ResetRuntimeResponse(response)
	return response
}

// gox:internal
// ResetRuntimeResponse clears and reinitializes a response for reuse by the
// GoX server runtime. User-visible response state never survives the reset.
func ResetRuntimeResponse(response *Response) {
	if response == nil {
		return
	}
	*response = Response{}
	response.Status = ResponseStatus{response: response, code: 200}
	response.Headers = ResponseHeaders{response: response}
	response.Cookies = ResponseCookies{response: response}
	response.Redirect = RedirectResponse{response: response}
	response.ImportMap = ResponseImportMap{response: response}
}

type ResponseStatus struct {
	response *Response
	code     int
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
}

func normalizeHeaderKey(key string) string {
	return strings.ToLower(key)
}

type ResponseHeaders struct {
	response *Response
	headers  map[string][]string
}

func (h *ResponseHeaders) Get(key string) (string, bool) {
	if h == nil {
		return "", false
	}
	key = normalizeHeaderKey(key)
	if key == "" {
		return "", false
	}
	values, ok := h.headers[key]
	if !ok || len(values) == 0 {
		return "", false
	}
	return values[0], true
}

func (h *ResponseHeaders) GetAll(key string) ([]string, bool) {
	if h == nil {
		return nil, false
	}
	key = normalizeHeaderKey(key)
	if key == "" {
		return nil, false
	}
	values, ok := h.headers[key]
	if !ok || len(values) == 0 {
		return nil, false
	}
	return cloneSlice(values), true
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
	values, ok := h.GetAll(key)
	if !ok {
		return false
	}
	for _, current := range values {
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
		values, _ := h.GetAll(key)
		entries = append(entries, Entry[string]{Key: key, Value: values})
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
	Expired  bool
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
	cookies  map[string][]Cookie
}

func (c *ResponseCookies) Get(key string) (Cookie, bool) {
	if c == nil || key == "" {
		return Cookie{}, false
	}
	values, ok := c.cookies[key]
	if !ok || len(values) == 0 {
		return Cookie{}, false
	}
	return values[0], true
}

func (c *ResponseCookies) GetAll(key string) ([]Cookie, bool) {
	if c == nil || key == "" {
		return nil, false
	}
	values, ok := c.cookies[key]
	if !ok || len(values) == 0 {
		return nil, false
	}
	return cloneSlice(values), true
}

func (c *ResponseCookies) Set(cookie Cookie) {
	if c == nil || cookie.Name == "" {
		return
	}
	c.ensureMutable()
	if c.cookies == nil {
		c.cookies = make(map[string][]Cookie)
	}
	c.cookies[cookie.Name] = []Cookie{cookie}
}

func (c *ResponseCookies) Append(cookie Cookie) {
	if c == nil || cookie.Name == "" {
		return
	}
	c.ensureMutable()
	if c.cookies == nil {
		c.cookies = make(map[string][]Cookie)
	}
	c.cookies[cookie.Name] = append(c.cookies[cookie.Name], cookie)
}

func (c *ResponseCookies) Delete(key string) {
	if c == nil || key == "" {
		return
	}
	c.ensureMutable()
	delete(c.cookies, key)
}

func (c *ResponseCookies) Has(key string) bool {
	if c == nil || key == "" {
		return false
	}
	values, ok := c.cookies[key]
	return ok && len(values) > 0
}

func (c *ResponseCookies) IsEmpty() bool { return c.Count() == 0 }

func (c *ResponseCookies) Count() int {
	if c == nil {
		return 0
	}
	count := 0
	for _, values := range c.cookies {
		if len(values) > 0 {
			count++
		}
	}
	return count
}

func (c *ResponseCookies) Keys() []string {
	if c == nil || c.Count() == 0 {
		return nil
	}
	keys := make([]string, 0, c.Count())
	for key, values := range c.cookies {
		if key == "" || len(values) == 0 {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (c *ResponseCookies) Values() map[string][]Cookie {
	if c == nil || c.Count() == 0 {
		return nil
	}
	out := make(map[string][]Cookie, c.Count())
	for key, values := range c.cookies {
		if key == "" || len(values) == 0 {
			continue
		}
		out[key] = cloneSlice(values)
	}
	return out
}

// PendingValues returns the cookies that must be written to the HTTP response.
// It is intended for the runtime bridge.
func (c *ResponseCookies) PendingValues() map[string][]Cookie {
	return c.Values()
}

func (c *ResponseCookies) Entries() []Entry[Cookie] {
	keys := c.Keys()
	entries := make([]Entry[Cookie], 0, len(keys))
	for _, key := range keys {
		values, _ := c.GetAll(key)
		entries = append(entries, Entry[Cookie]{Key: key, Value: values})
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

	r.Headers.Set("Location", location)
	r.Status.setCodeInternal(status)
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

var runtimeFlowCallsiteResolver func() (file string, line int)

// gox:internal
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

// gox:internal
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

// gox:internal
// ResponseImportMapRuntime contains operations reserved for generated GOX
// code and the server runtime. Application code should use ResponseImportMap
// directly through Set, Get, Has, Delete and Len.
type ResponseImportMapRuntime struct {
	importMap *ResponseImportMap
}

// gox:internal
// Runtime returns the runtime-only view used by generated GOX code.
func (m *ResponseImportMap) Runtime() ResponseImportMapRuntime {
	return ResponseImportMapRuntime{importMap: m}
}

func (m *ResponseImportMap) nonceValue() string {
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

func (m *ResponseImportMap) Get(specifier string) (string, bool) {
	if m == nil || m.imports == nil {
		return "", false
	}
	specifier = strings.TrimSpace(specifier)
	if specifier == "" {
		return "", false
	}
	url, ok := m.imports[specifier]
	return url, ok
}

func (m *ResponseImportMap) Has(specifier string) bool {
	if m == nil || m.imports == nil {
		return false
	}
	specifier = strings.TrimSpace(specifier)
	if specifier == "" {
		return false
	}
	_, ok := m.imports[specifier]
	return ok
}

func (m *ResponseImportMap) Delete(specifier string) {
	if m == nil {
		return
	}
	specifier = strings.TrimSpace(specifier)
	if specifier == "" {
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

// gox:internal
// ApplyCSP applies the import-map CSP required by generated GOX code.
func (r ResponseImportMapRuntime) ApplyCSP(staticHash string) {
	m := r.importMap
	if m == nil || m.response == nil {
		return
	}
	if _, ok := m.response.Headers.Get("Content-Security-Policy"); ok {
		return
	}
	scriptSrc := "'self'"
	if m.modified {
		n := m.nonceValue()
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

// gox:internal
// WriteCompiled writes exactly one import map for generated GOX code.
func (r ResponseImportMapRuntime) WriteCompiled(w Writer, staticEntries [][2]string, staticScript []byte, staticHash string) {
	m := r.importMap
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
	nonce := m.nonceValue()
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

// gox:internal
// SecuritySource returns the CSP source emitted for the import map.
func (r ResponseImportMapRuntime) SecuritySource() string {
	m := r.importMap
	if m == nil || !m.emitted {
		return ""
	}
	return m.securitySource
}

func (m *ResponseImportMap) ensureMutable() {
	if m != nil && m.response != nil {
		m.response.ensureMutable()
	}
}
