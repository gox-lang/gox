package gox

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"unicode/utf8"
)

const defaultFileMIMEType = "application/octet-stream"

type FileConfig struct {
	// Path creates a lazy, reopenable file backed by a regular filesystem file.
	Path string
	Name string
	Type string
	// Content is never read by NewFile. bytes.Reader, strings.Reader, and
	// bytes.Buffer are reopened without copying; other readers are one-shot.
	Content io.Reader
	// Open provides a lazy, reopenable content source. It is useful when the
	// bytes are owned by another subsystem (for example multipart.FileHeader).
	// Exactly one of Path, Content, or Open must be provided.
	Open func() (io.ReadCloser, error)
	// Size is the content size when known. Use -1 when it is unknown. For
	// Content, zero means "infer when possible, otherwise unknown".
	Size int64
}

type FileInfo struct {
	Name        string
	ContentType string
	Size        int64
}

type File struct {
	state *fileState
}

type fileState struct {
	mu sync.RWMutex

	name         string
	contentType  string
	explicitType bool
	size         int64
	open         func() (io.ReadCloser, error)

	data   []byte
	reader *bytes.Reader
	cursor io.ReadCloser
	loaded bool
	closed bool
	used   bool
}

func NewFile(config FileConfig) (File, error) {
	hasPath := strings.TrimSpace(config.Path) != ""
	hasContent := !isNilReader(config.Content)
	hasOpen := config.Open != nil
	if boolCount(hasPath, hasContent, hasOpen) != 1 {
		return File{}, fmt.Errorf("gox file: exactly one of Path, Content, or Open is required")
	}

	if hasPath {
		return newPathFile(config)
	}
	if hasOpen {
		return newOpenFile(config)
	}
	return newContentFile(config)
}

func boolCount(values ...bool) int {
	count := 0
	for _, value := range values {
		if value {
			count++
		}
	}
	return count
}

// Deprecated: use NewFile with FileConfig.
func NewFileWithContentType(name string, contentType string, reader io.Reader) (File, error) {
	return NewFile(FileConfig{Name: name, Type: contentType, Content: reader})
}

func newPathFile(config FileConfig) (File, error) {
	path := filepath.Clean(config.Path)
	info, err := os.Stat(path)
	if err != nil {
		return File{}, fmt.Errorf("gox file: stat %q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return File{}, fmt.Errorf("gox file: path %q is not a regular file", path)
	}

	name := config.Name
	if strings.TrimSpace(name) == "" {
		name = filepath.Base(path)
	}
	name, err = normalizeFileName(name)
	if err != nil {
		return File{}, err
	}
	contentType, explicitType, err := resolveFileMIMEType(name, config.Type)
	if err != nil {
		return File{}, err
	}

	state := &fileState{
		name:         name,
		contentType:  contentType,
		explicitType: explicitType,
		size:         info.Size(),
	}
	state.open = func() (io.ReadCloser, error) { return os.Open(path) }
	return File{state: state}, nil
}

func newContentFile(config FileConfig) (File, error) {
	name, err := normalizeFileName(config.Name)
	if err != nil {
		return File{}, err
	}
	contentType, explicitType, err := resolveFileMIMEType(name, config.Type)
	if err != nil {
		return File{}, err
	}
	state := &fileState{name: name, contentType: contentType, explicitType: explicitType, size: -1}
	if config.Size < 0 {
		state.size = -1
	} else if config.Size > 0 {
		state.size = config.Size
	}
	if open, size, ok := reusableReader(config.Content); ok {
		state.open = open
		if config.Size == 0 {
			state.size = size
		}
	} else {
		reader := config.Content
		state.open = func() (io.ReadCloser, error) {
			state.mu.Lock()
			defer state.mu.Unlock()
			if state.used {
				return nil, fmt.Errorf("content source has already been consumed")
			}
			state.used = true
			if closer, ok := reader.(io.ReadCloser); ok {
				return closer, nil
			}
			return io.NopCloser(reader), nil
		}
	}
	return File{state: state}, nil
}

func newOpenFile(config FileConfig) (File, error) {
	name, err := normalizeFileName(config.Name)
	if err != nil {
		return File{}, err
	}
	contentType, explicitType, err := resolveFileMIMEType(name, config.Type)
	if err != nil {
		return File{}, err
	}
	size := config.Size
	if size < 0 {
		size = -1
	}
	return File{state: &fileState{
		name: name, contentType: contentType, explicitType: explicitType,
		size: size, open: config.Open,
	}}, nil
}

func reusableReader(reader io.Reader) (func() (io.ReadCloser, error), int64, bool) {
	switch value := reader.(type) {
	case *bytes.Reader:
		snapshot := *value
		size := snapshot.Size()
		return func() (io.ReadCloser, error) {
			clone := snapshot
			_, _ = clone.Seek(0, io.SeekStart)
			return io.NopCloser(&clone), nil
		}, size, true
	case *strings.Reader:
		snapshot := *value
		size := snapshot.Size()
		return func() (io.ReadCloser, error) {
			clone := snapshot
			_, _ = clone.Seek(0, io.SeekStart)
			return io.NopCloser(&clone), nil
		}, size, true
	case *bytes.Buffer:
		data := value.Bytes()
		return func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(data)), nil
		}, int64(len(data)), true
	default:
		return nil, -1, false
	}
}

