package test

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type Executor interface {
	Execute(config Config, pipeline []ExecutePipeLine) (ExecuteResult, bool)
}

type Mode string

const (
	ModeTest  Mode = "test"
	ModeBench Mode = "bench"
)

type RuntimeContext struct {
	File       string
	Name       string
	Mode       Mode
	ResultRoot string
}

type Messages struct {
	Pass    []string
	Fail    []string
	Explain []string
}

type Failure struct {
	Message string
}

func (f Failure) Error() string { return f.Message }

var executorState struct {
	mu    sync.RWMutex
	value Executor
}

var recorderState struct {
	mu    sync.RWMutex
	value func(ExecuteResult)
}

var contextState struct {
	mu       sync.RWMutex
	current  RuntimeContext
	messages Messages
}

// SetExecutor registers the executor used by Execute.
// The gox test runner sets this before invoking compiled .gox tests.
// Passing nil restores the default no-runner executor.
func SetExecutor(value Executor) {
	executorState.mu.Lock()
	executorState.value = value
	executorState.mu.Unlock()
}

// SetResultRecorder registers an optional observer called after every Execute.
// It is used by the gox test runner for verbose output and global reports.
// Passing nil disables recording.
func SetResultRecorder(value func(ExecuteResult)) {
	recorderState.mu.Lock()
	recorderState.value = value
	recorderState.mu.Unlock()
}

// SetRuntimeContext sets metadata used by Execute, Save, IsBench, Pass and Fail.
// It is called by the gox test runner before invoking a test function.
func SetRuntimeContext(value RuntimeContext) {
	contextState.mu.Lock()
	if value.Mode == "" {
		value.Mode = ModeTest
	}
	contextState.current = value
	contextState.messages = Messages{}
	contextState.mu.Unlock()
}

// ClearRuntimeContext clears the current gox test runtime context.
func ClearRuntimeContext() {
	contextState.mu.Lock()
	contextState.current = RuntimeContext{}
	contextState.messages = Messages{}
	contextState.mu.Unlock()
}

// GetMode returns the current test execution mode.
func GetMode() Mode {
	contextState.mu.RLock()
	mode := contextState.current.Mode
	contextState.mu.RUnlock()
	if mode == "" {
		return ModeTest
	}
	return mode
}

// IsBench reports whether the current test is being executed in benchmark mode.
func IsBench() bool {
	return GetMode() == ModeBench
}

// Messages returns the current Pass/Fail messages collected during a test.
func CurrentMessages() Messages {
	contextState.mu.RLock()
	msg := contextState.messages
	contextState.mu.RUnlock()
	return msg
}

func currentContext() RuntimeContext {
	contextState.mu.RLock()
	ctx := contextState.current
	contextState.mu.RUnlock()
	if ctx.Mode == "" {
		ctx.Mode = ModeTest
	}
	return ctx
}

func currentExecutor() Executor {
	executorState.mu.RLock()
	value := executorState.value
	executorState.mu.RUnlock()
	if value == nil {
		return noopExecutor{}
	}
	return value
}

func recordResult(result ExecuteResult) {
	recorderState.mu.RLock()
	recorder := recorderState.value
	recorderState.mu.RUnlock()
	if recorder != nil {
		recorder(result)
	}
}

func Execute(config Config, pipeline []ExecutePipeLine) (ExecuteResult, bool) {
	result, match := currentExecutor().Execute(config, pipeline)
	result = applyContext(result)
	recordResult(result)
	return result, match
}

func applyContext(result ExecuteResult) ExecuteResult {
	ctx := currentContext()
	if result.File == "" {
		result.File = ctx.File
	}
	if result.Name == "" {
		result.Name = ctx.Name
	}
	if result.Mode == "" {
		result.Mode = ctx.Mode
	}
	return result
}

type noopExecutor struct{}

