package gox

import (
	"io"
	"strings"
	"testing"
)

func TestResetRuntimeContextClearsRequestStateAndRetainsSmallAssetCapacity(t *testing.T) {
	ctx := &Context{
		runtimeState:         "runtime",
		clientRuntimeWritten: true,
		Request:              NewRequest(RequestInput{Method: "POST"}),
		Response:             NewResponse(),
	}
	ctx.clientAssetsSmall = make([]uint64, 1, 4)
	ctx.clientAssetsSmall[0] = 42

	ResetRuntimeContext(ctx)

	if ctx.runtimeState != nil || ctx.Response != nil {
		t.Fatal("runtime context state survived reset")
	}
	if ctx.Request.Method() != "" || ctx.clientRuntimeWritten {
		t.Fatal("public or client runtime state survived reset")
	}
	if len(ctx.clientAssetsSmall) != 0 || cap(ctx.clientAssetsSmall) != 4 {
		t.Fatalf("small client asset storage was not retained: len=%d cap=%d", len(ctx.clientAssetsSmall), cap(ctx.clientAssetsSmall))
	}
}

func TestResetRuntimeResponseClearsState(t *testing.T) {
	response := NewResponse()
	response.Status.SetCode(201)
	response.Headers.Set("Content-Type", "application/json")
	response.Headers.Set("X-Test", "old")
	response.Cookies.Set(Cookie{Name: "token", Value: "old"})
	response.ImportMap.Set("old", "/old.js")
	SetRuntimeResponseState(response, "old")

	ResetRuntimeResponse(response)

	if response.Status.GetCode() != 200 {
		t.Fatalf("expected reset status 200, got %d", response.Status.GetCode())
	}
	if !response.Headers.IsEmpty() || !response.Cookies.IsEmpty() || response.ImportMap.Len() != 0 {
		t.Fatalf("response state survived reset")
	}
	if RuntimeResponseState(response) != nil {
		t.Fatalf("runtime state survived reset")
	}
}

func TestResetRuntimeBodyReusesBody(t *testing.T) {
	value := NewBody(nil, "")
	if !ResetRuntimeBody(value, io.NopCloser(strings.NewReader("hello")), "text/plain") {
		t.Fatal("expected body to be reusable")
	}
	text, err := value.ToText()
	if err != nil {
		t.Fatal(err)
	}
	if text != "hello" {
		t.Fatalf("unexpected body text %q", text)
	}
	if !ResetRuntimeBody(value, io.NopCloser(strings.NewReader("next")), "text/plain") {
		t.Fatal("expected second body reset to succeed")
	}
	text, err = value.ToText()
	if err != nil {
		t.Fatal(err)
	}
	if text != "next" {
		t.Fatalf("body data survived reset: %q", text)
	}
}
