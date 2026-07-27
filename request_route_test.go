package gox

import "testing"

func TestRouteClientAssetImpliesStatic(t *testing.T) {
	route := NewRoute(RouteInput{IsClientAsset: true})

	if !route.IsClientAsset() {
		t.Fatal("expected route to be marked as client asset")
	}
	if !route.IsStatic() {
		t.Fatal("client asset route must also be static")
	}
}

func TestRouteStaticDoesNotImplyClientAsset(t *testing.T) {
	route := NewRoute(RouteInput{IsStatic: true})

	if !route.IsStatic() {
		t.Fatal("expected static route")
	}
	if route.IsClientAsset() {
		t.Fatal("static route should not imply client asset")
	}
}

func TestURLStringUsesExplicitValue(t *testing.T) {
	url := NewURL(URLInput{
		String:      "https://cdn.example.com/explicit?q=1",
		Scheme:      "http",
		Host:        "ignored.example.com",
		Path:        "/ignored",
		QueryString: "?ignored=1",
	})

	if got := url.String(); got != "https://cdn.example.com/explicit?q=1" {
		t.Fatalf("String() = %q", got)
	}
}

func TestURLStringBuildsAbsoluteValueLazily(t *testing.T) {
	url := NewURL(URLInput{
		Scheme:      "https",
		Host:        "example.com:8443",
		Hostname:    "example.com",
		Port:        "8443",
		Path:        "/hello world",
		RawPath:     "/hello%20world",
		QueryString: "?page=1",
	})

	if got := url.String(); got != "https://example.com:8443/hello%20world?page=1" {
		t.Fatalf("String() = %q", got)
	}
}

func TestURLStringBuildsRelativeValueWithoutSchemeOrHost(t *testing.T) {
	url := NewURL(URLInput{Path: "/health", QueryString: "?deep=true"})

	if got := url.String(); got != "/health?deep=true" {
		t.Fatalf("String() = %q", got)
	}
}
