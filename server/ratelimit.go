package server

import "time"

const RateLimitAll = "ALL"

type RateLimitFilter string

const (
	RateLimitFilterIP      RateLimitFilter = "ip"
	RateLimitFilterSession RateLimitFilter = "session"
	RateLimitFilterRoute   RateLimitFilter = "route"
	RateLimitFilterMethod  RateLimitFilter = "method"
	RateLimitFilterURL     RateLimitFilter = "url"
)

type RateLimitConfig struct {
	Enabled bool
	Rules   []RateLimitRule
}

type RateLimitRule struct {
	Name    string
	Enabled bool
	Methods []string
	Match   string

	// Pattern is kept as a compatibility fallback. Prefer Match.
	Pattern string
	Limits  []RateLimitLimit
}

type RateLimitLimit struct {
	Name     string
	Enabled  bool
	By       []RateLimitFilter
	Strategy RateLimitStrategy
}

type RateLimitStrategy interface {
	rateLimitStrategy()
}

type RateLimitFixedWindow struct {
	Limit  int
	Window time.Duration
}

func (RateLimitFixedWindow) rateLimitStrategy() {}

type RateLimitSlidingWindow struct {
	Limit  int
	Window time.Duration
}

func (RateLimitSlidingWindow) rateLimitStrategy() {}

type RateLimitTokenBucket struct {
	Every  time.Duration
	Refill int
	Burst  int
}

func (RateLimitTokenBucket) rateLimitStrategy() {}

type RateLimitLeakyBucket struct {
	Every    time.Duration
	Drain    int
	Capacity int
}

func (RateLimitLeakyBucket) rateLimitStrategy() {}
