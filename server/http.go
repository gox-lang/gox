package server

type HTTPConfig struct {
	TLS                 TLSConfig
	Host                string
	Port                int
	MethodsAllowed      []string
	Compression         CompressionConfig
	ResolverIPForwarded ResolverIPForwardedConfig
}

type ResolverIPForwardedConfig struct {
	TrustedProxies      []string
	Headers             []string
	RequireTrustedProxy bool
}
