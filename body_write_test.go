package gox

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

var _ io.WriterTo = (Body)(nil)

type bodyTestSink struct {
	n   int
	err error
}

func (w bodyTestSink) Write(p []byte) (int, error) {
	n := w.n
	if n > len(p) {
		n = len(p)
	}
	return n, w.err
}

type bodyTestReader struct {
	io.Reader
	closed   bool
	closeErr error
}

func (r *bodyTestReader) Close() error { r.closed = true; return r.closeErr }

func TestBodyWriteToCountsAndErrors(t *testing.T) {
	failure := errors.New("write failed")
	for _, cached := range []bool{false, true} {
		for _, tc := range []struct {
			name string
			sink io.Writer
			n    int64
			err  error
		}{
			{"success", &bytes.Buffer{}, 5, nil},
			{"short", bodyTestSink{n: 2}, 2, io.ErrShortWrite},
			{"failure", bodyTestSink{n: 2, err: failure}, 2, failure},
			{"nil", nil, 0, nil},
		} {
			name := tc.name
			if cached {
				name += "/cached"
			} else {
				name += "/stream"
			}
			t.Run(name, func(t *testing.T) {
				reader := &bodyTestReader{Reader: strings.NewReader("hello")}
				var b Body = NewBody(reader, "text/plain")
				if cached {
					b = NewBodyFromBytes([]byte("hello"), "text/plain")
				}
				n, err := b.WriteTo(tc.sink)
				if n != tc.n {
					t.Fatalf("bytes=%d want %d", n, tc.n)
				}
				if tc.sink == nil {
					if err == nil {
						t.Fatal("nil destination accepted")
					}
					if reader.closed {
						t.Fatal("reader consumed for nil destination")
					}
				} else {
					if !errors.Is(err, tc.err) {
						t.Fatalf("error=%v want %v", err, tc.err)
					}
					if !cached && !reader.closed {
						t.Fatal("reader not closed")
					}
				}
			})
		}
	}
}

func TestBodyWriteToCloseErrorPreservesCount(t *testing.T) {
	failure := errors.New("close failed")
	reader := &bodyTestReader{Reader: strings.NewReader("hello"), closeErr: failure}
	b := NewBody(reader, "text/plain")
	n, err := b.WriteTo(io.Discard)
	if n != 5 || !errors.Is(err, failure) || !reader.closed {
		t.Fatalf("bytes=%d error=%v closed=%v", n, err, reader.closed)
	}
	n, err = b.WriteTo(io.Discard)
	if n != 0 || !errors.Is(err, failure) {
		t.Fatalf("retry bytes=%d error=%v", n, err)
	}
}

func TestBodyWriteToEmpty(t *testing.T) {
	n, err := NewBody(nil, "").WriteTo(io.Discard)
	if n != 0 || err != nil {
		t.Fatalf("bytes=%d error=%v", n, err)
	}
}
