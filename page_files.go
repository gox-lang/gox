package gox

// WritePageFiles writes Files as an ordered multipart/form-data page response.
// Every file is opened before response headers or body bytes are written, so an
// unavailable source cannot leave a partially initialized HTTP response.
func WritePageFiles(ctx *Context, files []File) error {
	return writeMultipartFiles(ctx, files, "gox files", func(contentType string) {
		ctx.Response.Headers.Set("Content-Type", contentType)
	})
}
