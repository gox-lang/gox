package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/textproto"
	"net/url"
	"sort"
	"strings"

	"github.com/gox-lang/gox"
)

type RequestBodyValue interface {
	isRequestBody()
	Encode() ([]byte, string, error)
}

type RequestBodyRaw struct {
	Content          string
	ContentTypeValue string
}

type RequestBodyQuery struct {
	Values map[string]string
}

type RequestBodyFormData struct {
	Values FormData
}

type RequestBodyJSON struct {
	Value any
}

func (RequestBodyRaw) isRequestBody()      {}
func (RequestBodyQuery) isRequestBody()    {}
func (RequestBodyFormData) isRequestBody() {}
func (RequestBodyJSON) isRequestBody()     {}

func (b RequestBodyRaw) Encode() ([]byte, string, error) {
	if b.ContentTypeValue != "" {
		return []byte(b.Content), b.ContentTypeValue, nil
	}
	return []byte(b.Content), "text/plain; charset=utf-8", nil
}

func (b RequestBodyQuery) Encode() ([]byte, string, error) {
	values := make(url.Values, len(b.Values))
	for k, v := range b.Values {
		values.Set(k, v)
	}
	return []byte(values.Encode()), "application/x-www-form-urlencoded", nil
}

func (b RequestBodyFormData) Encode() ([]byte, string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	keys := make([]string, 0, len(b.Values))
	for key := range b.Values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if err := writeFormValue(writer, key, b.Values[key]); err != nil {
			return nil, "", err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("gox/test: close multipart body: %w", err)
	}
	return body.Bytes(), writer.FormDataContentType(), nil
}

func (b RequestBodyJSON) Encode() ([]byte, string, error) {
	data, err := json.Marshal(b.Value)
	return data, "application/json; charset=utf-8", err
}

func writeFormValue(writer *multipart.Writer, key string, value any) error {
	switch typed := value.(type) {
	case string:
		return writer.WriteField(key, typed)
	case []string:
		for _, item := range typed {
			if err := writer.WriteField(key, item); err != nil {
				return err
			}
		}
		return nil
	case gox.File:
		return writeFormFile(writer, key, typed)
	case []gox.File:
		for _, file := range typed {
			if err := writeFormFile(writer, key, file); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("gox/test: form field %q has unsupported type %T", key, value)
	}
}

func writeFormFile(writer *multipart.Writer, key string, file gox.File) error {
	if file.IsZero() {
		return fmt.Errorf("gox/test: form field %q contains a zero file", key)
	}
	reader, err := file.Open()
	if err != nil {
		return fmt.Errorf("gox/test: open form file %q: %w", key, err)
	}
	defer reader.Close()
	disposition := mime.FormatMediaType("form-data", map[string]string{"name": key, "filename": file.Name()})
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", disposition)
	header.Set("Content-Type", file.ContentType())
	part, err := writer.CreatePart(header)
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, reader); err != nil {
		return err
	}
	return nil
}

type requestBodyFactory struct{}

var RequestBody requestBodyFactory

func (requestBodyFactory) Empty() RequestBodyRaw             { return RequestBodyRaw{} }
func (requestBodyFactory) Raw(content string) RequestBodyRaw { return RequestBodyRaw{Content: content} }
func (requestBodyFactory) RawWithType(content, contentType string) RequestBodyRaw {
	return RequestBodyRaw{Content: content, ContentTypeValue: contentType}
}
func (requestBodyFactory) Query(values map[string]string) RequestBodyQuery {
	return RequestBodyQuery{Values: values}
}
func (requestBodyFactory) FormData(values FormData) RequestBodyFormData {
	return RequestBodyFormData{Values: values}
}
func (requestBodyFactory) JSON(value any) RequestBodyJSON { return RequestBodyJSON{Value: value} }

type ExpectedBodyRule interface {
	MatchBody(body []byte) BodyMatchResult
}

