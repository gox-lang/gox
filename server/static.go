package server

type StaticConfig struct {
	Path              string
	StreamMinFileSize int64
	Compression       CompressionConfig
	Headers           []StaticHeaderRule
	Range             []StaticRangeRule
}

type CompressionConfig struct {
	Enabled       bool
	OrderPriority []string
	Gzip          CompressionAlgorithmConfig
	Brotli        CompressionAlgorithmConfig
	Deflate       CompressionAlgorithmConfig
	Zstd          CompressionAlgorithmConfig
}

type CompressionAlgorithmConfig struct {
	Enabled bool
	Level   int
}

func DefaultStaticCompressionConfig() CompressionConfig {
	return CompressionConfig{
		Enabled: true,
		OrderPriority: []string{
			"brotli",
			"gzip",
			"zstd",
			"deflate",
		},
		Gzip: CompressionAlgorithmConfig{
			Enabled: true,
			Level:   6,
		},
		Brotli: CompressionAlgorithmConfig{
			Enabled: true,
			Level:   7,
		},
		Deflate: CompressionAlgorithmConfig{
			Enabled: true,
			Level:   6,
		},
		Zstd: CompressionAlgorithmConfig{
			Enabled: true,
			Level:   5,
		},
	}
}

func DefaultHTTPCompressionConfig() CompressionConfig {
	return CompressionConfig{
		Enabled: true,
		OrderPriority: []string{
			"zstd",
			"gzip",
			"brotli",
			"deflate",
		},
		Gzip: CompressionAlgorithmConfig{
			Enabled: true,
			Level:   3,
		},
		Brotli: CompressionAlgorithmConfig{
			Enabled: true,
			Level:   3,
		},
		Deflate: CompressionAlgorithmConfig{
			Enabled: true,
			Level:   3,
		},
		Zstd: CompressionAlgorithmConfig{
			Enabled: true,
			Level:   1,
		},
	}
}

type Headers map[string]string

type StaticHeaderRule struct {
	Match   string
	Headers Headers
}

type StaticRangeRule struct {
	Match        string
	Enabled      bool
	MinFileSize  int64
	MaxChunkSize int64
}