func (noopExecutor) Execute(config Config, pipeline []ExecutePipeLine) (ExecuteResult, bool) {
	result := ExecuteResult{
		Match: false,
		Steps: []StepResult{{
			Index: 1,
			Type:  "error",
			Match: false,
			Error: "gox/test: Execute requires the gox test runner",
		}},
	}
	return result, false
}

// Save writes the complete result metadata to tests/results/<TestName>/<mode>.json.
// In test mode it also writes browser-like response artifacts under
// tests/results/<TestName>/responses/.
func Save(result ExecuteResult) {
	result = applyContext(result)
	path := resultSavePath(result)
	if path == "" {
		panic(Failure{Message: "gox/test: Save requires gox test runner context"})
	}
	result.Output = path
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		panic(Failure{Message: fmt.Sprintf("gox/test: save marshal failed: %v", err)})
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		panic(Failure{Message: fmt.Sprintf("gox/test: save mkdir failed: %v", err)})
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		panic(Failure{Message: fmt.Sprintf("gox/test: save write failed: %v", err)})
	}
	if result.Mode == ModeTest {
		if err := saveResponseArtifacts(filepath.Dir(path), result); err != nil {
			panic(Failure{Message: fmt.Sprintf("gox/test: save responses failed: %v", err)})
		}
	}
}

func saveResponseArtifacts(resultDir string, result ExecuteResult) error {
	responsesDir := filepath.Join(resultDir, "responses")
	wrote := false
	for _, step := range result.Steps {
		if !responseSnapshotHasData(step.Response) {
			continue
		}
		if !wrote {
			if err := os.MkdirAll(responsesDir, 0o755); err != nil {
				return err
			}
			wrote = true
		}
		name := responseArtifactName(step)
		if files, ok := responseArtifactFiles(step.Response); ok {
			dir := filepath.Join(responsesDir, strings.TrimSuffix(name, filepath.Ext(name)))
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			for _, file := range files {
				reader, err := file.Open()
				if err != nil {
					return err
				}
				data, err := io.ReadAll(reader)
				reader.Close()
				if err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(dir, file.Name()), data, 0o644); err != nil {
					return err
				}
			}
			continue
		}
		path := filepath.Join(responsesDir, name)
		if err := os.WriteFile(path, []byte(formatResponseArtifact(step.Response)), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func responseArtifactName(step StepResult) string {
	kind := strings.TrimSpace(step.Type)
	if kind == "" {
		kind = "step"
	}
	method := strings.TrimSpace(step.Method)
	if method == "" {
		method = kind
	}
	target := strings.TrimSpace(step.URL)
	if target == "" {
		target = kind
	}
	if kind == "action" && step.Method != "" {
		target = target + "_" + step.Method
	}
	return fmt.Sprintf("%03d_%s_%s.text", step.Index, sanitizePathSegment(method), sanitizePathSegment(target))
}

func formatResponseArtifact(response ResponseSnapshot) string {
	var b strings.Builder
	status := strings.TrimSpace(response.Status)
	if status == "" {
		if response.StatusCode != 0 {
			status = fmt.Sprintf("%d", response.StatusCode)
		} else {
			status = "200 OK"
		}
	}
	b.WriteString("HTTP/1.1 ")
	b.WriteString(status)
	b.WriteByte('\n')

	keys := make([]string, 0, len(response.Headers))
	for key := range response.Headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		for _, value := range response.Headers[key] {
			b.WriteString(key)
			b.WriteString(": ")
			b.WriteString(value)
			b.WriteByte('\n')
		}
	}
	b.WriteByte('\n')
	b.WriteString(response.Body)
	return b.String()
}

func resultSavePath(result ExecuteResult) string {
	ctx := currentContext()
	root := strings.TrimSpace(ctx.ResultRoot)
	if root == "" {
		return ""
	}
	name := strings.TrimSpace(result.Name)
	if name == "" {
		name = strings.TrimSpace(ctx.Name)
	}
	if name == "" {
		name = "result"
	}
	mode := result.Mode
	if mode == "" {
		mode = ctx.Mode
	}
	if mode == "" {
		mode = ModeTest
	}
	name = sanitizePathSegment(name)
	return filepath.Join(root, name, string(mode)+".json")
}

func sanitizePathSegment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "result"
	}
	value = strings.ReplaceAll(value, "\\", "_")
	value = strings.ReplaceAll(value, "/", "_")
	value = strings.ReplaceAll(value, string(filepath.Separator), "_")
	value = strings.ReplaceAll(value, "..", "_")
	return value
}