// ExpectedHTTPBodyRule optionally matches with response metadata available.
type ExpectedHTTPBodyRule interface {
	ExpectedBodyRule
	MatchHTTPBody(contentType, disposition string, body []byte) BodyMatchResult
}

type ExpectedBodyEqual struct {
	Content any `json:"content"`
}
type ExpectedBodyNotEqual struct {
	Content string `json:"content"`
}
type ExpectedBodyContains struct {
	Patterns []string `json:"patterns"`
}
type ExpectedBodyNotContains struct {
	Patterns []string `json:"patterns"`
}
type ExpectedBodySnapshot struct {
	File string `json:"file"`
	// Content is filled by the executor after reading File.
	Content string `json:"-"`
	Error   string `json:"-"`
}
type ExpectedBodyJSONEqual struct {
	Value any `json:"value,omitempty"`
}
type ExpectedBodyJSONNotEqual struct {
	Value any `json:"value,omitempty"`
}
type ExpectedBodyJSONContains struct {
	Value any `json:"value,omitempty"`
}
type ExpectedBodyJSONNotContains struct {
	Value any `json:"value,omitempty"`
}
type ExpectedBodyCustom struct {
	Rules []ExpectedBodyRule `json:"-"`
}

func (r ExpectedBodyEqual) MatchBody(body []byte) BodyMatchResult {
	if file, ok := r.Content.(gox.File); ok {
		return matchFileBody(file, "", "", body)
	}
	if files, ok := r.Content.([]gox.File); ok {
		return BodyMatchResult{Rule: "equal", Expected: fileMetadataList(files), Response: string(body), Match: false, Message: "multipart response metadata is required to compare files"}
	}
	expected, ok := r.Content.(string)
	if !ok {
		return BodyMatchResult{Rule: "equal", Expected: r.Content, Response: string(body), Match: false, Message: fmt.Sprintf("unsupported body expectation type %T", r.Content)}
	}
	actual := string(body)
	match := actual == expected
	result := BodyMatchResult{Rule: "equal", Expected: expected, Response: actual, Match: match}
	if !match {
		result.Diff = firstBodyDiff(expected, actual)
	}
	return result
}

func (r ExpectedBodyEqual) MatchHTTPBody(contentType, disposition string, body []byte) BodyMatchResult {
	switch expected := r.Content.(type) {
	case gox.File:
		return matchFileBody(expected, contentType, disposition, body)
	case []gox.File:
		actual, err := decodeMultipartFiles(contentType, body)
		if err != nil {
			return BodyMatchResult{Rule: "equal", Expected: fileMetadataList(expected), Match: false, Message: err.Error()}
		}
		match, message := filesEqual(expected, actual)
		return BodyMatchResult{Rule: "equal", Expected: fileMetadataList(expected), Response: fileMetadataList(actual), Match: match, Message: message}
	default:
		return r.MatchBody(body)
	}
}

func (r ExpectedBodyNotEqual) MatchBody(body []byte) BodyMatchResult {
	actual := string(body)
	match := actual != r.Content
	result := BodyMatchResult{Rule: "notEqual", Expected: r.Content, Response: actual, Match: match}
	if !match {
		result.Message = "response equals forbidden body"
	}
	return result
}

func (r ExpectedBodyContains) MatchBody(body []byte) BodyMatchResult {
	actual := string(body)
	missing := make([]string, 0)
	findings := make([]BodyMatchFinding, 0, len(r.Patterns))
	for _, pattern := range r.Patterns {
		finding := findBodyPattern(actual, pattern)
		findings = append(findings, finding)
		if !finding.Found {
			missing = append(missing, pattern)
		}
	}
	match := len(missing) == 0
	message := ""
	if !match {
		message = "missing patterns: " + strings.Join(missing, ", ")
	}
	return BodyMatchResult{Rule: "contains", Expected: r.Patterns, Response: actual, Match: match, Message: message, Findings: findings}
}

