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

func TestWriteActionFileWritesMetadataAndContent(t *testing.T) {
	file, err := NewFile(FileConfig{
		Name:    "report.pdf",
		Type:    "application/pdf",
		Content: strings.NewReader("pdf-data"),
	})
	if err != nil {
		t.Fatal(err)
	}

	response := NewResponse()
	writer := &pageFileTestWriter{}
	ctx := &Context{Response: response, Writer: writer}
	if err := WriteActionFile(ctx, file); err != nil {
		t.Fatal(err)
	}

	checks := map[string]string{
		"Action-Response":     "success",
		"Action-Type":         "file",
		"Content-Type":        "application/pdf",
		"Content-Disposition": `attachment; filename=report.pdf`,
		"Content-Length":      "8",
	}
	for name, expected := range checks {
		if got := responseHeaderValue(&response.Headers, name); got != expected {
			t.Fatalf("%s = %q, want %q", name, got, expected)
		}
	}
	if got := writer.body.String(); got != "pdf-data" {
		t.Fatalf("body = %q", got)
	}
}

func TestWriteActionFileOpensBeforeMutatingHeaders(t *testing.T) {
	file, err := NewFile(FileConfig{Name: "gone.txt", Content: strings.NewReader("gone")})
	if err != nil {
		t.Fatal(err)
	}
	file.state.mu.Lock()
	file.state.open = func() (io.ReadCloser, error) { return nil, errors.New("open failed") }
	file.state.mu.Unlock()

	response := NewResponse()
	writer := &pageFileTestWriter{}
	ctx := &Context{Response: response, Writer: writer}
	if err := WriteActionFile(ctx, file); err == nil {
		t.Fatal("expected open error")
	}
	if !response.Headers.IsEmpty() {
		t.Fatalf("headers mutated before open: %#v", response.Headers.Values())
	}
	if writer.body.Len() != 0 {
		t.Fatalf("unexpected body: %q", writer.body.String())
	}
}

