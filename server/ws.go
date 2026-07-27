package server

import "time"

type WebSocketConfig struct {
	Enabled          bool
	HandshakeTimeout time.Duration
	MaxMessageSize   int64
	WriteTimeout     time.Duration

	Heartbeat   WebSocketHeartbeatConfig
	Queue       WebSocketQueueConfig
	Buffers     WebSocketBufferConfig
	Compression WebSocketCompressionConfig
}

type WebSocketHeartbeatConfig struct {
	PingInterval time.Duration
	PongTimeout  time.Duration
}

type WebSocketQueueConfig struct {
	MaxMessages int
	MaxBytes    int64
}

type WebSocketBufferConfig struct {
	ReadSize  int
	WriteSize int
}

type WebSocketCompressionConfig struct {
	Enabled bool
	Level   int
}
