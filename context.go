package gox

type Context struct {
	runtimeState         any
	clientIslandRuntime  ClientIslandRuntime
	currentClientIsland  *ClientIslandSnapshot
	clientRuntimeWritten bool
	clientAssetsSmall    []uint64
	Request              Request
	Response             *Response
	Session              Session
	Body                 Body
	Writer               Writer
	Error                Error
}

// gox:internal
// ResetRuntimeContext clears a Context for reuse by the GoX server runtime.
// The context and all values reachable through it are request-scoped; callers
// must not retain them after the request handler returns.
func ResetRuntimeContext(ctx *Context) {
	if ctx == nil {
		return
	}
	clientAssetsSmall := ctx.clientAssetsSmall[:0]
	*ctx = Context{clientAssetsSmall: clientAssetsSmall}
}

func (c *Context) HasError() bool {
	return c != nil && c.Error != nil
}

func (c *Context) clientAssetWritten(key uint64) bool {
	if c == nil || key == 0 {
		return false
	}
	for _, current := range c.clientAssetsSmall {
		if current == key {
			return true
		}
	}
	return false
}

func (c *Context) markClientAssetWritten(key uint64) {
	if c == nil || key == 0 || c.clientAssetWritten(key) {
		return
	}
	c.clientAssetsSmall = append(c.clientAssetsSmall, key)
}

// gox:internal
// SetRuntimeContextState attaches opaque runtime-owned state to a context.
func SetRuntimeContextState(ctx *Context, state any) {
	if ctx != nil {
		ctx.runtimeState = state
	}
}

// gox:internal
// RuntimeContextState returns opaque state attached by the GoX server runtime.
func RuntimeContextState(ctx *Context) any {
	if ctx == nil {
		return nil
	}
	return ctx.runtimeState
}