func TestWriteActionFilesWritesOrderedMultipartResponse(t *testing.T) {
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
	if err := WriteActionFiles(ctx, []File{first, second}); err != nil {
		t.Fatal(err)
	}

	if got := responseHeaderValue(&response.Headers, "Action-Response"); got != "success" {
		t.Fatalf("Action-Response = %q", got)
	}
	if got := responseHeaderValue(&response.Headers, "Action-Type"); got != "files" {
		t.Fatalf("Action-Type = %q", got)
	}
	mediaType, params, err := mime.ParseMediaType(responseHeaderValue(&response.Headers, "Content-Type"))
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

func TestWriteActionFilesSupportsEmptySlice(t *testing.T) {
	response := NewResponse()
	writer := &pageFileTestWriter{}
	ctx := &Context{Response: response, Writer: writer}
	if err := WriteActionFiles(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if got := responseHeaderValue(&response.Headers, "Action-Response"); got != "success" {
		t.Fatalf("Action-Response = %q", got)
	}
	if got := responseHeaderValue(&response.Headers, "Action-Type"); got != "files" {
		t.Fatalf("Action-Type = %q", got)
	}
	mediaType, params, err := mime.ParseMediaType(responseHeaderValue(&response.Headers, "Content-Type"))
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

func TestWriteActionFilesOpensAllFilesBeforeMutatingResponse(t *testing.T) {
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
	if err := WriteActionFiles(ctx, []File{first, second}); err == nil {
		t.Fatal("expected open error")
	}
	if !response.Headers.IsEmpty() {
		t.Fatalf("headers mutated before all files opened: %#v", response.Headers.Values())
	}
	if writer.body.Len() != 0 {
		t.Fatalf("unexpected body: %q", writer.body.String())
	}
}

func TestWriteActionFilesRejectsZeroFileBeforeMutatingResponse(t *testing.T) {
	response := NewResponse()
	writer := &pageFileTestWriter{}
	ctx := &Context{Response: response, Writer: writer}
	if err := WriteActionFiles(ctx, []File{{}}); err == nil {
		t.Fatal("expected zero file error")
	}
	if !response.Headers.IsEmpty() {
		t.Fatalf("headers mutated: %#v", response.Headers.Values())
	}
	if writer.body.Len() != 0 {
		t.Fatalf("unexpected body: %q", writer.body.String())
	}
}

type actionFileTrackedReader struct {
	*strings.Reader
	closed *int
}

func (reader *actionFileTrackedReader) Close() error {
	*reader.closed++
	return nil
}

type actionFileFailWriter struct {
	Writer
	err error
}

func (writer *actionFileFailWriter) Write([]byte) (int, error) {
	return 0, writer.err
}

func TestWriteActionFileReturnsWriteErrorAndClosesReader(t *testing.T) {
	file, err := NewFile(FileConfig{Name: "file.txt", Content: strings.NewReader("content")})
	if err != nil {
		t.Fatal(err)
	}
	closed := 0
	file.state.mu.Lock()
	file.state.open = func() (io.ReadCloser, error) {
		return &actionFileTrackedReader{Reader: strings.NewReader("content"), closed: &closed}, nil
	}
	file.state.mu.Unlock()

	writeErr := errors.New("write failed")
	ctx := &Context{Response: NewResponse(), Writer: &actionFileFailWriter{err: writeErr}}
	err = WriteActionFile(ctx, file)
	if !errors.Is(err, writeErr) {
		t.Fatalf("error = %v", err)
	}
	if closed != 1 {
		t.Fatalf("reader closed %d times", closed)
	}
}

func TestWriteActionFilesClosesEveryOpenedReader(t *testing.T) {
	first, err := NewFile(FileConfig{Name: "first.txt", Content: strings.NewReader("first")})
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewFile(FileConfig{Name: "second.txt", Content: strings.NewReader("second")})
	if err != nil {
		t.Fatal(err)
	}
	firstClosed := 0
	secondClosed := 0
	first.state.mu.Lock()
	first.state.open = func() (io.ReadCloser, error) {
		return &actionFileTrackedReader{Reader: strings.NewReader("first"), closed: &firstClosed}, nil
	}
	first.state.mu.Unlock()
	second.state.mu.Lock()
	second.state.open = func() (io.ReadCloser, error) {
		return &actionFileTrackedReader{Reader: strings.NewReader("second"), closed: &secondClosed}, nil
	}
	second.state.mu.Unlock()

	ctx := &Context{Response: NewResponse(), Writer: &pageFileTestWriter{}}
	if err := WriteActionFiles(ctx, []File{first, second}); err != nil {
		t.Fatal(err)
	}
	if firstClosed != 1 || secondClosed != 1 {
		t.Fatalf("close counts = %d, %d", firstClosed, secondClosed)
	}
}

func TestWriteActionFileSupportsEmptyContentAndEmojiFilename(t *testing.T) {
	file, err := NewFile(FileConfig{Name: "reporte-🚀.txt", Type: "text/plain", Content: strings.NewReader("")})
	if err != nil {
		t.Fatal(err)
	}

	response := NewResponse()
	writer := &pageFileTestWriter{}
	ctx := &Context{Response: response, Writer: writer}
	if err := WriteActionFile(ctx, file); err != nil {
		t.Fatal(err)
	}
	if got := responseHeaderValue(&response.Headers, "Content-Length"); got != "0" {
		t.Fatalf("Content-Length = %q", got)
	}
	_, params, err := mime.ParseMediaType(responseHeaderValue(&response.Headers, "Content-Disposition"))
	if err != nil {
		t.Fatal(err)
	}
	if got := params["filename"]; got != "reporte-🚀.txt" {
		t.Fatalf("filename = %q", got)
	}
	if writer.body.Len() != 0 {
		t.Fatalf("body length = %d", writer.body.Len())
	}
}

func TestWriteActionFilesPreservesDuplicateNamesAndEmptyParts(t *testing.T) {
	first, err := NewFile(FileConfig{Name: "same-🚀.txt", Type: "text/plain", Content: strings.NewReader("")})
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewFile(FileConfig{Name: "same-🚀.txt", Type: "text/plain", Content: strings.NewReader("second")})
	if err != nil {
		t.Fatal(err)
	}

	response := NewResponse()
	writer := &pageFileTestWriter{}
	ctx := &Context{Response: response, Writer: writer}
	if err := WriteActionFiles(ctx, []File{first, second}); err != nil {
		t.Fatal(err)
	}

	_, params, err := mime.ParseMediaType(responseHeaderValue(&response.Headers, "Content-Type"))
	if err != nil {
		t.Fatal(err)
	}
	reader := multipart.NewReader(bytes.NewReader(writer.body.Bytes()), params["boundary"])
	for index, expectedBody := range []string{"", "second"} {
		part, err := reader.NextPart()
		if err != nil {
			t.Fatalf("part %d: %v", index, err)
		}
		if got := part.FileName(); got != "same-🚀.txt" {
			t.Fatalf("part %d filename = %q", index, got)
		}
		body, err := io.ReadAll(part)
		if err != nil {
			t.Fatal(err)
		}
		if got := string(body); got != expectedBody {
			t.Fatalf("part %d body = %q", index, got)
		}
	}
}
