package gox

// Middleware is the generic middleware contract used by Gox user code.
type Middleware[T any] func(ctx *Context, out *T) error

func UseMiddlewares[T any](ctx *Context, out *T, middlewares ...Middleware[T]) error {
	for _, middleware := range middlewares {
		if err := middleware(ctx, out); err != nil {
			return err
		}
	}

	return nil
}
