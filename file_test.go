package gox

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

func TestNewFileRequiresExactlyOneSource(t *testing.T) {
	if _, err := NewFile(FileConfig{}); err == nil {
		t.Fatal("expected missing source error")
	}
	if _, err := NewFile(FileConfig{Path: "x", Name: "x", Content: strings.NewReader("x")}); err == nil {
		t.Fatal("expected duplicate source error")
	}
	var nilBuffer *bytes.Buffer
	if _, err := NewFile(FileConfig{Name: "x.txt", Content: nilBuffer}); err == nil {
		t.Fatal("expected typed nil reader error")
	}
}

func TestNewFileFromContent(t *testing.T) {
	file, err := NewFile(FileConfig{Name: "report.JSON", Content: strings.NewReader(`{"ok":true}`)})
	if err != nil {
		t.Fatal(err)
	}
	if file.Name() != "report.JSON" || file.Filename() != "report.JSON" {
		t.Fatalf("unexpected name: %q", file.Name())
	}
	if file.Type() != "application/json" {
		t.Fatalf("unexpected MIME: %q", file.Type())
	}
	if file.Size() != 11 || file.Len() != 11 {
		t.Fatalf("unexpected size: %d", file.Size())
	}

	a, err := file.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := file.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	first := make([]byte, 1)
	if _, err := a.Read(first); err != nil {
		t.Fatal(err)
	}
	allB, err := io.ReadAll(b)
	if err != nil {
		t.Fatal(err)
	}
	if first[0] != '{' || string(allB) != `{"ok":true}` {
		t.Fatalf("readers do not have independent cursors: %q %q", first, allB)
	}
}

func TestNewFileContentRequiresNameAndPropagatesReadError(t *testing.T) {
	if _, err := NewFile(FileConfig{Content: strings.NewReader("x")}); err == nil {
		t.Fatal("expected name error")
	}
	if _, err := NewFile(FileConfig{Name: "x.txt", Content: failingReader{}}); err == nil {
		t.Fatal("expected reader error")
	}
}

func TestNewFileFromPathIsLazyAndReopenable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := NewFile(FileConfig{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if got := file.Info(); got.Name != "report.txt" || got.ContentType != "text/plain; charset=utf-8" || got.Size != 5 {
		t.Fatalf("unexpected info: %#v", got)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Open(); err == nil {
		t.Fatal("expected lazy open to fail after removal")
	}
}

func TestNewFileFromPathRejectsDirectory(t *testing.T) {
	if _, err := NewFile(FileConfig{Path: t.TempDir()}); err == nil {
		t.Fatal("expected directory rejection")
	}
}

func TestFileMetadataSetters(t *testing.T) {
	file, err := NewFile(FileConfig{Name: "report.txt", Content: strings.NewReader("x")})
	if err != nil {
		t.Fatal(err)
	}
	if err := file.SetName("report.json"); err != nil {
		t.Fatal(err)
	}
	if file.Type() != "application/json" {
		t.Fatalf("expected inferred JSON MIME, got %q", file.Type())
	}
	if err := file.SetMIMEType(" application/x-gox; version=1 "); err != nil {
		t.Fatal(err)
	}
	if file.Type() != "application/x-gox; version=1" {
		t.Fatalf("unexpected explicit MIME: %q", file.Type())
	}
	if err := file.SetName("report.pdf"); err != nil {
		t.Fatal(err)
	}
	if file.Type() != "application/x-gox; version=1" {
		t.Fatalf("explicit MIME changed with name: %q", file.Type())
	}
	if err := file.SetMIMEType(""); err != nil {
		t.Fatal(err)
	}
	if file.Type() != "application/pdf" {
		t.Fatalf("expected inferred PDF MIME, got %q", file.Type())
	}
}

func TestFileValidation(t *testing.T) {
	invalidNames := []string{"", ".", "..", "a/b.txt", `a\\b.txt`, "a\nb.txt", "a\x00b.txt"}
	for _, name := range invalidNames {
		if _, err := NewFile(FileConfig{Name: name, Content: strings.NewReader("x")}); err == nil {
			t.Errorf("expected invalid name %q to fail", name)
		}
	}
	if _, err := NewFile(FileConfig{Name: "válido.txt", Content: strings.NewReader("x")}); err != nil {
		t.Fatalf("unicode name should be valid: %v", err)
	}
	if _, err := NewFile(FileConfig{Name: "x.txt", Type: "not a mime", Content: strings.NewReader("x")}); err == nil {
		t.Fatal("expected invalid MIME error")
	}
	var zero File
	if err := zero.SetName("x.txt"); err == nil {
		t.Fatal("expected zero file SetName error")
	}
	if err := zero.SetMIMEType("text/plain"); err == nil {
		t.Fatal("expected zero file SetMIMEType error")
	}
	if _, err := zero.Open(); err == nil {
		t.Fatal("expected zero file Open error")
	}
}

func TestFileCopiesShareState(t *testing.T) {
	a, err := NewFile(FileConfig{Name: "a.txt", Content: strings.NewReader("abc")})
	if err != nil {
		t.Fatal(err)
	}
	b := a
	if err := b.SetName("b.json"); err != nil {
		t.Fatal(err)
	}
	if a.Name() != "b.json" || a.Type() != "application/json" {
		t.Fatalf("copy did not share metadata: %#v", a.Info())
	}
}

func TestFileLegacyCursorMaterializesAndMutatesSource(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "legacy.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := NewFile(FileConfig{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte("Y")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	reader, err := file.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "Yello" || file.Size() != 5 {
		t.Fatalf("unexpected materialized mutation: %q size=%d", data, file.Size())
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("legacy cursor should reopen after Close: %v", err)
	}
}

func TestFileReadFromReplacesOpenSource(t *testing.T) {
	file, err := NewFile(FileConfig{Name: "x.bin", Content: strings.NewReader("old")})
	if err != nil {
		t.Fatal(err)
	}
	if n, err := file.ReadFrom(strings.NewReader("replacement")); err != nil || n != 11 {
		t.Fatalf("ReadFrom: n=%d err=%v", n, err)
	}
	reader, err := file.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	data, _ := io.ReadAll(reader)
	if string(data) != "replacement" || file.Size() != 11 {
		t.Fatalf("unexpected replacement: %q size=%d", data, file.Size())
	}
}

func TestFileConcurrentMetadataAccess(t *testing.T) {
	file, err := NewFile(FileConfig{Name: "x.txt", Content: strings.NewReader("data")})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			_ = file.SetName("x.json")
			_ = file.SetMIMEType("")
		}(i)
		go func() {
			defer wg.Done()
			_ = file.Info()
			reader, err := file.Open()
			if err == nil {
				_, _ = io.Copy(io.Discard, reader)
				_ = reader.Close()
			}
		}()
	}
	wg.Wait()
}
