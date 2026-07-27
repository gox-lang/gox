package test

import "strings"

type ExpectedHeadersRule interface {
	MatchHeaders(headers map[string][]string) HeaderMatchResult
}

type ExpectedHeadersEqual Map
type ExpectedHeadersContains Map
type ExpectedHeadersNotContains Map
type ExpectedHeadersCustom map[string]ExpectedStringRule

func (r ExpectedHeadersEqual) MatchHeaders(headers map[string][]string) HeaderMatchResult {
	return matchStringMap("equal", Map(r), flattenHeaders(headers), true, false)
}

func (r ExpectedHeadersContains) MatchHeaders(headers map[string][]string) HeaderMatchResult {
	return matchStringMap("contains", Map(r), flattenHeaders(headers), false, false)
}

func (r ExpectedHeadersNotContains) MatchHeaders(headers map[string][]string) HeaderMatchResult {
	return matchStringMap("notContains", Map(r), flattenHeaders(headers), false, true)
}

func (r ExpectedHeadersCustom) MatchHeaders(headers map[string][]string) HeaderMatchResult {
	return matchStringCustomMap("custom", map[string]ExpectedStringRule(r), flattenHeaders(headers))
}

type expectedHeadersFactory struct{}

var ExpectedHeaders expectedHeadersFactory

func (expectedHeadersFactory) Equal(values Map) ExpectedHeadersEqual {
	return ExpectedHeadersEqual(values)
}

func (expectedHeadersFactory) Contains(values Map) ExpectedHeadersContains {
	return ExpectedHeadersContains(values)
}

func (expectedHeadersFactory) NotContains(values Map) ExpectedHeadersNotContains {
	return ExpectedHeadersNotContains(values)
}

func (expectedHeadersFactory) Custom(values map[string]ExpectedStringRule) ExpectedHeadersCustom {
	return ExpectedHeadersCustom(values)
}

func flattenHeaders(headers map[string][]string) map[string]string {
	out := make(map[string]string, len(headers))
	for k, values := range headers {
		if canonicalKey(k) == "set-cookie" {
			continue
		}
		out[canonicalKey(k)] = strings.Join(values, ", ")
	}
	return out
}

func canonicalKey(key string) string {
	return strings.ToLower(strings.TrimSpace(key))
}