func (r ExpectedBodyNotContains) MatchBody(body []byte) BodyMatchResult {
	actual := string(body)
	found := make([]string, 0)
	findings := make([]BodyMatchFinding, 0, len(r.Patterns))
	for _, pattern := range r.Patterns {
		finding := findBodyPattern(actual, pattern)
		findings = append(findings, finding)
		if finding.Found {
			found = append(found, pattern)
		}
	}
	match := len(found) == 0
	message := ""
	if !match {
		message = "found forbidden patterns: " + strings.Join(found, ", ")
	}
	return BodyMatchResult{Rule: "notContains", Expected: r.Patterns, Response: actual, Match: match, Message: message, Findings: findings}
}

func (r ExpectedBodySnapshot) MatchBody(body []byte) BodyMatchResult {
	if r.Error != "" {
		return BodyMatchResult{Rule: "snapshot", Match: false, Message: r.Error}
	}
	actual := string(body)
	match := actual == r.Content
	result := BodyMatchResult{Rule: "snapshot", Expected: r.Content, Response: actual, Match: match}
	if !match {
		result.Diff = firstBodyDiff(r.Content, actual)
	}
	return result
}

func (r ExpectedBodyJSONEqual) MatchBody(body []byte) BodyMatchResult {
	actual, result, ok := decodeJSONBody("jsonEqual", body, r.Value)
	if !ok {
		return result
	}
	return BodyMatchResult{Rule: "jsonEqual", Expected: r.Value, Response: actual, Match: valuesEqual(actual, r.Value)}
}

func (r ExpectedBodyJSONNotEqual) MatchBody(body []byte) BodyMatchResult {
	actual, result, ok := decodeJSONBody("jsonNotEqual", body, r.Value)
	if !ok {
		return result
	}
	return BodyMatchResult{Rule: "jsonNotEqual", Expected: r.Value, Response: actual, Match: !valuesEqual(actual, r.Value)}
}

func (r ExpectedBodyJSONContains) MatchBody(body []byte) BodyMatchResult {
	actual, result, ok := decodeJSONBody("jsonContains", body, r.Value)
	if !ok {
		return result
	}
	findings := jsonContainsFindings(actual, r.Value)
	match := jsonFindingsMatch(findings)
	message := ""
	if !match {
		message = "missing expected JSON subset"
	}
	return BodyMatchResult{Rule: "jsonContains", Expected: r.Value, Response: actual, Match: match, Message: message, JSONFindings: findings}
}

func (r ExpectedBodyJSONNotContains) MatchBody(body []byte) BodyMatchResult {
	actual, result, ok := decodeJSONBody("jsonNotContains", body, r.Value)
	if !ok {
		return result
	}
	contained := jsonContains(actual, r.Value)
	findings := []JSONMatchFinding(nil)
	message := ""
	if contained {
		findings = jsonForbiddenFindings(actual, r.Value)
		message = "found forbidden JSON subset"
	}
	return BodyMatchResult{Rule: "jsonNotContains", Expected: r.Value, Response: actual, Match: !contained, Message: message, JSONFindings: findings}
}

func (r ExpectedBodyCustom) MatchBody(body []byte) BodyMatchResult {
	results := make([]BodyMatchResult, 0, len(r.Rules))
	match := true
	for _, rule := range r.Rules {
		if rule == nil {
			result := BodyMatchResult{Rule: "custom", Match: false, Message: "custom rule is nil"}
			results = append(results, result)
			match = false
			continue
		}
		result := rule.MatchBody(body)
		results = append(results, result)
		if !result.Match {
			match = false
		}
	}
	return BodyMatchResult{Rule: "custom", Response: string(body), Match: match, Rules: results}
}

func decodeJSONBody(rule string, body []byte, expected any) (any, BodyMatchResult, bool) {
	var actual any
	if err := json.Unmarshal(body, &actual); err != nil {
		return nil, BodyMatchResult{Rule: rule, Expected: expected, Response: string(body), Match: false, Message: err.Error()}, false
	}
	return actual, BodyMatchResult{}, true
}

