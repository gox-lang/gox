package gox

import (
	"fmt"
	"mime"
	"strconv"
)

// WritePageFile writes a File as an HTTP page response while preserving its
// filename, MIME type, and lazy/reopenable content source.
func WritePageFile(ctx *Context, file File) error {
	if err := validateFileResponseContext(ctx, "gox file"); err != nil {
		return err
	}
	if file.IsZero() {
		return fmt.Errorf("gox file: zero file")
	}

	info := file.Info()
	reader, err := file.Open()
	if err != nil {
		return err
	}
	defer reader.Close()

	disposition := mime.FormatMediaType("inline", map[string]string{"filename": info.Name})
	if disposition == "" {
		return fmt.Errorf("gox file: invalid response filename %q", info.Name)
	}

	if _, ok := ctx.Response.Headers.Get("Content-Type"); !ok {
		ctx.Response.Headers.Set("Content-Type", info.ContentType)
	}
	if _, ok := ctx.Response.Headers.Get("Content-Disposition"); !ok {
		ctx.Response.Headers.Set("Content-Disposition", disposition)
	}
	ctx.Response.Headers.Set("Content-Length", strconv.FormatInt(info.Size, 10))
	ctx.Writer.WriteStream(reader)
	return nil
}