// Pass adds an optional success message to the current test output.
func Pass(message ...string) {
	text := strings.TrimSpace(strings.Join(message, "\n"))
	contextState.mu.Lock()
	if text != "" {
		contextState.messages.Pass = append(contextState.messages.Pass, text)
	}
	contextState.mu.Unlock()
}

// Fail adds an optional failure message to the current test output.
// It does not abort execution; the gox test runner reports the failure at the end of the handler.
func Fail(message ...string) {
	text := strings.TrimSpace(strings.Join(message, "\n"))
	contextState.mu.Lock()
	if text != "" {
		contextState.messages.Fail = append(contextState.messages.Fail, text)
	}
	contextState.mu.Unlock()
}

// Explain appends a compact, human-readable diagnostic report to the current test output.
func Explain(result ExecuteResult) {
	text := Format(result)
	if strings.TrimSpace(text) == "" {
		return
	}
	contextState.mu.Lock()
	contextState.messages.Explain = append(contextState.messages.Explain, text)
	contextState.mu.Unlock()
}

// Format returns a compact, human-readable diagnostic report for a result.
func Format(result ExecuteResult) string {
	result = applyContext(result)
	if result.Mode == ModeBench || result.Benchmark != nil {
		return FormatBenchmark(result)
	}
	var b strings.Builder
	title := "Gox Test Result"
	if result.Name != "" {
		title = result.Name
	}
	b.WriteString("╭─ ")
	if result.Match {
		b.WriteString("PASS ")
	} else {
		b.WriteString("FAIL ")
	}
	if result.File != "" {
		b.WriteString(result.File)
		if result.Name != "" {
			b.WriteString("::")
			b.WriteString(result.Name)
		}
	} else {
		b.WriteString(title)
	}
	b.WriteByte('\n')

	wrote := false
	for _, step := range result.Steps {
		if step.Match && step.Error == "" {
			continue
		}
		wrote = true
		b.WriteString("│\n")
		b.WriteString("│ Step ")
		b.WriteString(fmt.Sprint(step.Index))
		if step.Type == "action" {
			b.WriteString(" ACTION ")
			b.WriteString(step.URL)
			if step.Method != "" {
				b.WriteString("::")
				b.WriteString(step.Method)
			}
		} else {
			b.WriteByte(' ')
			if step.Method != "" {
				b.WriteString(step.Method)
				b.WriteByte(' ')
			}
			if step.URL != "" {
				b.WriteString(step.URL)
			} else {
				b.WriteString(step.Type)
			}
		}
		b.WriteByte('\n')
		if step.Error != "" {
			writeExplainField(&b, "Error", "", step.Error, "")
		}
		if matchValueHasData(step.Status) && !step.Status.Match {
			writeExplainField(&b, "Status mismatch", step.Status.Expected, step.Status.Response, step.Status.Message)
		}
		writeExplainHeaderFailures(&b, "Header", step.Headers)
		writeExplainHeaderFailures(&b, "Cookie", step.Cookies)
		if bodyMatchHasData(step.Body) && !step.Body.Match {
			writeExplainBody(&b, step.Body)
		}
		if actionMatchHasData(step.Action) && !step.Action.Match {
			writeExplainAction(&b, step.Action)
		}
	}
	if !wrote {
		b.WriteString("│\n│ No mismatches found.\n")
	}
	b.WriteString("╰─")
	return b.String()
}

