package gox

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

type pageFileTestWriter struct {
	Writer
	body bytes.Buffer
	err  error
}

func (writer *pageFileTestWriter) Write(data []byte) (int, error) {
	return writer.body.Write(data)
}

func (writer *pageFileTestWriter) WriteStream(reader io.Reader) {
	_, writer.err = io.Copy(&writer.body, reader)
}

func (writer *pageFileTestWriter) WriteError(err error) {
	writer.err = err
}

func TestWritePageFileWritesMetadataAndContent(t *testing.T) {
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
	if err := WritePageFile(ctx, file); err != nil {
		t.Fatal(err)
	}

	if got := responseHeaderValue(&response.Headers, "Content-Type"); got != "application/pdf" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := responseHeaderValue(&response.Headers, "Content-Disposition"); got != `inline; filename=report.pdf` {
		t.Fatalf("Content-Disposition = %q", got)
	}
	if got := responseHeaderValue(&response.Headers, "Content-Length"); got != "8" {
		t.Fatalf("Content-Length = %q", got)
	}
	if got := writer.body.String(); got != "pdf-data" {
		t.Fatalf("body = %q", got)
	}
}

func TestWritePageFileStreamsUnknownSizeWithoutContentLength(t *testing.T) {
	file, err := NewFile(FileConfig{Name: "stream.txt", Content: &observedReader{data: strings.NewReader("stream")}})
	if err != nil {
		t.Fatal(err)
	}
	response := NewResponse()
	writer := &pageFileTestWriter{}
	if err := WritePageFile(&Context{Response: response, Writer: writer}, file); err != nil {
		t.Fatal(err)
	}
	if _, ok := response.Headers.Get("Content-Length"); ok {
		t.Fatal("unknown stream must not advertise Content-Length")
	}
	if writer.body.String() != "stream" {
		t.Fatalf("body = %q", writer.body.String())
	}
}

func TestWritePageFileOpensBeforeMutatingHeaders(t *testing.T) {
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
	if err := WritePageFile(ctx, file); err == nil {
		t.Fatal("expected open error")
	}
	if !response.Headers.IsEmpty() {
		t.Fatalf("headers mutated before open: %#v", response.Headers.Values())
	}
	if writer.body.Len() != 0 {
		t.Fatalf("unexpected body: %q", writer.body.String())
	}
}

func TestWritePageFilePreservesExplicitDisposition(t *testing.T) {
	file, err := NewFile(FileConfig{Name: "report.pdf", Content: strings.NewReader("hello")})
	if err != nil {
		t.Fatal(err)
	}
	response := NewResponse()
	writer := &pageFileTestWriter{}
	ctx := &Context{Response: response, Writer: writer}
	response.Headers.Set("Content-Disposition", `attachment; filename="download.pdf"`)
	if err := WritePageFile(ctx, file); err != nil {
		t.Fatal(err)
	}
	if got := responseHeaderValue(&response.Headers, "Content-Disposition"); got != `attachment; filename="download.pdf"` {
		t.Fatalf("Content-Disposition=%q", got)
	}
}
