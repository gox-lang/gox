package gox

import (
	"bytes"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"strings"
	"testing"
)

func TestWritePageFilesWritesOrderedMultipartResponse(t *testing.T) {
	first, err := NewFile(FileConfig{Name: "first.txt", Type: "text/plain", Content: strings.NewReader("first")})
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewFile(FileConfig{Name: "second.json", Type: "application/json", Content: strings.NewReader(`{"second":true}`)})
	if err != nil {
		t.Fatal(err)
	}

	response := NewResponse()
	writer := &pageFileTestWriter{}
	ctx := &Context{Response: response, Writer: writer}
	if err := WritePageFiles(ctx, []File{first, second}); err != nil {
		t.Fatal(err)
	}

	mediaType, params, err := mimeParseMediaType(responseHeaderValue(&response.Headers, "Content-Type"))
	if err != nil {
		t.Fatal(err)
	}
	if mediaType != "multipart/form-data" {
		t.Fatalf("Content-Type = %q", mediaType)
	}

	reader := multipart.NewReader(bytes.NewReader(writer.body.Bytes()), params["boundary"])
	want := []struct {
		name     string
		typeName string
		body     string
	}{
		{name: "first.txt", typeName: "text/plain", body: "first"},
		{name: "second.json", typeName: "application/json", body: `{"second":true}`},
	}
	for index, expected := range want {
		part, err := reader.NextPart()
		if err != nil {
			t.Fatalf("part %d: %v", index, err)
		}
		if got := part.FormName(); got != "files" {
			t.Fatalf("part %d form name = %q", index, got)
		}
		if got := part.FileName(); got != expected.name {
			t.Fatalf("part %d filename = %q", index, got)
		}
		if got := part.Header.Get("Content-Type"); got != expected.typeName {
			t.Fatalf("part %d Content-Type = %q", index, got)
		}
		body, err := io.ReadAll(part)
		if err != nil {
			t.Fatal(err)
		}
		if got := string(body); got != expected.body {
			t.Fatalf("part %d body = %q", index, got)
		}
	}
	if _, err := reader.NextPart(); !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF, got %v", err)
	}
}

func TestWritePageFilesSupportsEmptySlice(t *testing.T) {
	response := NewResponse()
	writer := &pageFileTestWriter{}
	ctx := &Context{Response: response, Writer: writer}
	if err := WritePageFiles(ctx, nil); err != nil {
		t.Fatal(err)
	}
	mediaType, params, err := mimeParseMediaType(responseHeaderValue(&response.Headers, "Content-Type"))
	if err != nil {
		t.Fatal(err)
	}
	if mediaType != "multipart/form-data" {
		t.Fatalf("Content-Type = %q", mediaType)
	}
	reader := multipart.NewReader(bytes.NewReader(writer.body.Bytes()), params["boundary"])
	if _, err := reader.NextPart(); !errors.Is(err, io.EOF) {
		t.Fatalf("expected empty multipart body, got %v", err)
	}
}

func TestWritePageFilesOpensAllFilesBeforeMutatingResponse(t *testing.T) {
	first, err := NewFile(FileConfig{Name: "first.txt", Content: strings.NewReader("first")})
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewFile(FileConfig{Name: "gone.txt", Content: strings.NewReader("gone")})
	if err != nil {
		t.Fatal(err)
	}
	second.state.mu.Lock()
	second.state.open = func() (io.ReadCloser, error) { return nil, errors.New("open failed") }
	second.state.mu.Unlock()

	response := NewResponse()
	writer := &pageFileTestWriter{}
	ctx := &Context{Response: response, Writer: writer}
	if err := WritePageFiles(ctx, []File{first, second}); err == nil {
		t.Fatal("expected open error")
	}
	if !response.Headers.IsEmpty() {
		t.Fatalf("headers mutated before all files opened: %#v", response.Headers.Values())
	}
	if writer.body.Len() != 0 {
		t.Fatalf("unexpected body: %q", writer.body.String())
	}
}

// Kept as a tiny wrapper so tests remain explicit about the standard parser.
func mimeParseMediaType(value string) (string, map[string]string, error) {
	return mime.ParseMediaType(value)
}