func newMaterializedFile(name, contentType string, explicitType bool, data []byte) File {
	state := &fileState{name: name, contentType: contentType, explicitType: explicitType}
	state.setMaterializedLocked(data, 0)
	return File{state: state}
}

func isNilReader(reader io.Reader) bool {
	if reader == nil {
		return true
	}
	value := reflect.ValueOf(reader)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func normalizeFileName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("gox file: name cannot be empty")
	}
	if name == "." || name == ".." {
		return "", fmt.Errorf("gox file: invalid name %q", name)
	}
	if !utf8.ValidString(name) {
		return "", fmt.Errorf("gox file: name must be valid UTF-8")
	}
	if strings.ContainsAny(name, `/\\`) {
		return "", fmt.Errorf("gox file: name cannot contain path separators")
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return "", fmt.Errorf("gox file: name cannot contain control characters")
		}
	}
	return name, nil
}

func resolveFileMIMEType(name, value string) (string, bool, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return inferFileMIMEType(name), false, nil
	}
	if _, _, err := mime.ParseMediaType(value); err != nil {
		return "", false, fmt.Errorf("gox file: invalid MIME type %q: %w", value, err)
	}
	return value, true, nil
}

func inferFileMIMEType(name string) string {
	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(name)))
	if contentType == "" {
		return defaultFileMIMEType
	}
	return contentType
}

func (file File) IsZero() bool { return file.state == nil }

func (file File) Info() FileInfo {
	if file.state == nil {
		return FileInfo{}
	}
	file.state.mu.RLock()
	defer file.state.mu.RUnlock()
	return FileInfo{Name: file.state.name, ContentType: file.state.contentType, Size: file.state.size}
}

func (file File) Name() string        { return file.Info().Name }
func (file File) Filename() string    { return file.Name() }
func (file File) Len() int64          { return file.Info().Size }
func (file File) Size() int64         { return file.Len() }
func (file File) ContentType() string { return file.Info().ContentType }
func (file File) Type() string        { return file.ContentType() }

func (file File) SetName(name string) error {
	if file.state == nil {
		return fmt.Errorf("gox file: zero file")
	}
	name, err := normalizeFileName(name)
	if err != nil {
		return err
	}
	file.state.mu.Lock()
	defer file.state.mu.Unlock()
	file.state.name = name
	if !file.state.explicitType {
		file.state.contentType = inferFileMIMEType(name)
	}
	return nil
}

func (file File) SetMIMEType(contentType string) error {
	if file.state == nil {
		return fmt.Errorf("gox file: zero file")
	}
	contentType = strings.TrimSpace(contentType)
	file.state.mu.Lock()
	defer file.state.mu.Unlock()
	if contentType == "" {
		file.state.explicitType = false
		file.state.contentType = inferFileMIMEType(file.state.name)
		return nil
	}
	if _, _, err := mime.ParseMediaType(contentType); err != nil {
		return fmt.Errorf("gox file: invalid MIME type %q: %w", contentType, err)
	}
	file.state.contentType = contentType
	file.state.explicitType = true
	return nil
}

func (file File) Open() (io.ReadCloser, error) {
	if file.state == nil {
		return nil, fmt.Errorf("gox file: zero file")
	}
	file.state.mu.RLock()
	open := file.state.open
	file.state.mu.RUnlock()
	if open == nil {
		return nil, fmt.Errorf("gox file: content source is unavailable")
	}
	reader, err := open()
	if err != nil {
		return nil, fmt.Errorf("gox file: open content: %w", err)
	}
	return reader, nil
}

