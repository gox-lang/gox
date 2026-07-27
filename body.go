package gox

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
)

type Body interface {
	GetBytes() ([]byte, error)
	ToText() (string, error)
	ToJSON(out any) error
	WriteTo(dst io.Writer) error
}

type body struct {
	mu          sync.Mutex
	reader      io.ReadCloser
	contentType string
	loaded      bool
	data        []byte
	err         error
}

func NewBody(reader io.ReadCloser, contentType string) Body {
	return &body{reader: reader, contentType: contentType}
}

func (b *body) GetBytes() ([]byte, error) {
	data, err := b.load()
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(data))
	copy(out, data)
	return out, nil
}

func (b *body) ToText() (string, error) {
	data, err := b.load()
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (b *body) ToJSON(out any) error {
	if out == nil {
		return fmt.Errorf("gox body json: output cannot be nil")
	}
	if !isJSONContentType(b.contentType) {
		return fmt.Errorf("gox body json: expected application/json content-type, got %q", b.contentType)
	}
	data, err := b.load()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

func (b *body) WriteTo(dst io.Writer) error {
	if dst == nil {
		return fmt.Errorf("gox body write: destination cannot be nil")
	}

	b.mu.Lock()
	if b.loaded {
		data := b.data
		err := b.err
		b.mu.Unlock()
		if err != nil {
			return err
		}
		_, err = dst.Write(data)
		return err
	}

	reader := b.reader
	b.loaded = true
	b.reader = nil
	b.mu.Unlock()

	if reader == nil {
		_, err := dst.Write(nil)
		return err
	}
	_, err := io.Copy(dst, reader)
	closeErr := reader.Close()
	if err != nil {
		b.mu.Lock()
		b.err = err
		b.mu.Unlock()
		return err
	}
	if closeErr != nil {
		b.mu.Lock()
		b.err = closeErr
		b.mu.Unlock()
		return closeErr
	}
	b.mu.Lock()
	b.data = nil
	b.err = nil
	b.mu.Unlock()
	return nil
}

func (b *body) load() ([]byte, error) {
	b.mu.Lock()
	if b.loaded {
		data, err := b.data, b.err
		b.mu.Unlock()
		return data, err
	}
	reader := b.reader
	b.loaded = true
	b.reader = nil
	b.mu.Unlock()

	if reader == nil {
		b.mu.Lock()
		b.data = nil
		b.err = nil
		b.mu.Unlock()
		return nil, nil
	}

	data, err := io.ReadAll(reader)
	closeErr := reader.Close()
	if err == nil && closeErr != nil {
		err = closeErr
	}
	b.mu.Lock()
	b.data = data
	b.err = err
	b.mu.Unlock()
	return data, err
}

func isJSONContentType(contentType string) bool {
	contentType = strings.TrimSpace(strings.ToLower(contentType))
	if idx := strings.IndexByte(contentType, ';'); idx >= 0 {
		contentType = strings.TrimSpace(contentType[:idx])
	}
	return contentType == "application/json" || strings.HasSuffix(contentType, "+json")
}

func NewBodyFromBytes(data []byte, contentType string) Body {
	copyData := make([]byte, len(data))
	copy(copyData, data)
	return &body{contentType: contentType, loaded: true, data: copyData}
}

// ResetRuntimeBody reuses a Body created by NewBody for a new request.
// It is intended for the GoX server runtime.
func ResetRuntimeBody(value Body, reader io.ReadCloser, contentType string) bool {
	b, ok := value.(*body)
	if !ok || b == nil {
		return false
	}
	b.mu.Lock()
	b.reader = reader
	b.contentType = contentType
	b.loaded = false
	b.data = nil
	b.err = nil
	b.mu.Unlock()
	return true
}
