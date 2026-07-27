package test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type ExecuteResult struct {
	File  string `json:"file,omitempty"`
	Name  string `json:"name,omitempty"`
	Mode  Mode   `json:"mode,omitempty"`
	Match bool   `json:"match"`

	Steps  []StepResult `json:"steps"`
	Output string       `json:"output,omitempty"`

	Benchmark *BenchmarkResult `json:"benchmark,omitempty"`
}

type StepResult struct {
	Index  int    `json:"index"`
	Type   string `json:"type"`
	Method string `json:"method"`
	URL    string `json:"url"`
	Match  bool   `json:"match"`

	Response ResponseSnapshot `json:"response,omitempty"`

	Status  MatchValue        `json:"status,omitempty"`
	Headers HeaderMatchResult `json:"headers,omitempty"`
	Cookies HeaderMatchResult `json:"cookies,omitempty"`
	Body    BodyMatchResult   `json:"body,omitempty"`
	Action  ActionMatchResult `json:"action,omitempty"`

	Error string `json:"error,omitempty"`

	Benchmark BenchmarkStepResult `json:"benchmark,omitempty"`
}

// ResponseSnapshot stores the full HTTP response captured for a step.
// It is intentionally complete so test.Save can write both JSON metadata and
// browser-like response artifacts without depending on matcher-specific data.
type ResponseSnapshot struct {
	StatusCode int                 `json:"statusCode,omitempty"`
	Status     string              `json:"status,omitempty"`
	Headers    map[string][]string `json:"headers,omitempty"`
	Cookies    map[string]string   `json:"cookies,omitempty"`
	Body       string              `json:"body,omitempty"`
}

// BenchmarkResult stores aggregate metrics for gox test -bench.
type BenchmarkResult struct {
	Warmup            int                       `json:"warmup"`
	Iterations        int                       `json:"iterations"`
	SampleRate        float64                   `json:"sampleRate"`
	AllocationSamples int                       `json:"allocationSamples"`
	DurationNS        int64                     `json:"durationNs"`
	NsPerOp           float64                   `json:"nsPerOp"`
	OpsPerSec         float64                   `json:"opsPerSec"`
	BytesSentPerOp    float64                   `json:"bytesSentPerOp"`
	BytesRecvPerOp    float64                   `json:"bytesReceivedPerOp"`
	HandlerAllocsOp   float64                   `json:"handler_allocs/op,omitempty"`
	HandlerBytesOp    float64                   `json:"handler_bytes/op,omitempty"`
	Status            map[int]int               `json:"status"`
	Errors            int                       `json:"errors"`
	AllocationSites   []BenchmarkAllocationSite `json:"allocationSites,omitempty"`
	Steps             []BenchmarkStepResult     `json:"steps,omitempty"`
}

// BenchmarkStepResult stores per-pipeline-step benchmark metrics.
type BenchmarkStepResult struct {
	Index             int                       `json:"index"`
	Type              string                    `json:"type"`
	Method            string                    `json:"method,omitempty"`
	URL               string                    `json:"url,omitempty"`
	Iterations        int                       `json:"iterations"`
	AllocationSamples int                       `json:"allocationSamples"`
	DurationNS        int64                     `json:"durationNs"`
	NsPerOp           float64                   `json:"nsPerOp"`
	OpsPerSec         float64                   `json:"opsPerSec"`
	BytesSentPerOp    float64                   `json:"bytesSentPerOp"`
	BytesRecvPerOp    float64                   `json:"bytesReceivedPerOp"`
	HandlerAllocsOp   float64                   `json:"handler_allocs/op,omitempty"`
	HandlerBytesOp    float64                   `json:"handler_bytes/op,omitempty"`
	AllocationSites   []BenchmarkAllocationSite `json:"allocationSites,omitempty"`
	Status            map[int]int               `json:"status"`
	Errors            int                       `json:"errors"`
	Error             string                    `json:"error,omitempty"`
}

// BenchmarkAllocationSite describes one handler allocation origin reported by gox test -bench.
type BenchmarkAllocationSite struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Name   string `json:"name,omitempty"`
	Allocs uint64 `json:"allocs"`
	Bytes  uint64 `json:"bytes"`
}

type MatchValue struct {
	Expected any    `json:"expected"`
	Response any    `json:"response"`
	Match    bool   `json:"match"`
	Rule     string `json:"rule,omitempty"`
	Message  string `json:"message,omitempty"`
}

type HeaderMatchResult struct {
	Rule   string                `json:"rule,omitempty"`
	Match  bool                  `json:"match"`
	Fields map[string]MatchValue `json:"fields,omitempty"`
}

type BodyMatchResult struct {
	Rule     string            `json:"rule,omitempty"`
	Expected any               `json:"expected,omitempty"`
	Response any               `json:"response,omitempty"`
	Match    bool              `json:"match"`
	Message  string            `json:"message,omitempty"`
	Rules    []BodyMatchResult `json:"rules,omitempty"`

	Findings     []BodyMatchFinding `json:"findings,omitempty"`
	JSONFindings []JSONMatchFinding `json:"json_findings,omitempty"`
	Diff         *BodyMatchDiff     `json:"diff,omitempty"`
}