// FormatBenchmark returns a compact, human-readable benchmark report.
func FormatBenchmark(result ExecuteResult) string {
	result = applyContext(result)
	var b strings.Builder
	b.WriteString("╭─ BENCH ")
	if result.File != "" {
		b.WriteString(result.File)
		if result.Name != "" {
			b.WriteString("::")
			b.WriteString(result.Name)
		}
	} else if result.Name != "" {
		b.WriteString(result.Name)
	} else {
		b.WriteString("Gox Benchmark Result")
	}
	b.WriteByte('\n')
	if result.Benchmark == nil {
		b.WriteString("│\n│ No benchmark data.\n╰─")
		return b.String()
	}
	bench := result.Benchmark
	if len(bench.Steps) > 1 {
		b.WriteString("│\n│ pipeline\n")
		writeBenchmarkMetric(&b, "iterations", fmt.Sprintf("%d", bench.Iterations))
		writeBenchmarkMetric(&b, "steps", fmt.Sprintf("%d", len(bench.Steps)))
		writeBenchmarkMetric(&b, "measured_ops", fmt.Sprintf("%d", bench.Iterations*len(bench.Steps)))
	}
	b.WriteString("│\n│ metrics\n")
	writeBenchmarkMetric(&b, "ns/op", fmt.Sprintf("%.0f", bench.NsPerOp))
	writeBenchmarkMetric(&b, "ops/sec", fmt.Sprintf("%.2f", bench.OpsPerSec))
	writeBenchmarkMetric(&b, "bytes_received/op", fmt.Sprintf("%.0f", bench.BytesRecvPerOp))
	writeBenchmarkMetric(&b, "bytes_sent/op", fmt.Sprintf("%.0f", bench.BytesSentPerOp))
	writeBenchmarkMetric(&b, "handler_allocs/op", formatBenchmarkAllocsOp(bench.HandlerAllocsOp))
	writeBenchmarkMetric(&b, "handler_bytes/op", fmt.Sprintf("%.0f", bench.HandlerBytesOp))

	if len(bench.Status) > 0 {
		b.WriteString("│\n│ status\n")
		keys := make([]int, 0, len(bench.Status))
		for status := range bench.Status {
			keys = append(keys, status)
		}
		sort.Ints(keys)
		for _, status := range keys {
			b.WriteString("│   ")
			b.WriteString(fmt.Sprintf("%d", status))
			b.WriteString("  ")
			b.WriteString(fmt.Sprintf("%d", bench.Status[status]))
			b.WriteByte('\n')
		}
	}
	if bench.AllocationSamples > 0 {
		b.WriteString("│\n│ allocation sampling\n")
		b.WriteString("│   ")
		b.WriteString(formatBenchmarkAllocationSampling(bench.AllocationSamples, bench.SampleRate))
		b.WriteByte('\n')
	}

	if len(bench.Steps) > 1 {
		b.WriteString("│\n│ steps\n")
		for i, step := range bench.Steps {
			if i > 0 {
				b.WriteString("│\n")
			}
			writeBenchmarkStepBlock(&b, step)
		}
	}

	if len(bench.AllocationSites) > 0 && len(bench.Steps) <= 1 {
		b.WriteString("│\n│ allocation sites\n")
		limit := len(bench.AllocationSites)
		if limit > 5 {
			limit = 5
		}
		for i := 0; i < limit; i++ {
			b.WriteString("│   ")
			b.WriteString(fmt.Sprintf("%d. ", i+1))
			b.WriteString(formatBenchmarkAllocationSite(bench.AllocationSites[i]))
			b.WriteByte('\n')
		}
	}
	b.WriteString("╰─")
	return b.String()
}

