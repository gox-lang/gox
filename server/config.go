package server

type Config struct {
	ProjectName   string
	Dev           DevConfig
	UnmarshalText map[string]func(string) (any, error)
	Cluster       ClusterAdapter

	HTTP      HTTPConfig
	RateLimit RateLimitConfig
	Session   SessionConfig
	Static    StaticConfig
	Routes    RoutesConfig
	WebSocket WebSocketConfig

	OnStartServer func() error
	OnStopServer  func() error
}

type DevConfig struct {
	Enabled bool
	Logs    DevLogsConfig
}

type DevLogsConfig struct {
	Static    bool
	GoxFile   bool
	Trace     bool
	TraceDeep bool
}