func findBodyPattern(body string, pattern string) BodyMatchFinding {
	finding := BodyMatchFinding{Pattern: pattern}
	if pattern == "" {
		finding.Found = true
		finding.Line = 1
		finding.Column = 1
		finding.Snippet = bodySnippet(body, 0, 0)
		return finding
	}
	offset := strings.Index(body, pattern)
	if offset < 0 {
		return finding
	}
	line, column := bodyLineColumn(body, offset)
	finding.Found = true
	finding.Offset = offset
	finding.Line = line
	finding.Column = column
	finding.Snippet = bodySnippet(body, offset, len(pattern))
	return finding
}

func firstBodyDiff(expected string, response string) *BodyMatchDiff {
	limit := len(expected)
	if len(response) < limit {
		limit = len(response)
	}
	offset := 0
	for offset < limit && expected[offset] == response[offset] {
		offset++
	}
	line, column := bodyLineColumn(response, offset)
	return &BodyMatchDiff{
		Line:            line,
		Column:          column,
		Offset:          offset,
		ExpectedSnippet: bodySnippet(expected, offset, 1),
		ResponseSnippet: bodySnippet(response, offset, 1),
	}
}

func bodyLineColumn(text string, offset int) (int, int) {
	if offset < 0 {
		offset = 0
	}
	if offset > len(text) {
		offset = len(text)
	}
	line := 1
	column := 1
	for i, r := range text {
		if i >= offset {
			break
		}
		if r == '\n' {
			line++
			column = 1
		} else {
			column++
		}
	}
	return line, column
}

func bodySnippet(text string, offset int, matchLen int) string {
	if text == "" {
		return ""
	}
	if offset < 0 {
		offset = 0
	}
	if offset > len(text) {
		offset = len(text)
	}
	start := offset - 80
	if start < 0 {
		start = 0
	}
	end := offset + matchLen + 80
	if end < offset+1 {
		end = offset + 1
	}
	if end > len(text) {
		end = len(text)
	}
	snippet := strings.TrimSpace(text[start:end])
	snippet = strings.ReplaceAll(snippet, "\n", "\n")
	if start > 0 {
		snippet = "…" + snippet
	}
	if end < len(text) {
		snippet += "…"
	}
	return snippet
}

func jsonFindingsMatch(findings []JSONMatchFinding) bool {
	for _, finding := range findings {
		if !finding.Match {
			return false
		}
	}
	return true
}

func jsonContainsFindings(actual any, expected any) []JSONMatchFinding {
	findings := make([]JSONMatchFinding, 0)
	collectJSONContainsFindings(&findings, normalizeJSONValue(actual), normalizeJSONValue(expected), "")
	return findings
}

func collectJSONContainsFindings(findings *[]JSONMatchFinding, actual any, expected any, path string) bool {
	switch exp := expected.(type) {
	case map[string]any:
		act, ok := actual.(map[string]any)
		if !ok {
			*findings = append(*findings, JSONMatchFinding{Path: displayJSONPath(path), Expected: exp, Response: actual, Found: false, Match: false, Message: "response node is not an object"})
			return false
		}
		match := true
		for key, expectedValue := range exp {
			childPath := joinJSONPath(path, key)
			actualValue, exists := act[key]
			if !exists {
				*findings = append(*findings, JSONMatchFinding{Path: childPath, Expected: expectedValue, Found: false, Match: false, Message: "path not found"})
				match = false
				continue
			}
			if !collectJSONContainsFindings(findings, actualValue, expectedValue, childPath) {
				match = false
			}
		}
		return match
	case []any:
		act, ok := actual.([]any)
		if !ok {
			*findings = append(*findings, JSONMatchFinding{Path: displayJSONPath(path), Expected: exp, Response: actual, Found: false, Match: false, Message: "response node is not an array"})
			return false
		}
		match := true
		for i, expectedItem := range exp {
			itemFound := false
			for j, actualItem := range act {
				localFindings := make([]JSONMatchFinding, 0)
				if collectJSONContainsFindings(&localFindings, actualItem, expectedItem, fmt.Sprintf("%s[%d]", displayJSONPath(path), j)) {
					itemFound = true
					break
				}
			}
			if !itemFound {
				*findings = append(*findings, JSONMatchFinding{Path: fmt.Sprintf("%s[%d]", displayJSONPath(path), i), Expected: expectedItem, Found: false, Match: false, Message: "array item not found"})
				match = false
			}
		}
		return match
	default:
		match := valuesEqual(actual, expected)
		*findings = append(*findings, JSONMatchFinding{Path: displayJSONPath(path), Expected: expected, Response: actual, Found: true, Match: match})
		return match
	}
}

