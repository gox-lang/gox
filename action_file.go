package gox

import (
	"fmt"
	"mime"
	"strconv"
)

// WriteActionFile writes a File as a successful Action response while
// preserving its filename and MIME type while streaming its content source.
func WriteActionFile(ctx *Context, file File) error {
	const prefix = "gox action file"
	if err := validateFileResponseContext(ctx, prefix); err != nil {
		return err
	}
	if file.IsZero() {
		return fmt.Errorf("%s: zero file", prefix)
	}

	info := file.Info()
	reader, err := file.Open()
	if err != nil {
		return err
	}
	defer reader.Close()

	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": info.Name})
	if disposition == "" {
		return fmt.Errorf("%s: invalid response filename %q", prefix, info.Name)
	}

	ctx.Response.Headers.Set("Action-Response", "success")
	ctx.Response.Headers.Set("Action-Type", "file")
	ctx.Response.Headers.Set("Content-Type", info.ContentType)
	ctx.Response.Headers.Set("Content-Disposition", disposition)
	if info.Size >= 0 {
		ctx.Response.Headers.Set("Content-Length", strconv.FormatInt(info.Size, 10))
	}
	if _, err := copyFileContent(ctx.Writer, reader); err != nil {
		return fmt.Errorf("%s: write response: %w", prefix, err)
	}
	return nil
}

// WriteActionFiles streams Files in order as a multipart/form-data successful
// Action response. Sources are opened one at a time.
func WriteActionFiles(ctx *Context, files []File) error {
	return writeMultipartFiles(ctx, files, "gox action files", func(contentType string) {
		ctx.Response.Headers.Set("Action-Response", "success")
		ctx.Response.Headers.Set("Action-Type", "files")
		ctx.Response.Headers.Set("Content-Type", contentType)
	})
}
