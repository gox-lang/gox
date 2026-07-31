package gox

func responseHeaderValue(headers *ResponseHeaders, key string) string {
	value, _ := headers.Get(key)
	return value
}

func responseHeaderValues(headers *ResponseHeaders, key string) []string {
	values, _ := headers.GetAll(key)
	return values
}

func responseCookieValue(cookies *ResponseCookies, key string) Cookie {
	value, _ := cookies.Get(key)
	return value
}

func responseCookieValues(cookies *ResponseCookies, key string) []Cookie {
	values, _ := cookies.GetAll(key)
	return values
}
