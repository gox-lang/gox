package test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExpectedHeadersPartial(t *testing.T) {
	rule := ExpectedHeaders.Contains(Map{
		"Content-Type": "text/html",
	})
	got := rule.MatchHeaders(map[string][]string{
		"Content-Type": {"text/html"},
		"X-Extra":      {"1"},
	})
	if !got.Match {
		t.Fatalf("match = false: %#v", got)
	}
}

func TestExpectedHeadersStrictExtraFails(t *testing.T) {
	rule := ExpectedHeaders.Equal(Map{
		"Content-Type": "text/html",
	})
	got := rule.MatchHeaders(map[string][]string{
		"Content-Type": {"text/html"},
		"X-Extra":      {"1"},
	})
	if got.Match {
		t.Fatalf("match = true, want false")
	}
	if got.Fields["x-extra"].Match {
		t.Fatalf("extra field should not match: %#v", got.Fields["x-extra"])
	}
}

func TestExpectedHeadersExclude(t *testing.T) {
	rule := ExpectedHeaders.NotContains(Map{
		"X-Debug": "",
	})
	got := rule.MatchHeaders(map[string][]string{
		"Content-Type": {"text/html"},
	})
	if !got.Match {
		t.Fatalf("match = false: %#v", got)
	}

	got = rule.MatchHeaders(map[string][]string{
		"X-Debug": {"true"},
	})
	if got.Match {
		t.Fatalf("match = true, want false")
	}
}

func TestExpectedBodyContains(t *testing.T) {
	got := ExpectedBody.Contains("Dashboard").MatchBody([]byte("<h1>Dashboard</h1>"))
	if !got.Match {
		t.Fatalf("match = false: %#v", got)
	}
}

func TestExpectedBodyJSONPath(t *testing.T) {
	body := []byte(`{"r":{"ok":true,"id":123},"e":null}`)
	got := ExpectedBody.JSONContains(Map{"r": Map{"id": 123}}).MatchBody(body)
	if !got.Match {
		t.Fatalf("match = false: %#v", got)
	}
}

func TestExpectedActionErrorNil(t *testing.T) {
	got := ExpectedActionError.Nil().MatchActionError(nil)
	if !got.Match {
		t.Fatalf("match = false: %#v", got)
	}
	got = ExpectedActionError.Nil().MatchActionError("boom")
	if got.Match {
		t.Fatalf("match = true, want false")
	}
}

func TestExpectedActionResultPartial(t *testing.T) {
	rule := ExpectedActionResult.Partial(map[string]any{"ok": true})
	got := rule.MatchActionResult(map[string]any{"ok": true, "id": 123})
	if !got.Match {
		t.Fatalf("match = false: %#v", got)
	}
}

func TestRequestBodyJSON(t *testing.T) {
	body := RequestBody.JSON(map[string]any{"name": "milo"})
	data, err := body.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["name"] != "milo" {
		t.Fatalf("name = %#v", decoded["name"])
	}
}

type fakeExecutor struct{}

func (fakeExecutor) Execute(config Config, pipeline []ExecutePipeLine) (ExecuteResult, bool) {
	return ExecuteResult{
		Match: true,
		Steps: []StepResult{{Index: 1, Type: "request", Method: "GET", URL: "/", Match: true}},
	}, true
}