func (file File) ensureCompatibilityReader() error {
	if file.state == nil {
		return fmt.Errorf("gox file: zero file")
	}
	file.state.mu.Lock()
	if file.state.loaded {
		if file.state.closed {
			file.state.reader = bytes.NewReader(file.state.data)
			file.state.closed = false
		}
		file.state.mu.Unlock()
		return nil
	}
	open := file.state.open
	file.state.mu.Unlock()
	if open == nil {
		return fmt.Errorf("gox file: content source is unavailable")
	}

	reader, err := open()
	if err != nil {
		return fmt.Errorf("gox file: open content: %w", err)
	}
	data, readErr := io.ReadAll(reader)
	closeErr := reader.Close()
	if readErr != nil {
		return fmt.Errorf("gox file: read content: %w", readErr)
	}
	if closeErr != nil {
		return fmt.Errorf("gox file: close content: %w", closeErr)
	}

	file.state.mu.Lock()
	if !file.state.loaded {
		file.state.setMaterializedLocked(data, 0)
	}
	file.state.mu.Unlock()
	return nil
}

func (state *fileState) setMaterializedLocked(data []byte, position int64) {
	// Every caller transfers ownership of data. Mutation paths create a fresh
	// slice first, so one immutable backing array can safely serve both the
	// compatibility cursor and independently reopened readers.
	state.data = data
	state.size = int64(len(data))
	state.loaded = true
	state.closed = false
	state.reader = bytes.NewReader(data)
	if position > int64(len(data)) {
		position = int64(len(data))
	}
	_, _ = state.reader.Seek(position, io.SeekStart)
	snapshot := data
	state.open = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(snapshot)), nil
	}
}

func (file File) Read(p []byte) (int, error) {
	if file.state == nil {
		return 0, io.EOF
	}
	file.state.mu.Lock()
	if file.state.closed {
		file.state.mu.Unlock()
		return 0, fmt.Errorf("gox file: closed")
	}
	if file.state.loaded {
		n, err := file.state.reader.Read(p)
		file.state.mu.Unlock()
		return n, err
	}
	cursor := file.state.cursor
	open := file.state.open
	file.state.mu.Unlock()
	if cursor == nil {
		if open == nil {
			return 0, fmt.Errorf("gox file: content source is unavailable")
		}
		opened, err := open()
		if err != nil {
			return 0, fmt.Errorf("gox file: open content: %w", err)
		}
		file.state.mu.Lock()
		if file.state.cursor == nil {
			file.state.cursor = opened
			cursor = opened
		} else {
			cursor = file.state.cursor
			_ = opened.Close()
		}
		file.state.mu.Unlock()
	}
	return cursor.Read(p)
}

func (file File) Write(p []byte) (int, error) {
	if err := file.ensureCompatibilityReader(); err != nil {
		return 0, err
	}
	file.state.mu.Lock()
	defer file.state.mu.Unlock()
	if file.state.closed {
		return 0, fmt.Errorf("gox file: closed")
	}
	position, _ := file.state.reader.Seek(0, io.SeekCurrent)
	end := int(position) + len(p)
	data := append([]byte(nil), file.state.data...)
	if end > len(data) {
		data = append(data, make([]byte, end-len(data))...)
	}
	copy(data[int(position):end], p)
	file.state.setMaterializedLocked(data, int64(end))
	return len(p), nil
}

func (file File) Seek(offset int64, whence int) (int64, error) {
	if err := file.ensureCompatibilityReader(); err != nil {
		return 0, err
	}
	file.state.mu.Lock()
	defer file.state.mu.Unlock()
	if file.state.closed {
		return 0, fmt.Errorf("gox file: closed")
	}
	return file.state.reader.Seek(offset, whence)
}

func (file File) Close() error {
	if file.state == nil {
		return nil
	}
	file.state.mu.Lock()
	file.state.closed = true
	cursor := file.state.cursor
	file.state.cursor = nil
	file.state.mu.Unlock()
	if cursor != nil {
		return cursor.Close()
	}
	return nil
}

func (file File) WriteTo(writer io.Writer) (int64, error) {
	if writer == nil {
		return 0, fmt.Errorf("gox file: writer cannot be nil")
	}
	if file.state == nil {
		return 0, fmt.Errorf("gox file: zero file")
	}
	return io.Copy(writer, struct{ io.Reader }{Reader: file})
}

func (file File) ReadFrom(reader io.Reader) (int64, error) {
	if file.state == nil {
		return 0, fmt.Errorf("gox file: zero file")
	}
	if isNilReader(reader) {
		return 0, fmt.Errorf("gox file: reader cannot be nil")
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return 0, err
	}
	file.state.mu.Lock()
	file.state.setMaterializedLocked(data, 0)
	file.state.mu.Unlock()
	return int64(len(data)), nil
}
