package gox

// GoxSocketErrorCode is a stable WebSocket error identifier shared by Go and JavaScript.
type GoxSocketErrorCode string

const (
	GoxSocketHandshakeFailed       GoxSocketErrorCode = "HANDSHAKE_FAILED"
	GoxSocketConnectionFailed      GoxSocketErrorCode = "CONNECTION_FAILED"
	GoxSocketTransportError        GoxSocketErrorCode = "TRANSPORT_ERROR"
	GoxSocketHandlerFailed         GoxSocketErrorCode = "HANDLER_FAILED"
	GoxSocketInvalidState          GoxSocketErrorCode = "INVALID_STATE"
	GoxSocketConnectionClosed      GoxSocketErrorCode = "CONNECTION_CLOSED"
	GoxSocketProtocolError         GoxSocketErrorCode = "PROTOCOL_ERROR"
	GoxSocketEncodeFailed          GoxSocketErrorCode = "ENCODE_FAILED"
	GoxSocketBindFailed            GoxSocketErrorCode = "BIND_FAILED"
	GoxSocketEventNotFound         GoxSocketErrorCode = "EVENT_NOT_FOUND"
	GoxSocketArgumentCountMismatch GoxSocketErrorCode = "ARGUMENT_COUNT_MISMATCH"
	GoxSocketArgumentTypeMismatch  GoxSocketErrorCode = "ARGUMENT_TYPE_MISMATCH"
	GoxSocketSessionSaveFailed     GoxSocketErrorCode = "SESSION_SAVE_FAILED"
	GoxSocketSessionTouchFailed    GoxSocketErrorCode = "SESSION_TOUCH_FAILED"
	GoxSocketSessionExpired        GoxSocketErrorCode = "SESSION_EXPIRED"
)

// GoxSocketErrorPhase identifies the part of the WebSocket lifecycle that failed.
type GoxSocketErrorPhase string

const (
	GoxSocketPhaseHandshake  GoxSocketErrorPhase = "handshake"
	GoxSocketPhaseConnection GoxSocketErrorPhase = "connection"
	GoxSocketPhaseTransport  GoxSocketErrorPhase = "transport"
	GoxSocketPhaseHandler    GoxSocketErrorPhase = "handler"
	GoxSocketPhaseProtocol   GoxSocketErrorPhase = "protocol"
	GoxSocketPhaseSession    GoxSocketErrorPhase = "session"
)

// GoxSocketError is the canonical WebSocket error exposed by GOX.
type GoxSocketError struct {
	Code      GoxSocketErrorCode
	Message   string
	Phase     GoxSocketErrorPhase
	Event     string
	Retryable bool
	Cause     error
	EventID   uint64
	Argument  int
	Received  string
	Expected  string
	Remote    bool
}

func (err GoxSocketError) Error() string { return err.Message }

func (err GoxSocketError) Unwrap() error { return err.Cause }

// NewGoxSocketError creates a canonical WebSocket error.
func NewGoxSocketError(code GoxSocketErrorCode, message string, phase GoxSocketErrorPhase, event string, retryable bool, cause error) GoxSocketError {
	return GoxSocketError{
		Code:      code,
		Message:   message,
		Phase:     phase,
		Event:     event,
		Retryable: retryable,
		Cause:     cause,
		Argument:  -1,
	}
}
