package gox

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

type fileBenchmarkWriter struct{ Writer }

func (fileBenchmarkWriter) Write(p []byte) (int, error) { return len(p), nil }
func (fileBenchmarkWriter) WriteStream(r io.Reader)     { _, _ = io.Copy(io.Discard, r) }
func (fileBenchmarkWriter) WriteError(error)            {}

func benchmarkContentFile(b *testing.B, size int) {
	data := bytes.Repeat([]byte("x"), size)
	b.ReportAllocs()
	b.SetBytes(int64(size))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		file, err := NewFile(FileConfig{Name: "payload.bin", Content: bytes.NewReader(data)})
		if err != nil {
			b.Fatal(err)
		}
		if file.Size() != int64(size) {
			b.Fatal("unexpected size")
		}
	}
}

func BenchmarkNewFileContent1KiB(b *testing.B) { benchmarkContentFile(b, 1<<10) }
func BenchmarkNewFileContent1MiB(b *testing.B) { benchmarkContentFile(b, 1<<20) }

func BenchmarkFileOpenAndCopy1MiB(b *testing.B) {
	data := bytes.Repeat([]byte("x"), 1<<20)
	file, err := NewFile(FileConfig{Name: "payload.bin", Content: bytes.NewReader(data)})
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r, err := file.Open()
		if err != nil {
			b.Fatal(err)
		}
		if _, err := io.Copy(io.Discard, r); err != nil {
			b.Fatal(err)
		}
		_ = r.Close()
	}
}

func BenchmarkWritePageFile1MiB(b *testing.B) {
	data := bytes.Repeat([]byte("x"), 1<<20)
	file, err := NewFile(FileConfig{Name: "payload.bin", Content: bytes.NewReader(data)})
	if err != nil {
		b.Fatal(err)
	}
	writer := fileBenchmarkWriter{}
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := &Context{Response: NewResponse(), Writer: writer}
		if err := WritePageFile(ctx, file); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkWriteActionFile1MiB(b *testing.B) {
	data := bytes.Repeat([]byte("x"), 1<<20)
	file, err := NewFile(FileConfig{Name: "payload.bin", Content: bytes.NewReader(data)})
	if err != nil {
		b.Fatal(err)
	}
	writer := fileBenchmarkWriter{}
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := &Context{Response: NewResponse(), Writer: writer}
		if err := WriteActionFile(ctx, file); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkWriteActionFiles2x64KiB(b *testing.B) {
	data := bytes.Repeat([]byte("x"), 64<<10)
	first, _ := NewFile(FileConfig{Name: "first.bin", Content: bytes.NewReader(data)})
	second, _ := NewFile(FileConfig{Name: "second.bin", Content: bytes.NewReader(data)})
	writer := fileBenchmarkWriter{}
	b.ReportAllocs()
	b.SetBytes(int64(2 * len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := &Context{Response: NewResponse(), Writer: writer}
		if err := WriteActionFiles(ctx, []File{first, second}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNewFilePath(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "payload.bin")
	if err := os.WriteFile(path, bytes.Repeat([]byte("x"), 1<<20), 0o600); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		file, err := NewFile(FileConfig{Path: path, Name: "payload-" + strconv.Itoa(i%10) + ".bin"})
		if err != nil || file.Size() != 1<<20 {
			b.Fatal(err)
		}
	}
}