func writeBenchmarkStepBlock(b *strings.Builder, step BenchmarkStepResult) {
	b.WriteString("│   ")
	b.WriteString(fmt.Sprintf("%d. ", step.Index))
	b.WriteString(benchmarkStepLabel(step))
	b.WriteByte('\n')
	b.WriteString("│   │\n")
	b.WriteString("│   │ metrics\n")
	writeBenchmarkStepMetric(b, "ns/op", fmt.Sprintf("%.0f", step.NsPerOp))
	writeBenchmarkStepMetric(b, "bytes_received/op", fmt.Sprintf("%.0f", step.BytesRecvPerOp))
	writeBenchmarkStepMetric(b, "bytes_sent/op", fmt.Sprintf("%.0f", step.BytesSentPerOp))
	writeBenchmarkStepMetric(b, "handler_allocs/op", formatBenchmarkAllocsOp(step.HandlerAllocsOp))
	writeBenchmarkStepMetric(b, "handler_bytes/op", fmt.Sprintf("%.0f", step.HandlerBytesOp))

	if len(step.Status) > 0 {
		b.WriteString("│   │\n")
		b.WriteString("│   │ status\n")
		keys := make([]int, 0, len(step.Status))
		for code := range step.Status {
			keys = append(keys, code)
		}
		sort.Ints(keys)
		for _, code := range keys {
			b.WriteString("│   │   ")
			b.WriteString(fmt.Sprintf("%d", code))
			b.WriteString("  ")
			b.WriteString(fmt.Sprintf("%d", step.Status[code]))
			b.WriteByte('\n')
		}
	}

	if step.AllocationSamples > 0 {
		b.WriteString("│   │\n")
		b.WriteString("│   │ sampling\n")
		b.WriteString("│   │   ")
		b.WriteString(formatBenchmarkAllocationSampling(step.AllocationSamples, 0))
		b.WriteByte('\n')
	}

	if len(step.AllocationSites) > 0 {
		b.WriteString("│   │\n")
		b.WriteString("│   │ allocation sites\n")
		limit := len(step.AllocationSites)
		if limit > 3 {
			limit = 3
		}
		for i := 0; i < limit; i++ {
			b.WriteString("│   │   ")
			b.WriteString(fmt.Sprintf("%d. ", i+1))
			b.WriteString(formatBenchmarkAllocationSite(step.AllocationSites[i]))
			b.WriteByte('\n')
		}
	}
	b.WriteString("│   ╰─\n")
}

func writeBenchmarkStepMetric(b *strings.Builder, name string, value string) {
	b.WriteString("│   │   ")
	b.WriteString(name)
	if padding := 20 - len(name); padding > 0 {
		b.WriteString(strings.Repeat(" ", padding))
	} else {
		b.WriteByte(' ')
	}
	b.WriteString(value)
	b.WriteByte('\n')
}

func formatBenchmarkAllocsOp(value float64) string {
	return fmt.Sprintf("%.0f", value)
}

func formatBenchmarkStatusInline(status map[int]int) string {
	if len(status) == 0 {
		return ""
	}
	keys := make([]int, 0, len(status))
	for code := range status {
		keys = append(keys, code)
	}
	sort.Ints(keys)
	parts := make([]string, 0, len(keys))
	for _, code := range keys {
		parts = append(parts, fmt.Sprintf("%d=%d", code, status[code]))
	}
	return strings.Join(parts, " ")
}

func formatBenchmarkAllocationSampling(samples int, sampleRate float64) string {
	if samples <= 0 {
		return ""
	}
	if sampleRate > 0 {
		percent := sampleRate * 100
		rate := fmt.Sprintf("%.2f%%", percent)
		if percent == float64(int(percent)) {
			rate = fmt.Sprintf("%d%%", int(percent))
		}
		return fmt.Sprintf("%s (%d samples)", rate, samples)
	}
	return fmt.Sprintf("%d samples", samples)
}

func formatBenchmarkAllocationSite(site BenchmarkAllocationSite) string {
	location := site.File
	if site.Line > 0 {
		location = fmt.Sprintf("%s:%d", location, site.Line)
	}
	name := cleanBenchmarkFormatName(site.Name)
	if name != "" {
		location = strings.TrimSpace(location + " " + name)
	}
	return fmt.Sprintf("%s  %d allocs  %d B", location, site.Allocs, site.Bytes)
}