func jsonForbiddenFindings(actual any, expected any) []JSONMatchFinding {
	findings := make([]JSONMatchFinding, 0)
	collectJSONForbiddenFindings(&findings, normalizeJSONValue(actual), normalizeJSONValue(expected), "")
	return findings
}

func collectJSONForbiddenFindings(findings *[]JSONMatchFinding, actual any, expected any, path string) {
	switch exp := expected.(type) {
	case map[string]any:
		act, ok := actual.(map[string]any)
		if !ok {
			return
		}
		for key, expectedValue := range exp {
			actualValue, exists := act[key]
			if !exists {
				continue
			}
			collectJSONForbiddenFindings(findings, actualValue, expectedValue, joinJSONPath(path, key))
		}
	case []any:
		act, ok := actual.([]any)
		if !ok {
			return
		}
		for _, expectedItem := range exp {
			for j, actualItem := range act {
				if jsonContains(actualItem, expectedItem) {
					collectJSONForbiddenFindings(findings, actualItem, expectedItem, fmt.Sprintf("%s[%d]", displayJSONPath(path), j))
					break
				}
			}
		}
	default:
		if valuesEqual(actual, expected) {
			*findings = append(*findings, JSONMatchFinding{Path: displayJSONPath(path), Expected: expected, Response: actual, Found: true, Match: false, Message: "forbidden JSON value found"})
		}
	}
}

func joinJSONPath(parent string, key string) string {
	if parent == "" {
		return key
	}
	return parent + "." + key
}

func displayJSONPath(path string) string {
	if path == "" {
		return "$"
	}
	return path
}

type expectedBodyFactory struct{}

var ExpectedBody expectedBodyFactory

func (expectedBodyFactory) Equal(content any) ExpectedBodyEqual {
	return ExpectedBodyEqual{Content: content}
}
func (expectedBodyFactory) NotEqual(content string) ExpectedBodyNotEqual {
	return ExpectedBodyNotEqual{Content: content}
}
func (expectedBodyFactory) Contains(patterns ...string) ExpectedBodyContains {
	return ExpectedBodyContains{Patterns: patterns}
}
func (expectedBodyFactory) NotContains(patterns ...string) ExpectedBodyNotContains {
	return ExpectedBodyNotContains{Patterns: patterns}
}
func (expectedBodyFactory) Snapshot(file string) ExpectedBodySnapshot {
	return ExpectedBodySnapshot{File: file}
}
func (expectedBodyFactory) JSONEqual(value any) ExpectedBodyJSONEqual {
	return ExpectedBodyJSONEqual{Value: value}
}
func (expectedBodyFactory) JSONNotEqual(value any) ExpectedBodyJSONNotEqual {
	return ExpectedBodyJSONNotEqual{Value: value}
}
func (expectedBodyFactory) JSONContains(value any) ExpectedBodyJSONContains {
	return ExpectedBodyJSONContains{Value: value}
}
func (expectedBodyFactory) JSONNotContains(value any) ExpectedBodyJSONNotContains {
	return ExpectedBodyJSONNotContains{Value: value}
}

func (expectedBodyFactory) Custom(rules ...ExpectedBodyRule) ExpectedBodyCustom {
	return ExpectedBodyCustom{Rules: rules}
}
