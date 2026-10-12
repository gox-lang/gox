package server

import "github.com/libdns/libdns"

// DNSProvider is the DNS record contract required for automatic DNS-01
// validation. Providers must preserve unrelated records, support concurrent
// challenges, and honor context cancellation.
type DNSProvider interface {
	libdns.RecordAppender
	libdns.RecordDeleter
}

// TLSConfig describes HTTPS configuration for the GOX server runtime.
// The SDK defines this contract; certificate management is implemented by
// gox-server. The zero value leaves TLS disabled.
type TLSConfig struct {
	Enabled bool

	// Type is "auto" or "manual". An empty value means "auto".
	Type string

	// Port is the HTTPS listener port, independent of HTTPConfig.Port.
	// Zero means 443.
	Port int

	// Domains lists the certificate names managed in auto mode.
	// Wildcards require a DNS provider.
	Domains []string

	// Email is the optional ACME account contact in auto mode.
	Email string

	// CertFile and KeyFile are the PEM certificate chain and private key
	// paths required in manual mode.
	CertFile string
	KeyFile  string

	// DNS selects DNS-01 validation in auto mode. Nil selects HTTP-01.
	// The provider must be non-nil when supplied and is invalid in manual mode.
	DNS DNSProvider

	// RedirectHTTP redirects ordinary HTTP requests to HTTPS.
	// It does not disable ACME validation when false.
	RedirectHTTP bool
}
