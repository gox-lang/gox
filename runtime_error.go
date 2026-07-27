package gox

type GoxErrorStage string

const (
	GoxErrorResolve  GoxErrorStage = "resolve"
	GoxErrorCompile  GoxErrorStage = "compile"
	GoxErrorBuild    GoxErrorStage = "build"
	GoxErrorLoad     GoxErrorStage = "load"
	GoxErrorDispatch GoxErrorStage = "dispatch"
	GoxErrorPanic    GoxErrorStage = "panic"
	GoxErrorWriter   GoxErrorStage = "writer"
)

// Error is the read-only runtime error contract exposed to .gox pages.
// The concrete implementation is owned by gox-server.
type Error interface {
	error

	GetStage() GoxErrorStage
	GetRoute() string
	GetMethod() string
	GetIsAction() bool

	GetFile() string
	GetLine() int
	GetColumn() int
	GetGeneratedFile() string
	GetPluginPath() string

	GetKind() string
	GetMessage() string
	GetHint() string
	GetExpected() string
	GetFound() string

	GetCause() error
	GetStack() string
	GetOriginalError() Error

	GetStatusCode() int
	GetStatusMessage() string
	GetDetails() map[string]string
}
