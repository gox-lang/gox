package test

import "net/http"

type ExpectedCookiesRule interface {
	MatchCookies(cookies []*http.Cookie) HeaderMatchResult
}

type ExpectedCookiesEqual Map
type ExpectedCookiesContains Map
type ExpectedCookiesNotContains Map
type ExpectedCookiesCustom map[string]ExpectedStringRule

func (r ExpectedCookiesEqual) MatchCookies(cookies []*http.Cookie) HeaderMatchResult {
	return matchStringMap("equal", Map(r), flattenCookies(cookies), true, false)
}

func (r ExpectedCookiesContains) MatchCookies(cookies []*http.Cookie) HeaderMatchResult {
	return matchStringMap("contains", Map(r), flattenCookies(cookies), false, false)
}

func (r ExpectedCookiesNotContains) MatchCookies(cookies []*http.Cookie) HeaderMatchResult {
	return matchStringMap("notContains", Map(r), flattenCookies(cookies), false, true)
}

func (r ExpectedCookiesCustom) MatchCookies(cookies []*http.Cookie) HeaderMatchResult {
	return matchStringCustomMap("custom", map[string]ExpectedStringRule(r), flattenCookies(cookies))
}

type expectedCookiesFactory struct{}

var ExpectedCookies expectedCookiesFactory

func (expectedCookiesFactory) Equal(values Map) ExpectedCookiesEqual {
	return ExpectedCookiesEqual(values)
}

func (expectedCookiesFactory) Contains(values Map) ExpectedCookiesContains {
	return ExpectedCookiesContains(values)
}

func (expectedCookiesFactory) NotContains(values Map) ExpectedCookiesNotContains {
	return ExpectedCookiesNotContains(values)
}

func (expectedCookiesFactory) Custom(values map[string]ExpectedStringRule) ExpectedCookiesCustom {
	return ExpectedCookiesCustom(values)
}

func flattenCookies(cookies []*http.Cookie) map[string]string {
	out := make(map[string]string, len(cookies))
	for _, cookie := range cookies {
		if cookie == nil || cookie.Name == "" {
			continue
		}
		out[canonicalKey(cookie.Name)] = cookie.Value
	}
	return out
}