type BodyMatchFinding struct {
	Pattern string `json:"pattern,omitempty"`
	Found   bool   `json:"found"`
	Line    int    `json:"line,omitempty"`
	Column  int    `json:"column,omitempty"`
	Offset  int    `json:"offset,omitempty"`
	Snippet string `json:"snippet,omitempty"`
}

type BodyMatchDiff struct {
	Line            int    `json:"line,omitempty"`
	Column          int    `json:"column,omitempty"`
	Offset          int    `json:"offset,omitempty"`
	ExpectedSnippet string `json:"expected_snippet,omitempty"`
	ResponseSnippet string `json:"response_snippet,omitempty"`
}

type JSONMatchFinding struct {
	Path     string `json:"path,omitempty"`
	Expected any    `json:"expected,omitempty"`
	Response any    `json:"response,omitempty"`
	Found    bool   `json:"found"`
	Match    bool   `json:"match"`
	Message  string `json:"message,omitempty"`
}

type ActionValueMatchResult struct {
	Rule         string                `json:"rule,omitempty"`
	Expected     any                   `json:"expected,omitempty"`
	Response     any                   `json:"response,omitempty"`
	Match        bool                  `json:"match"`
	Fields       map[string]MatchValue `json:"fields,omitempty"`
	JSONFindings []JSONMatchFinding    `json:"json_findings,omitempty"`
	Message      string                `json:"message,omitempty"`
}

type ActionMatchResult struct {
	Error  ActionValueMatchResult `json:"error,omitempty"`
	Result ActionValueMatchResult `json:"result,omitempty"`
	Match  bool                   `json:"match"`
}

func (s StepResult) MarshalJSON() ([]byte, error) {
	type stepResultJSON struct {
		Index     int                  `json:"index"`
		Type      string               `json:"type"`
		Method    string               `json:"method,omitempty"`
		URL       string               `json:"url,omitempty"`
		Match     bool                 `json:"match"`
		Response  *ResponseSnapshot    `json:"response,omitempty"`
		Status    *MatchValue          `json:"status,omitempty"`
		Headers   *HeaderMatchResult   `json:"headers,omitempty"`
		Cookies   *HeaderMatchResult   `json:"cookies,omitempty"`
		Body      *BodyMatchResult     `json:"body,omitempty"`
		Action    *ActionMatchResult   `json:"action,omitempty"`
		Error     string               `json:"error,omitempty"`
		Benchmark *BenchmarkStepResult `json:"benchmark,omitempty"`
	}

	out := stepResultJSON{
		Index:  s.Index,
		Type:   s.Type,
		Method: s.Method,
		URL:    s.URL,
		Match:  s.Match,
		Error:  s.Error,
	}
	if benchmarkStepHasData(s.Benchmark) {
		out.Benchmark = &s.Benchmark
	}

	if responseSnapshotHasData(s.Response) {
		out.Response = &s.Response
	}

	if matchValueHasData(s.Status) {
		out.Status = &s.Status
	}
	if headerMatchHasData(s.Headers) {
		out.Headers = &s.Headers
	}
	if headerMatchHasData(s.Cookies) {
		out.Cookies = &s.Cookies
	}
	if bodyMatchHasData(s.Body) {
		out.Body = &s.Body
	}
	if actionMatchHasData(s.Action) {
		out.Action = &s.Action
	}

	return json.Marshal(out)
}

func benchmarkResultHasData(value BenchmarkResult) bool {
	return value.Warmup != 0 || value.Iterations != 0 || value.AllocationSamples != 0 || value.DurationNS != 0 || value.NsPerOp != 0 || value.OpsPerSec != 0 || len(value.Status) > 0 || value.Errors != 0 || len(value.Steps) > 0
}

func benchmarkStepHasData(value BenchmarkStepResult) bool {
	return value.Iterations != 0 || value.AllocationSamples != 0 || value.DurationNS != 0 || value.NsPerOp != 0 || value.OpsPerSec != 0 || len(value.Status) > 0 || value.Errors != 0 || value.Error != ""
}

func responseSnapshotHasData(value ResponseSnapshot) bool {
	return value.StatusCode != 0 || value.Status != "" || len(value.Headers) > 0 || len(value.Cookies) > 0 || value.Body != ""
}

func matchValueHasData(value MatchValue) bool {
	return value.Rule != "" || value.Expected != nil || value.Response != nil || value.Message != ""
}

func headerMatchHasData(value HeaderMatchResult) bool {
	return value.Rule != "" || len(value.Fields) > 0
}

func bodyMatchHasData(value BodyMatchResult) bool {
	return value.Rule != "" || value.Expected != nil || value.Response != nil || value.Message != "" || len(value.Rules) > 0 || len(value.Findings) > 0 || len(value.JSONFindings) > 0 || value.Diff != nil
}

func actionValueMatchHasData(value ActionValueMatchResult) bool {
	return value.Rule != "" || value.Expected != nil || value.Response != nil || len(value.Fields) > 0 || len(value.JSONFindings) > 0 || value.Message != ""
}

func actionMatchHasData(value ActionMatchResult) bool {
	return actionValueMatchHasData(value.Error) || actionValueMatchHasData(value.Result)
}

func (r ExecuteResult) WriteOutput(path string) error {
	if path == "" {
		return nil
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, data, 0o644)
}

func (r ExecuteResult) Error() string {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Sprintf("gox test result: match=%v", r.Match)
	}
	return string(data)
}