func TestSaveWritesCurrentResult(t *testing.T) {
	SetExecutor(fakeExecutor{})
	defer SetExecutor(nil)

	dir := t.TempDir()
	SetRuntimeContext(RuntimeContext{
		File:       "tests/index.gox",
		Name:       "IndexTest",
		Mode:       ModeTest,
		ResultRoot: dir,
	})
	defer ClearRuntimeContext()

	result, match := Execute(Config{}, []ExecutePipeLine{
		{Request: NewRequest{Method: GET, URL: "/"}},
	})
	if !match || !result.Match {
		t.Fatalf("result = %#v match=%v", result, match)
	}

	Save(result)

	data, err := os.ReadFile(filepath.Join(dir, "IndexTest", "test.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("empty output")
	}
}

func TestIsBenchAndPassFailMessages(t *testing.T) {
	SetRuntimeContext(RuntimeContext{Name: "BenchTest", Mode: ModeBench, ResultRoot: t.TempDir()})
	defer ClearRuntimeContext()

	if !IsBench() {
		t.Fatal("IsBench = false, want true")
	}

	Pass("ok")
	if got := CurrentMessages(); len(got.Pass) != 1 || got.Pass[0] != "ok" {
		t.Fatalf("pass messages = %#v", got.Pass)
	}

	Fail("bad")
	if got := CurrentMessages(); len(got.Fail) != 1 || got.Fail[0] != "bad" {
		t.Fatalf("fail messages = %#v", got.Fail)
	}
}

func TestExplainIncludesMismatch(t *testing.T) {
	result := ExecuteResult{
		File:  "tests/index.gox",
		Name:  "IndexTest",
		Mode:  ModeTest,
		Match: false,
		Steps: []StepResult{{
			Index:  1,
			Type:   "request",
			Method: GET,
			URL:    "/index",
			Match:  false,
			Status: MatchValue{Rule: "status", Expected: 200, Response: 404, Match: false},
			Body:   BodyMatchResult{Rule: "contains", Expected: []string{"Hello"}, Response: "Not Found", Match: false},
		}},
	}

	SetRuntimeContext(RuntimeContext{File: "tests/index.gox", Name: "IndexTest", Mode: ModeTest})
	defer ClearRuntimeContext()

	Explain(result)
	messages := CurrentMessages()
	if len(messages.Explain) != 1 {
		t.Fatalf("explain messages = %#v", messages.Explain)
	}
	out := messages.Explain[0]
	if !strings.Contains(out, "FAIL tests/index.gox::IndexTest") || !strings.Contains(out, "Status mismatch") || !strings.Contains(out, "Body mismatch") {
		t.Fatalf("unexpected explain output:\n%s", out)
	}
}

func TestExpectedBodyContainsMultiple(t *testing.T) {
	got := ExpectedBody.Contains("Hello", "Gox").MatchBody([]byte("<h1>Hello Gox</h1>"))
	if !got.Match {
		t.Fatalf("match = false: %#v", got)
	}
	got = ExpectedBody.Contains("Hello", "Missing").MatchBody([]byte("<h1>Hello Gox</h1>"))
	if got.Match {
		t.Fatalf("match = true, want false")
	}
}

func TestExpectedBodyNotContains(t *testing.T) {
	got := ExpectedBody.NotContains("panic", "debug").MatchBody([]byte("ok"))
	if !got.Match {
		t.Fatalf("match = false: %#v", got)
	}
	got = ExpectedBody.NotContains("debug").MatchBody([]byte("debug enabled"))
	if got.Match {
		t.Fatalf("match = true, want false")
	}
}

func TestExpectedBodyEqualAndNotEqual(t *testing.T) {
	if got := ExpectedBody.Equal("ok").MatchBody([]byte("ok")); !got.Match {
		t.Fatalf("equal = %#v", got)
	}
	if got := ExpectedBody.NotEqual("forbidden").MatchBody([]byte("ok")); !got.Match {
		t.Fatalf("not equal = %#v", got)
	}
}

func TestExpectedBodyJSONEqual(t *testing.T) {
	body := []byte(`{"ok":true,"id":123}`)
	got := ExpectedBody.JSONEqual(Map{"ok": true, "id": 123}).MatchBody(body)
	if !got.Match {
		t.Fatalf("match = false: %#v", got)
	}
}

func TestExpectedBodyJSONNotEqual(t *testing.T) {
	body := []byte(`{"ok":true,"id":123}`)
	got := ExpectedBody.JSONNotEqual(Map{"ok": false, "id": 123}).MatchBody(body)
	if !got.Match {
		t.Fatalf("match = false: %#v", got)
	}
}

func TestExpectedBodyJSONContains(t *testing.T) {
	body := []byte(`{"ok":true,"user":{"id":123,"name":"Milo"},"extra":1}`)
	got := ExpectedBody.JSONContains(Map{"user": Map{"id": 123}}).MatchBody(body)
	if !got.Match {
		t.Fatalf("match = false: %#v", got)
	}
	got = ExpectedBody.JSONContains(Map{"id": 123}).MatchBody(body)
	if got.Match {
		t.Fatalf("root-only contains matched nested key")
	}
}

func TestExpectedBodyJSONNotContains(t *testing.T) {
	body := []byte(`{"user":{"id":123,"name":"Milo"}}`)
	got := ExpectedBody.JSONNotContains(Map{"user": Map{"password": "123"}}).MatchBody(body)
	if !got.Match {
		t.Fatalf("match = false: %#v", got)
	}
	got = ExpectedBody.JSONNotContains(Map{"user": Map{"id": 123}}).MatchBody(body)
	if got.Match {
		t.Fatalf("match = true, want false")
	}
}

func TestExpectedCookiesContains(t *testing.T) {
	rule := ExpectedCookies.Contains(Map{"SessionID": ""})
	got := rule.MatchCookies([]*http.Cookie{{Name: "SessionID", Value: "abc"}})
	if !got.Match {
		t.Fatalf("match = false: %#v", got)
	}
}

func TestExpectedHeadersCustom(t *testing.T) {
	rule := ExpectedHeaders.Custom(map[string]ExpectedStringRule{
		"Content-Type":  Expect.Contains("text/html"),
		"Cache-Control": Expect.Equal("no-store"),
		"X-Debug":       Expect.NotExists(),
	})
	got := rule.MatchHeaders(map[string][]string{
		"Content-Type":  {"text/html; charset=utf-8"},
		"Cache-Control": {"no-store"},
	})
	if !got.Match {
		t.Fatalf("match = false: %#v", got)
	}

	got = rule.MatchHeaders(map[string][]string{
		"Content-Type":  {"application/json"},
		"Cache-Control": {"no-store"},
		"X-Debug":       {"true"},
	})
	if got.Match {
		t.Fatalf("match = true, want false")
	}
	if got.Fields["Content-Type"].Match {
		t.Fatalf("content-type should not match: %#v", got.Fields["Content-Type"])
	}
	if got.Fields["X-Debug"].Match {
		t.Fatalf("x-debug should not match: %#v", got.Fields["X-Debug"])
	}
}

func TestExpectedCookiesCustom(t *testing.T) {
	rule := ExpectedCookies.Custom(map[string]ExpectedStringRule{
		"SessionID": Expect.Exists(),
		"Debug":     Expect.NotExists(),
		"Theme":     Expect.NotEqual("light"),
	})
	got := rule.MatchCookies([]*http.Cookie{
		{Name: "SessionID", Value: "abc"},
		{Name: "Theme", Value: "dark"},
	})
	if !got.Match {
		t.Fatalf("match = false: %#v", got)
	}

	got = rule.MatchCookies([]*http.Cookie{
		{Name: "Debug", Value: "1"},
		{Name: "Theme", Value: "light"},
	})
	if got.Match {
		t.Fatalf("match = true, want false")
	}
}

func TestExpectedBodyCustom(t *testing.T) {
	body := []byte(`{"ok":true,"user":{"id":123,"name":"Milo"},"html":"<h1>Dashboard</h1>"}`)
	rule := ExpectedBody.Custom(
		Expect.Contains("Dashboard"),
		Expect.NotContains("panic"),
		Expect.JSONContains(Map{"user": Map{"id": 123}}),
		Expect.JSONNotContains(Map{"user": Map{"password": "secret"}}),
	)
	got := rule.MatchBody(body)
	if !got.Match {
		t.Fatalf("match = false: %#v", got)
	}
	if len(got.Rules) != 4 {
		t.Fatalf("rules len = %d, want 4", len(got.Rules))
	}

	got = ExpectedBody.Custom(
		Expect.Contains("missing"),
		Expect.JSONNotContains(Map{"user": Map{"id": 123}}),
	).MatchBody(body)
	if got.Match {
		t.Fatalf("match = true, want false")
	}
}

func TestExplainBodyIsTruncated(t *testing.T) {
	longBody := strings.Repeat("<section>Hello Gox</section>", 80)
	result := ExecuteResult{
		Match: false,
		Steps: []StepResult{
			{
				Index:  1,
				Type:   "request",
				Method: "GET",
				URL:    "/index",
				Match:  false,
				Body: BodyMatchResult{
					Rule:     "contains",
					Expected: "Missing",
					Response: longBody,
					Match:    false,
				},
			},
		},
	}

	explained := Format(result)
	if !strings.Contains(explained, "body truncated") {
		t.Fatalf("expected truncated body summary, got:\n%s", explained)
	}
	if strings.Count(explained, "<section>") > 12 {
		t.Fatalf("explain printed too much body: %d section tags", strings.Count(explained, "<section>"))
	}
}

func TestExpectedBodyContainsFindings(t *testing.T) {
	body := []byte("<html>\n<body>\n<h1>Hello Gox</h1>\n</body>\n</html>")
	got := ExpectedBody.Contains("Hello", "Missing").MatchBody(body)
	if got.Match {
		t.Fatalf("match = true, want false")
	}
	if len(got.Findings) != 2 {
		t.Fatalf("findings len = %d, want 2", len(got.Findings))
	}
	if !got.Findings[0].Found || got.Findings[0].Line != 3 || got.Findings[0].Column != 5 {
		t.Fatalf("hello finding = %#v", got.Findings[0])
	}
	if got.Findings[1].Found {
		t.Fatalf("missing finding = %#v", got.Findings[1])
	}
}

func TestExpectedBodyNotContainsFindingLocation(t *testing.T) {
	body := []byte("<html>\n<body>\n<input name=\"password\" value=\"123\">\n</body>\n</html>")
	got := ExpectedBody.NotContains("password").MatchBody(body)
	if got.Match {
		t.Fatalf("match = true, want false")
	}
	if len(got.Findings) != 1 {
		t.Fatalf("findings len = %d, want 1", len(got.Findings))
	}
	finding := got.Findings[0]
	if !finding.Found || finding.Line != 3 || finding.Column != 14 || !strings.Contains(finding.Snippet, "password") {
		t.Fatalf("finding = %#v", finding)
	}

	explained := Format(ExecuteResult{Match: false, Steps: []StepResult{{Index: 1, Type: "request", Method: GET, URL: "/profile", Match: false, Body: got}}})
	if !strings.Contains(explained, "line 3, column 14") || !strings.Contains(explained, "snippet") {
		t.Fatalf("explain missing finding details:\n%s", explained)
	}
}

func TestExpectedBodyEqualDiff(t *testing.T) {
	got := ExpectedBody.Equal("<h1>Hello Gox</h1>").MatchBody([]byte("<h1>Hello GoX</h1>"))
	if got.Match {
		t.Fatalf("match = true, want false")
	}
	if got.Diff == nil {
		t.Fatal("diff = nil")
	}
	if got.Diff.Line != 1 || got.Diff.Column == 0 || !strings.Contains(got.Diff.ExpectedSnippet, "Gox") || !strings.Contains(got.Diff.ResponseSnippet, "GoX") {
		t.Fatalf("diff = %#v", got.Diff)
	}

	explained := Format(ExecuteResult{Match: false, Steps: []StepResult{{Index: 1, Type: "request", Method: GET, URL: "/index", Match: false, Body: got}}})
	if !strings.Contains(explained, "first diff line") {
		t.Fatalf("explain missing diff:\n%s", explained)
	}
}

func TestExpectedBodySnapshotDiff(t *testing.T) {
	got := ExpectedBodySnapshot{File: "index.html", Content: "<h1>Hello Gox</h1>"}.MatchBody([]byte("<h1>Hello GoX</h1>"))
	if got.Match {
		t.Fatalf("match = true, want false")
	}
	if got.Diff == nil {
		t.Fatal("diff = nil")
	}
}

func TestSaveWritesResponseArtifactsInTestMode(t *testing.T) {
	dir := t.TempDir()
	SetRuntimeContext(RuntimeContext{Name: "IndexTest", Mode: ModeTest, ResultRoot: dir})
	defer ClearRuntimeContext()

	result := ExecuteResult{
		Match: true,
		Steps: []StepResult{{
			Index:  1,
			Type:   "request",
			Method: GET,
			URL:    "/index",
			Match:  true,
			Response: ResponseSnapshot{
				StatusCode: 200,
				Status:     "200 OK",
				Headers: map[string][]string{
					"Content-Type": {"text/html; charset=utf-8"},
					"Set-Cookie":   {"sid=abc; Path=/"},
				},
				Body: "<h1>Hello Gox</h1>",
			},
		}},
	}

	Save(result)

	responsePath := filepath.Join(dir, "IndexTest", "responses", "001_GET__index.text")
	data, err := os.ReadFile(responsePath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "HTTP/1.1 200 OK") || !strings.Contains(text, "Content-Type: text/html; charset=utf-8") || !strings.Contains(text, "Set-Cookie: sid=abc; Path=/") || !strings.Contains(text, "<h1>Hello Gox</h1>") {
		t.Fatalf("response artifact mismatch:\n%s", text)
	}
}

func TestSaveSkipsResponseArtifactsInBenchMode(t *testing.T) {
	dir := t.TempDir()
	SetRuntimeContext(RuntimeContext{Name: "IndexTest", Mode: ModeBench, ResultRoot: dir})
	defer ClearRuntimeContext()

	result := ExecuteResult{
		Match: true,
		Steps: []StepResult{{
			Index:    1,
			Type:     "request",
			Method:   GET,
			URL:      "/index",
			Match:    true,
			Response: ResponseSnapshot{StatusCode: 200, Status: "200 OK", Body: "hello"},
		}},
	}

	Save(result)

	if _, err := os.Stat(filepath.Join(dir, "IndexTest", "bench.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "IndexTest", "responses")); !os.IsNotExist(err) {
		t.Fatalf("responses dir exists in bench mode: %v", err)
	}
}

func TestExplainHeaderCookieJSONActionDetails(t *testing.T) {
	result := ExecuteResult{
		File:  "tests/detail.gox",
		Name:  "DetailTest",
		Mode:  ModeTest,
		Match: false,
		Steps: []StepResult{
			{
				Index:  1,
				Type:   "request",
				Method: GET,
				URL:    "/detail",
				Match:  false,
				Headers: ExpectedHeaders.Contains(Map{
					"Content-Type": "text/html",
				}).MatchHeaders(map[string][]string{
					"Content-Type": {"application/json"},
				}),
				Cookies: ExpectedCookies.NotContains(Map{
					"Debug": "",
				}).MatchCookies([]*http.Cookie{{Name: "Debug", Value: "1"}}),
				Body: ExpectedBody.JSONContains(Map{
					"user": Map{"name": "Milo"},
				}).MatchBody([]byte(`{"user":{"id":123}}`)),
			},
			{
				Index:  2,
				Type:   "action",
				URL:    "/users",
				Method: "SaveUser",
				Match:  false,
				Action: ActionMatchResult{
					Match: false,
					Result: ExpectedActionResult.Partial(map[string]any{
						"user": Map{"id": 123, "name": "Milo"},
					}).MatchActionResult(map[string]any{
						"user": map[string]any{"id": 999},
					}),
				},
			},
		},
	}

	out := Format(result)
	checks := []string{
		"Header mismatch (contains)",
		"field value mismatch",
		"Cookie mismatch (notContains)",
		"field must not exist",
		"Body mismatch (jsonContains)",
		"path      user.name",
		"Action result mismatch (partial)",
		"path      user.id",
		"path      user.name",
	}
	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Fatalf("explain missing %q:\n%s", check, out)
		}
	}
}