func benchmarkStepLabel(step BenchmarkStepResult) string {
	kind := strings.ToLower(strings.TrimSpace(step.Type))
	method := strings.TrimSpace(step.Method)
	url := strings.TrimSpace(step.URL)
	if kind == "" {
		kind = "step"
	}
	parts := []string{kind}
	if method != "" {
		parts = append(parts, method)
	}
	if url != "" {
		parts = append(parts, url)
	}
	return strings.Join(parts, " ")
}
func benchmarkStatusSummary(status map[int]int) string {
	if len(status) == 0 {
		return ""
	}
	keys := make([]int, 0, len(status))
	for code := range status {
		keys = append(keys, code)
	}
	sort.Ints(keys)
	parts := make([]string, 0, len(keys))
	for _, code := range keys {
		parts = append(parts, fmt.Sprintf("%d=%d", code, status[code]))
	}
	return strings.Join(parts, ",")
}

func writeBenchmarkMetric(b *strings.Builder, name string, value string) {
	b.WriteString("│   ")
	b.WriteString(name)
	if padding := 20 - len(name); padding > 0 {
		b.WriteString(strings.Repeat(" ", padding))
	} else {
		b.WriteByte(' ')
	}
	b.WriteString(value)
	b.WriteByte('\n')
}

func cleanBenchmarkFormatName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if idx := strings.Index(name, ")."); idx >= 0 && idx+2 < len(name) {
		name = name[idx+2:]
	}
	name = strings.ReplaceAll(name, "-fm", "")
	return strings.TrimSpace(name)
}

func writeExplainHeaderFailures(b *strings.Builder, prefix string, result HeaderMatchResult) {
	if !headerMatchHasData(result) || result.Match {
		return
	}
	b.WriteString("│\n│ ")
	b.WriteString(prefix)
	b.WriteString(" mismatch")
	if result.Rule != "" {
		b.WriteString(" (")
		b.WriteString(result.Rule)
		b.WriteString(")")
	}
	b.WriteByte('\n')
	keys := make([]string, 0, len(result.Fields))
	for key := range result.Fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		field := result.Fields[key]
		if field.Match {
			continue
		}
		b.WriteString("│   ")
		b.WriteString(key)
		if field.Rule != "" {
			b.WriteString("  ")
			b.WriteString(field.Rule)
		}
		b.WriteByte('\n')
		if explainHasValue(field.Expected) {
			b.WriteString("│     expected  ")
			b.WriteString(formatExplainValue(field.Expected))
			b.WriteByte('\n')
		}
		if explainHasValue(field.Response) {
			b.WriteString("│     response  ")
			b.WriteString(formatExplainValue(field.Response))
			b.WriteByte('\n')
		} else {
			b.WriteString("│     response  <missing>\n")
		}
		if field.Message != "" {
			b.WriteString("│     message   ")
			b.WriteString(field.Message)
			b.WriteByte('\n')
		}
	}
}

func writeExplainBody(b *strings.Builder, body BodyMatchResult) {
	if len(body.Rules) > 0 {
		for _, rule := range body.Rules {
			if !rule.Match {
				writeExplainBody(b, rule)
			}
		}
		return
	}

	if len(body.Findings) > 0 {
		writeExplainBodyFindings(b, body)
		return
	}

	if len(body.JSONFindings) > 0 {
		writeExplainJSONFindings(b, "Body", body.Rule, body.Message, body.JSONFindings)
		return
	}

	if body.Diff != nil {
		writeExplainBodyDiff(b, body)
		return
	}

	writeExplainField(b, "Body mismatch ("+body.Rule+")", explainBodyValue(body.Expected), explainBodyValue(body.Response), body.Message)
}

