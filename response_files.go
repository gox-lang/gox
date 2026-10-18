package gox

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
	"strings"
	"sync"
)

var multipartCopyBuffers = sync.Pool{New: func() any {
	buffer := make([]byte, 32<<10)
	return &buffer
}}

func copyFileContent(writer io.Writer, reader io.Reader) (int64, error) {
	bufferPtr := multipartCopyBuffers.Get().(*[]byte)
	written, err := io.CopyBuffer(writer, reader, *bufferPtr)
	multipartCopyBuffers.Put(bufferPtr)
	return written, err
}

func validateFileResponseContext(ctx *Context, prefix string) error {
	if ctx == nil {
		return fmt.Errorf("%s: nil context", prefix)
	}
	if ctx.Response == nil {
		return fmt.Errorf("%s: nil response", prefix)
	}
	if ctx.Writer == nil {
		return fmt.Errorf("%s: nil writer", prefix)
	}
	return nil
}

func writeMultipartFiles(ctx *Context, files []File, prefix string, setHeaders func(contentType string)) error {
	if err := validateFileResponseContext(ctx, prefix); err != nil {
		return err
	}

	for index, file := range files {
		if file.IsZero() {
			return fmt.Errorf("%s: zero file at index %d", prefix, index)
		}
	}

	writer := multipart.NewWriter(ctx.Writer)
	setHeaders(writer.FormDataContentType())

	for index, file := range files {
		info := file.Info()
		// FormData parsers expect quoted parameters and UTF-8 filenames,
		// rather than RFC 2231 filename* parameters from mime.FormatMediaType.
		disposition := `form-data; name="files"; filename="` +
			strings.ReplaceAll(info.Name, `"`, "%22") + `"`

		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", disposition)
		header.Set("Content-Type", info.ContentType)
		part, err := writer.CreatePart(header)
		if err != nil {
			return fmt.Errorf("%s: create multipart part at index %d: %w", prefix, index, err)
		}
		reader, err := file.Open()
		if err != nil {
			return fmt.Errorf("%s: open file at index %d: %w", prefix, index, err)
		}
		_, copyErr := copyFileContent(part, reader)
		closeErr := reader.Close()
		if copyErr != nil {
			return fmt.Errorf("%s: write multipart part at index %d: %w", prefix, index, copyErr)
		}
		if closeErr != nil {
			return fmt.Errorf("%s: close file at index %d: %w", prefix, index, closeErr)
		}
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("%s: close multipart response: %w", prefix, err)
	}
	return nil
}
