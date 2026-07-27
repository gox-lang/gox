package test

import "strings"

// ExpectedStringRule is used by Custom header and cookie matchers.
type ExpectedStringRule interface {
	MatchString(response string, exists bool) MatchValue
}

type ExpectedStringEqual struct{ Value string }
type ExpectedStringNotEqual struct{ Value string }
type ExpectedStringContains struct{ Value string }
type ExpectedStringNotContains struct{ Value string }
type ExpectedStringExists struct{}
type ExpectedStringNotExists struct{}

func (r ExpectedStringEqual) MatchString(response string, exists bool) MatchValue {
	match := exists && response == r.Value
	return MatchValue{Rule: "equal", Expected: r.Value, Response: response, Match: match}
}

func (r ExpectedStringNotEqual) MatchString(response string, exists bool) MatchValue {
	match := !exists || response != r.Value
	return MatchValue{Rule: "notEqual", Expected: r.Value, Response: response, Match: match}
}

func (r ExpectedStringContains) MatchString(response string, exists bool) MatchValue {
	match := exists && strings.Contains(response, r.Value)
	return MatchValue{Rule: "contains", Expected: r.Value, Response: response, Match: match}
}

func (r ExpectedStringNotContains) MatchString(response string, exists bool) MatchValue {
	match := !exists || !strings.Contains(response, r.Value)
	return MatchValue{Rule: "notContains", Expected: r.Value, Response: response, Match: match}
}

func (ExpectedStringExists) MatchString(response string, exists bool) MatchValue {
	return MatchValue{Rule: "exists", Expected: true, Response: response, Match: exists}
}

func (ExpectedStringNotExists) MatchString(response string, exists bool) MatchValue {
	return MatchValue{Rule: "notExists", Expected: false, Response: response, Match: !exists}
}

type expectFactory struct{}

// Expect creates field/body rules used by Custom matchers.
var Expect expectFactory

func (expectFactory) Equal(value string) ExpectedStringEqual {
	return ExpectedStringEqual{Value: value}
}
func (expectFactory) NotEqual(value string) ExpectedStringNotEqual {
	return ExpectedStringNotEqual{Value: value}
}
func (expectFactory) Contains(value string) ExpectedStringContains {
	return ExpectedStringContains{Value: value}
}
func (expectFactory) NotContains(value string) ExpectedStringNotContains {
	return ExpectedStringNotContains{Value: value}
}
func (expectFactory) Exists() ExpectedStringExists       { return ExpectedStringExists{} }
func (expectFactory) NotExists() ExpectedStringNotExists { return ExpectedStringNotExists{} }

func (expectFactory) JSONEqual(value any) ExpectedBodyJSONEqual {
	return ExpectedBodyJSONEqual{Value: value}
}
func (expectFactory) JSONNotEqual(value any) ExpectedBodyJSONNotEqual {
	return ExpectedBodyJSONNotEqual{Value: value}
}
func (expectFactory) JSONContains(value any) ExpectedBodyJSONContains {
	return ExpectedBodyJSONContains{Value: value}
}
func (expectFactory) JSONNotContains(value any) ExpectedBodyJSONNotContains {
	return ExpectedBodyJSONNotContains{Value: value}
}

func (r ExpectedStringEqual) MatchBody(body []byte) BodyMatchResult {
	return ExpectedBodyEqual{Content: r.Value}.MatchBody(body)
}

func (r ExpectedStringNotEqual) MatchBody(body []byte) BodyMatchResult {
	return ExpectedBodyNotEqual{Content: r.Value}.MatchBody(body)
}

func (r ExpectedStringContains) MatchBody(body []byte) BodyMatchResult {
	return ExpectedBodyContains{Patterns: []string{r.Value}}.MatchBody(body)
}

func (r ExpectedStringNotContains) MatchBody(body []byte) BodyMatchResult {
	return ExpectedBodyNotContains{Patterns: []string{r.Value}}.MatchBody(body)
}
