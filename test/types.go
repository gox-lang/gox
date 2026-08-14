package test

import "github.com/gox-lang/gox/server"

// Method constants for NewRequest.Method.
const (
	GET     = "GET"
	POST    = "POST"
	PUT     = "PUT"
	PATCH   = "PATCH"
	DELETE  = "DELETE"
	HEAD    = "HEAD"
	OPTIONS = "OPTIONS"
)

// Map is a concise JSON object helper used by JSON body/action matchers.
type Map map[string]any

// Config configures one Execute or Benchmark run.
//
// Session is server-side initial session state. It is not a matcher.
type Config struct {
	Session Session `json:"session,omitempty"`

	// OverrideConfig can mutate the app/server config for this execution only.
	// It is used by the gox test runner before creating the in-memory runtime.
	OverrideConfig func(*server.Config) `json:"-"`

	// Benchmark configuration. Test mode ignores these fields.
	Iterations      int     `json:"iterations,omitempty"`
	Warmup          int     `json:"warmup,omitempty"`
	BenchSampleRate float64 `json:"benchSampleRate,omitempty"`
}

// Session is the initial server-side session state for a test execution.
type Session map[string]any

type RequestURLParams map[string]string
type RequestHeaders map[string]string
type RequestCookies map[string]string

// ActionParams contains Action arguments in declaration order, matching the
// positional form fields produced by the GOX JavaScript client.
type ActionParams []any

// FormData contains multipart form fields. Supported values are string,
// []string, gox.File and []gox.File.
type FormData map[string]any

// PipeRequest is implemented by request types that can be executed by the GoX test runner.
type PipeRequest interface {
	isPipeRequest()
}

// PipeExpected is implemented by expected-response contracts.
type PipeExpected interface {
	isPipeExpected()
}

// NewRequest describes a normal HTTP route request.
type NewRequest struct {
	Method string `json:"method"`
	URL    string `json:"url"`

	URLParams RequestURLParams `json:"urlParams,omitempty"`
	Headers   RequestHeaders   `json:"headers,omitempty"`
	Cookies   RequestCookies   `json:"cookies,omitempty"`
	Body      RequestBodyValue `json:"-"`
}

// NewActionRequest describes a client-side Action invocation.
//
// Method is the Action method name. It is not an HTTP method.
type NewActionRequest struct {
	URL    string `json:"url"`
	Method string `json:"method"`

	Params ActionParams `json:"params,omitempty"`
}

func (NewRequest) isPipeRequest()       {}
func (NewActionRequest) isPipeRequest() {}

// ExecutePipeLine describes one request/action step and its optional expected contract.
type ExecutePipeLine struct {
	Request  PipeRequest  `json:"-"`
	Expected PipeExpected `json:"-"`
}

// NewResponseExpected describes the expected contract for a normal HTTP route response.
type NewResponseExpected struct {
	Status  int                 `json:"status,omitempty"`
	Headers ExpectedHeadersRule `json:"-"`
	Cookies ExpectedCookiesRule `json:"-"`
	Body    ExpectedBodyRule    `json:"-"`
}

// NewResponseActionExpected describes the expected contract for an Action response.
//
// GoX Action responses are normal HTTP responses tagged with:
//
//	Action-Response: success|error
type NewResponseActionExpected struct {
	Error  ExpectedActionErrorRule  `json:"-"`
	Result ExpectedActionResultRule `json:"-"`
}

func (NewResponseExpected) isPipeExpected()       {}
func (NewResponseActionExpected) isPipeExpected() {}
