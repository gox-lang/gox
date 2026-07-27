package gox

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
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
	response    *Response
	contentType string
	single      map[string]string
	multi       map[string][]string
}

func (h *ResponseHeaders) Get(key string) string {
	if h == nil {
		return ""
	}
	key = normalizeHeaderKey(key)
	if key == "" {
		return ""
	}
	if key == "content-type" && h.contentType != "" {
		return h.contentType
	}
	if h.single != nil {
		if value, ok := h.single[key]; ok {
			return value
		}
	}
	if h.multi == nil {
		return ""
	}
	values := h.multi[key]
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
	if key == "content-type" && h.contentType != "" {
		return []string{h.contentType}
	}
	if h.multi != nil {
		if values := h.multi[key]; len(values) > 0 {
			return cloneSlice(values)
		}
	}
	if h.single != nil {
		if value, ok := h.single[key]; ok {
			return []string{value}
		}
	}
	return nil
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
	if key == "content-type" {
		h.contentType = value
		if h.single != nil {
			delete(h.single, key)
		}
		if h.multi != nil {
			delete(h.multi, key)
		}
		return
	}
	if h.single == nil {
		h.single = make(map[string]string)
	}
	h.single[key] = value
	if h.multi != nil {
		delete(h.multi, key)
	}
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
	if key == "content-type" {
		if h.contentType == "" {
			h.contentType = value
			return
		}
		if h.multi == nil {
			h.multi = make(map[string][]string)
		}
		h.multi[key] = append(h.multi[key], h.contentType, value)
		h.contentType = ""
		return
	}
	if h.multi == nil {
		h.multi = make(map[string][]string)
	}
	if h.single != nil {
		if current, ok := h.single[key]; ok {
			h.multi[key] = []string{current, value}
			delete(h.single, key)
			return
		}
	}
	h.multi[key] = append(h.multi[key], value)
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
	if key == "content-type" {
		h.contentType = ""
	}
	if h.single != nil {
		delete(h.single, key)
	}
	if h.multi != nil {
		delete(h.multi, key)
	}
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
	if key == "content-type" && h.contentType != "" {
		return true
	}
	if h.single != nil {
		if _, ok := h.single[key]; ok {
			return true
		}
	}
	if h.multi == nil {
		return false
	}
	values, ok := h.multi[key]
	return ok && len(values) > 0
}

func (h *ResponseHeaders) IsEmpty() bool { return h.Count() == 0 }

func (h *ResponseHeaders) Count() int {
	if h == nil {
		return 0
	}
	count := len(h.single)
	if h.contentType != "" {
		count++
	}
	for key, values := range h.multi {
		if len(values) == 0 {
			continue
		}
		if h.single != nil {
			if _, exists := h.single[key]; exists {
				continue
			}
		}
		count++
	}
	return count
}

func (h *ResponseHeaders) Keys() []string {
	if h == nil || h.Count() == 0 {
		return nil
	}
	seen := make(map[string]struct{}, h.Count())
	keys := make([]string, 0, h.Count())
	if h.contentType != "" {
		seen["content-type"] = struct{}{}
		keys = append(keys, "content-type")
	}
	for key := range h.single {
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	for key, values := range h.multi {
		if len(values) == 0 {
			continue
		}
		if _, ok := seen[key]; ok {
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
	if h.contentType != "" && !fn("content-type", h.contentType, true) {
		return
	}
	for key, value := range h.single {
		if key == "" {
			continue
		}
		if !fn(key, value, true) {
			return
		}
	}
	for key, values := range h.multi {
		if key == "" || len(values) == 0 {
			continue
		}
		if h.single != nil {
			if _, exists := h.single[key]; exists {
				continue
			}
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
	if h.contentType != "" {
		out["content-type"] = []string{h.contentType}
	}
	for key, value := range h.single {
		out[key] = []string{value}
	}
	for key, values := range h.multi {
		if len(values) == 0 {
			continue
		}
		if _, exists := out[key]; exists {
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