func writeExplainBodyFindings(b *strings.Builder, body BodyMatchResult) {
	b.WriteString("│\n│ Body mismatch (")
	b.WriteString(body.Rule)
	b.WriteString(")\n")
	if body.Message != "" {
		b.WriteString("│   message   ")
		b.WriteString(body.Message)
		b.WriteByte('\n')
	}
	for _, finding := range body.Findings {
		shouldPrint := false
		switch body.Rule {
		case "contains":
			shouldPrint = !finding.Found
		case "notContains":
			shouldPrint = finding.Found
		default:
			shouldPrint = !finding.Found
		}
		if !shouldPrint {
			continue
		}
		b.WriteString("│   pattern   ")
		b.WriteString(formatExplainValue(finding.Pattern))
		b.WriteByte('\n')
		if finding.Found {
			b.WriteString("│   found     line ")
			b.WriteString(fmt.Sprint(finding.Line))
			b.WriteString(", column ")
			b.WriteString(fmt.Sprint(finding.Column))
			b.WriteByte('\n')
			if finding.Snippet != "" {
				b.WriteString("│   snippet   ")
				b.WriteString(formatExplainValue(finding.Snippet))
				b.WriteByte('\n')
			}
		} else {
			b.WriteString("│   found     false\n")
		}
	}
}

func writeExplainBodyDiff(b *strings.Builder, body BodyMatchResult) {
	diff := body.Diff
	b.WriteString("│\n│ Body mismatch (")
	b.WriteString(body.Rule)
	b.WriteString(")\n")
	if body.Message != "" {
		b.WriteString("│   message   ")
		b.WriteString(body.Message)
		b.WriteByte('\n')
	}
	b.WriteString("│   first diff line ")
	b.WriteString(fmt.Sprint(diff.Line))
	b.WriteString(", column ")
	b.WriteString(fmt.Sprint(diff.Column))
	b.WriteByte('\n')
	if diff.ExpectedSnippet != "" {
		b.WriteString("│   expected  ")
		b.WriteString(formatExplainValue(explainBodySummary(formatBodySummary(diff.ExpectedSnippet))))
		b.WriteByte('\n')
	}
	if diff.ResponseSnippet != "" {
		b.WriteString("│   response  ")
		b.WriteString(formatExplainValue(explainBodySummary(formatBodySummary(diff.ResponseSnippet))))
		b.WriteByte('\n')
	}
}

func writeExplainAction(b *strings.Builder, action ActionMatchResult) {
	if actionValueMatchHasData(action.Error) && !action.Error.Match {
		writeExplainActionValue(b, "Action error", action.Error)
	}
	if actionValueMatchHasData(action.Result) && !action.Result.Match {
		writeExplainActionValue(b, "Action result", action.Result)
	}
}

func writeExplainActionValue(b *strings.Builder, title string, value ActionValueMatchResult) {
	b.WriteString("│\n│ ")
	b.WriteString(title)
	b.WriteString(" mismatch")
	if value.Rule != "" {
		b.WriteString(" (")
		b.WriteString(value.Rule)
		b.WriteString(")")
	}
	b.WriteByte('\n')
	if value.Message != "" {
		b.WriteString("│   message   ")
		b.WriteString(value.Message)
		b.WriteByte('\n')
	}
	if len(value.JSONFindings) > 0 {
		writeExplainJSONFindingLines(b, value.JSONFindings, "│   ")
		return
	}
	if len(value.Fields) > 0 {
		keys := make([]string, 0, len(value.Fields))
		for key := range value.Fields {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			field := value.Fields[key]
			if field.Match {
				continue
			}
			b.WriteString("│   path      ")
			b.WriteString(key)
			b.WriteByte('\n')
			if explainHasValue(field.Expected) {
				b.WriteString("│   expected  ")
				b.WriteString(formatExplainValue(field.Expected))
				b.WriteByte('\n')
			}
			if explainHasValue(field.Response) {
				b.WriteString("│   response  ")
				b.WriteString(formatExplainValue(field.Response))
				b.WriteByte('\n')
			} else {
				b.WriteString("│   response  <missing>\n")
			}
		}
		return
	}
	writeExplainField(b, title+" mismatch ("+value.Rule+")", value.Expected, value.Response, "")
}

