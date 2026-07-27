package gox

// ChannelErrorCode is a stable logical-channel error identifier shared by Go
// and JavaScript.
type ChannelErrorCode string

const (
	ChannelInvalidState   ChannelErrorCode = "INVALID_STATE"
	ChannelProtocolError  ChannelErrorCode = "PROTOCOL_ERROR"
	ChannelHandlerFailed  ChannelErrorCode = "HANDLER_FAILED"
	ChannelEncodeFailed   ChannelErrorCode = "ENCODE_FAILED"
	ChannelTransportError ChannelErrorCode = "TRANSPORT_ERROR"
)

// ChannelError describes a failure isolated to one logical channel instance.
type ChannelError struct {
	Code      ChannelErrorCode
	ChannelID string
	Event     string
	Message   string
	Retryable bool
	Cause     error
}

func (err ChannelError) Error() string { return err.Message }

func (err ChannelError) Unwrap() error { return err.Cause }

// ChannelDisconnectReason describes why one socket left a logical channel.
// It does not describe the lifetime of the underlying WebSocket connection.
type ChannelDisconnectReason string

const (
	ChannelDisconnectLeft               ChannelDisconnectReason = "left"
	ChannelDisconnectRemoved            ChannelDisconnectReason = "removed"
	ChannelDisconnectRejected           ChannelDisconnectReason = "rejected"
	ChannelDisconnectSocketDisconnected ChannelDisconnectReason = "socket-disconnected"
	ChannelDisconnectClosed             ChannelDisconnectReason = "closed"
	ChannelDisconnectServerStopped      ChannelDisconnectReason = "server-stopped"
)
