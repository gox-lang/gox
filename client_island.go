package gox

import (
	"encoding/json"
)

// gox:internal
// ClientAssetTag is runtime data used by generated GoX client-island calls.
// It is an implementation contract for generated code and the server runtime.
type ClientAssetTag struct {
	ID   uint64
	HTML []byte
}

// gox:internal
type ClientIslandSnapshot struct {
	ScriptURL  string
	StyleTags  []ClientAssetTag
	DataAssets []byte
}

// gox:internal
// ClientIslandRuntime resolves the current client build for a logical client key.
type ClientIslandRuntime interface {
	ResolveClientIsland(clientKey string) (ClientIslandSnapshot, bool)
	ClientRuntimeHTML() []byte
}

// gox:internal
// BindClientIslandRuntime binds the current runtime client-asset resolver to a request context.
func BindClientIslandRuntime(ctx *Context, runtime ClientIslandRuntime) {
	if ctx == nil {
		return
	}
	ctx.clientIslandRuntime = runtime
}

// gox:internal
// WriteClientRuntime writes the shared gox-isle runtime assets once for the response.
func WriteClientRuntime(ctx *Context) {
	if ctx == nil || ctx.Writer == nil || ctx.clientIslandRuntime == nil || ctx.clientRuntimeWritten {
		return
	}
	html := ctx.clientIslandRuntime.ClientRuntimeHTML()
	if len(html) == 0 {
		return
	}
	ctx.Writer.WriteRawBytes(html)
	ctx.clientRuntimeWritten = true
}

// gox:internal
// WriteClientIsland writes one client island using the current hot-reload asset snapshot.
// This function is intended for generated GoX code.
func WriteClientIsland(ctx *Context, clientKey string, writeOpen func(*Context), writeChildren func(*Context)) {
	if ctx == nil || ctx.Writer == nil || ctx.clientIslandRuntime == nil {
		return
	}
	island, ok := ctx.clientIslandRuntime.ResolveClientIsland(clientKey)
	if !ok || island.ScriptURL == "" {
		return
	}
	WriteClientRuntime(ctx)
	for _, tag := range island.StyleTags {
		if tag.ID == 0 || len(tag.HTML) == 0 {
			continue
		}
		if ctx.clientAssetWritten(tag.ID) {
			continue
		}
		ctx.Writer.WriteRawBytes(tag.HTML)
		ctx.markClientAssetWritten(tag.ID)
	}
	ctx.Writer.WriteRawString(`<gox-isle data-script="`)
	ctx.Writer.WriteTextString(island.ScriptURL)
	if err := ctx.Writer.WriteByte('"'); err != nil {
		panic(err)
	}
	previous := ctx.currentClientIsland
	ctx.currentClientIsland = &island
	if writeOpen != nil {
		writeOpen(ctx)
	}
	ctx.currentClientIsland = previous
	if err := ctx.Writer.WriteByte('>'); err != nil {
		panic(err)
	}
	if writeChildren != nil {
		writeChildren(ctx)
	}
	ctx.Writer.WriteRawString(`</gox-isle>`)
}

// gox:internal
// WriteClientIslandDataAssets writes the current island's adopted-style-sheet payload.
// It is intended for generated isolate attribute code.
func WriteClientIslandDataAssets(ctx *Context) {
	if ctx == nil || ctx.Writer == nil || ctx.currentClientIsland == nil || len(ctx.currentClientIsland.DataAssets) == 0 {
		return
	}
	ctx.Writer.WriteRawBytes(ctx.currentClientIsland.DataAssets)
}

// gox:internal
// WriteClientPropsAttribute writes dynamic client props as an HTML-escaped JSON attribute.
// Static props are precompiled by the GoX compiler and do not call this function.
func WriteClientPropsAttribute(w Writer, props map[string]any, spreads ...any) {
	if w == nil {
		return
	}
	for _, spread := range spreads {
		switch values := spread.(type) {
		case map[string]any:
			for key, value := range values {
				props[key] = value
			}
		case map[string]string:
			for key, value := range values {
				props[key] = value
			}
		}
	}
	if len(props) == 0 {
		return
	}
	encoded, err := json.Marshal(props)
	if err != nil {
		return
	}
	w.WriteRawString(` data-props="`)
	w.WriteTextBytes(encoded)
	if err := w.WriteByte('"'); err != nil {
		panic(err)
	}
}