func writeExplainJSONFindings(b *strings.Builder, section string, rule string, message string, findings []JSONMatchFinding) {
	b.WriteString("│\n│ ")
	b.WriteString(section)
	b.WriteString(" mismatch")
	if rule != "" {
		b.WriteString(" (")
		b.WriteString(rule)
		b.WriteString(")")
	}
	b.WriteByte('\n')
	if message != "" {
		b.WriteString("│   message   ")
		b.WriteString(message)
		b.WriteByte('\n')
	}
	writeExplainJSONFindingLines(b, findings, "│   ")
}

func writeExplainJSONFindingLines(b *strings.Builder, findings []JSONMatchFinding, prefix string) {
	for _, finding := range findings {
		if finding.Match {
			continue
		}
		if finding.Path != "" {
			b.WriteString(prefix)
			b.WriteString("path      ")
			b.WriteString(finding.Path)
			b.WriteByte('\n')
		}
		if finding.Message != "" {
			b.WriteString(prefix)
			b.WriteString("message   ")
			b.WriteString(finding.Message)
			b.WriteByte('\n')
		}
		if explainHasValue(finding.Expected) {
			b.WriteString(prefix)
			b.WriteString("expected  ")
			b.WriteString(formatExplainValue(finding.Expected))
			b.WriteByte('\n')
		}
		if explainHasValue(finding.Response) {
			b.WriteString(prefix)
			b.WriteString("response  ")
			b.WriteString(formatExplainValue(finding.Response))
			b.WriteByte('\n')
		} else if finding.Found {
			b.WriteString(prefix)
			b.WriteString("response  <found>\n")
		} else {
			b.WriteString(prefix)
			b.WriteString("response  <missing>\n")
		}
	}
}

func writeExplainField(b *strings.Builder, title string, expected any, response any, message string) {
	b.WriteString("│\n│ ")
	b.WriteString(title)
	b.WriteString("\n")
	if explainHasValue(expected) {
		b.WriteString("│   expected  ")
		b.WriteString(formatExplainValue(expected))
		b.WriteByte('\n')
	}
	if explainHasValue(response) {
		b.WriteString("│   response  ")
		b.WriteString(formatExplainValue(response))
		b.WriteByte('\n')
	}
	if message != "" {
		b.WriteString("│   message   ")
		b.WriteString(message)
		b.WriteByte('\n')
	}
}

func explainHasValue(value any) bool {
	if value == nil {
		return false
	}
	if text, ok := value.(string); ok {
		return text != ""
	}
	return true
}

func formatExplainValue(value any) string {
	if value == nil {
		return "<nil>"
	}
	switch v := value.(type) {
	case explainBodySummary:
		return string(v)
	case string:
		return fmt.Sprintf("%q", summarizeExplainText(v, 160))
	default:
		data, err := json.Marshal(v)
		if err == nil {
			return summarizeExplainText(string(data), 180)
		}
		return summarizeExplainText(fmt.Sprint(v), 180)
	}
}

type explainBodySummary string

func explainBodyValue(value any) any {
	text, ok := value.(string)
	if !ok {
		return value
	}
	return explainBodySummary(formatBodySummary(text))
}

func formatBodySummary(text string) string {
	const limit = 220
	preview := summarizeExplainText(text, limit)
	if len(text) <= limit {
		return fmt.Sprintf("%q", preview)
	}
	return fmt.Sprintf("%q  <body truncated: %d bytes total>", preview, len(text))
}

func summarizeExplainText(text string, limit int) string {
	text = compactExplainWhitespace(text)
	if limit <= 0 || len(text) <= limit {
		return text
	}
	if limit > 1 {
		limit--
	}
	return text[:limit] + "…"
}

func compactExplainWhitespace(text string) string {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return ""
	}
	return strings.Join(fields, " ")
}
