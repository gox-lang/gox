package gox

import (
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/textproto"
)

type openedResponseFile struct {
	info   FileInfo
	reader io.ReadCloser
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

func openResponseFiles(files []File, prefix string) ([]openedResponseFile, error) {
	opened := make([]openedResponseFile, 0, len(files))
	for index, file := range files {
		if file.IsZero() {
			closeResponseFiles(opened)
			return nil, fmt.Errorf("%s: zero file at index %d", prefix, index)
		}
		info := file.Info()
		reader, err := file.Open()
		if err != nil {
			closeResponseFiles(opened)
			return nil, fmt.Errorf("%s: open file at index %d: %w", prefix, index, err)
		}
		opened = append(opened, openedResponseFile{info: info, reader: reader})
	}
	return opened, nil
}

func closeResponseFiles(opened []openedResponseFile) {
	for index := len(opened) - 1; index >= 0; index-- {
		_ = opened[index].reader.Close()
	}
}

func writeMultipartFiles(ctx *Context, files []File, prefix string, setHeaders func(contentType string)) error {
	if err := validateFileResponseContext(ctx, prefix); err != nil {
		return err
	}

	opened, err := openResponseFiles(files, prefix)
	if err != nil {
		return err
	}
	defer closeResponseFiles(opened)

	writer := multipart.NewWriter(ctx.Writer)
	setHeaders(writer.FormDataContentType())

	for index, file := range opened {
		disposition := mime.FormatMediaType("form-data", map[string]string{
			"name":     "files",
			"filename": file.info.Name,
		})
		if disposition == "" {
			return fmt.Errorf("%s: invalid response filename at index %d: %q", prefix, index, file.info.Name)
		}

		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", disposition)
		header.Set("Content-Type", file.info.ContentType)
		part, err := writer.CreatePart(header)
		if err != nil {
			return fmt.Errorf("%s: create multipart part at index %d: %w", prefix, index, err)
		}
		if _, err := io.Copy(part, file.reader); err != nil {
			return fmt.Errorf("%s: write multipart part at index %d: %w", prefix, index, err)
		}
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("%s: close multipart response: %w", prefix, err)
	}
	return nil
}