func TestSaveWritesBenchJSONWithoutResponses(t *testing.T) {
	dir := t.TempDir()
	SetRuntimeContext(RuntimeContext{
		File:       "tests/index.gox",
		Name:       "IndexTest",
		Mode:       ModeBench,
		ResultRoot: dir,
	})
	defer ClearRuntimeContext()

	result := ExecuteResult{
		Match: true,
		Steps: []StepResult{{
			Index:  1,
			Type:   "request",
			Method: GET,
			URL:    "/index",
			Match:  true,
			Response: ResponseSnapshot{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       "benchmark should not write response artifacts",
			},
		}},
		Benchmark: &BenchmarkResult{
			Warmup:     1,
			Iterations: 2,
			DurationNS: 100,
			NsPerOp:    50,
			OpsPerSec:  20_000_000,
			Status:     map[int]int{200: 2},
		},
	}

	Save(result)

	benchPath := filepath.Join(dir, "IndexTest", "bench.json")
	data, err := os.ReadFile(benchPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"mode": "bench"`) || !strings.Contains(string(data), `"benchmark"`) {
		t.Fatalf("bench.json missing benchmark metadata:\n%s", data)
	}
	if _, err := os.Stat(filepath.Join(dir, "IndexTest", "responses")); !os.IsNotExist(err) {
		t.Fatalf("responses directory should not exist in bench mode, err=%v", err)
	}
}

func TestFormatBenchmarkDoesNotUsePassOrFailFrame(t *testing.T) {
	out := Format(ExecuteResult{
		File:  "tests/index.gox",
		Name:  "IndexTest",
		Mode:  ModeBench,
		Match: true,
		Benchmark: &BenchmarkResult{
			Warmup:            1,
			Iterations:        2,
			NsPerOp:           1234,
			OpsPerSec:         810.5,
			BytesRecvPerOp:    99,
			BytesSentPerOp:    55,
			HandlerAllocsOp:   2.5,
			HandlerBytesOp:    512,
			SampleRate:        0.01,
			AllocationSamples: 20,
			Status:            map[int]int{200: 3, 500: 1},
			Steps: []BenchmarkStepResult{
				{Index: 1, Type: "request", Method: GET, URL: "/index", Iterations: 2, AllocationSamples: 10, NsPerOp: 1000, HandlerAllocsOp: 1, Status: map[int]int{200: 2}, AllocationSites: []BenchmarkAllocationSite{{File: "pages/index.gox", Line: 8, Name: "GET", Allocs: 10, Bytes: 2048}}},
				{Index: 2, Type: "action", Method: "Save", URL: "/index", Iterations: 2, AllocationSamples: 10, NsPerOp: 1468, HandlerAllocsOp: 4, Status: map[int]int{200: 1, 500: 1}},
			},
			AllocationSites: []BenchmarkAllocationSite{{File: "pages/index.gox", Line: 8, Name: "unnamed-deadbeef.(*GoxModuleMain).GET-fm", Allocs: 10, Bytes: 2048}},
		},
	})
	for _, forbidden := range []string{"PASS", "FAIL", "No mismatches found"} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("benchmark format contains %q:\n%s", forbidden, out)
		}
	}
	for _, want := range []string{"BENCH tests/index.gox::IndexTest", "pipeline", "iterations", "steps", "measured_ops", "metrics", "ns/op", "allocation sampling", "1% (20 samples)", "1. request GET /index", "2. action Save /index", "bytes_received/op", "sampling", "10 samples", "status", "200  3", "500  1", "200  2", "│   │", "│   ╰─", "site", "pages/index.gox:8 GET"} {
		if !strings.Contains(out, want) {
			t.Fatalf("benchmark format missing %q:\n%s", want, out)
		}
	}
}
