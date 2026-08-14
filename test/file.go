package test

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"strings"

	"github.com/gox-lang/gox"
)

func responseArtifactFiles(response ResponseSnapshot) ([]gox.File, bool) {
	contentType := firstHeaderValue(response.Headers, "Content-Type")
	disposition := firstHeaderValue(response.Headers, "Content-Disposition")
	if strings.HasPrefix(strings.ToLower(contentType), "multipart/form-data") {
		files, err := decodeMultipartFiles(contentType, []byte(response.Body))
		return files, err == nil
	}
	if _, params, err := mime.ParseMediaType(disposition); err == nil && params["filename"] != "" {
		mediaType, _, err := mime.ParseMediaType(contentType)
		if err != nil {
			return nil, false
		}
		file, err := gox.NewFile(gox.FileConfig{Name: params["filename"], Type: mediaType, Content: strings.NewReader(response.Body)})
		if err != nil {
			return nil, false
		}
		return []gox.File{file}, true
	}
	return nil, false
}

func firstHeaderValue(headers map[string][]string, name string) string {
	for key, values := range headers {
		if strings.EqualFold(key, name) && len(values) > 0 {
			return values[0]
		}
	}
	return ""
}

type fileMetadata struct {
	Name        string `json:"name"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
}

func fileMetadataList(files []gox.File) []fileMetadata {
	out := make([]fileMetadata, len(files))
	for i, file := range files {
		info := file.Info()
		out[i] = fileMetadata{Name: info.Name, ContentType: info.ContentType, Size: info.Size}
	}
	return out
}

func matchFileBody(expected gox.File, contentType, disposition string, body []byte) BodyMatchResult {
	actualName := ""
	if _, params, err := mime.ParseMediaType(disposition); err == nil {
		actualName = params["filename"]
	}
	actualType := contentType
	if mediaType, _, err := mime.ParseMediaType(contentType); err == nil {
		actualType = mediaType
	}
	actual, err := gox.NewFile(gox.FileConfig{Name: actualName, Type: actualType, Content: bytes.NewReader(body)})
	if err != nil {
		return BodyMatchResult{Rule: "equal", Expected: fileMetadataList([]gox.File{expected})[0], Match: false, Message: err.Error()}
	}
	match, message := filesEqual([]gox.File{expected}, []gox.File{actual})
	return BodyMatchResult{Rule: "equal", Expected: fileMetadataList([]gox.File{expected})[0], Response: fileMetadataList([]gox.File{actual})[0], Match: match, Message: message}
}

func decodeMultipartFiles(contentType string, body []byte) ([]gox.File, error) {
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || !strings.EqualFold(mediaType, "multipart/form-data") || params["boundary"] == "" {
		return nil, fmt.Errorf("gox/test: response is not valid multipart/form-data")
	}
	reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	var files []gox.File
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("gox/test: read multipart response: %w", err)
		}
		name := part.FileName()
		if name == "" {
			part.Close()
			return nil, fmt.Errorf("gox/test: multipart response contains a non-file part")
		}
		data, readErr := io.ReadAll(part)
		part.Close()
		if readErr != nil {
			return nil, readErr
		}
		file, createErr := gox.NewFile(gox.FileConfig{Name: name, Type: part.Header.Get("Content-Type"), Content: bytes.NewReader(data)})
		if createErr != nil {
			return nil, createErr
		}
		files = append(files, file)
	}
	return files, nil
}

func filesEqual(expected, actual []gox.File) (bool, string) {
	if len(expected) != len(actual) {
		return false, fmt.Sprintf("file count differs: expected %d, received %d", len(expected), len(actual))
	}
	for i := range expected {
		e, a := expected[i].Info(), actual[i].Info()
		if e.Name != a.Name {
			return false, fmt.Sprintf("file %d name differs: expected %q, received %q", i, e.Name, a.Name)
		}
		if e.ContentType != a.ContentType {
			return false, fmt.Sprintf("file %d content type differs: expected %q, received %q", i, e.ContentType, a.ContentType)
		}
		if e.Size != a.Size {
			return false, fmt.Sprintf("file %d size differs: expected %d, received %d", i, e.Size, a.Size)
		}
		er, err := expected[i].Open()
		if err != nil {
			return false, err.Error()
		}
		ar, err := actual[i].Open()
		if err != nil {
			er.Close()
			return false, err.Error()
		}
		eb, ee := io.ReadAll(er)
		ab, ae := io.ReadAll(ar)
		er.Close()
		ar.Close()
		if ee != nil {
			return false, ee.Error()
		}
		if ae != nil {
			return false, ae.Error()
		}
		if !bytes.Equal(eb, ab) {
			return false, fmt.Sprintf("file %d content differs", i)
		}
	}
	return true, ""
}

func fileValuesEqual(a, b any) (bool, bool) {
	switch expected := a.(type) {
	case gox.File:
		actual, ok := b.(gox.File)
		if !ok {
			return false, true
		}
		match, _ := filesEqual([]gox.File{expected}, []gox.File{actual})
		return match, true
	case []gox.File:
		actual, ok := b.([]gox.File)
		if !ok {
			return false, true
		}
		match, _ := filesEqual(expected, actual)
		return match, true
	}
	return false, false
}

func fileDisplayValue(value any) any {
	switch typed := value.(type) {
	case gox.File:
		items := fileMetadataList([]gox.File{typed})
		if len(items) == 1 {
			return items[0]
		}
	case []gox.File:
		return fileMetadataList(typed)
	}
	return value
}
