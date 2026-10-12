# TLS configuration contract

This SDK defines the configuration contract. It does not start HTTPS listeners,
obtain certificates, or renew them. Those features require a gox-server runtime
implementing this contract.

## Scaffold

Inside server.HTTPConfig:

```go
TLS: server.TLSConfig{
    Enabled:      false,
    Type:         "auto",
    Domains:      []string{},
    RedirectHTTP: true,
},
```

Disabled TLS preserves HTTP behavior. An omitted Type means "auto"; an omitted
TLS Port means 443. HTTPConfig.Port remains the independent HTTP listener port.
RedirectHTTP uses the Go zero value (false); the scaffold explicitly enables it.

## Automatic certificates

With no DNS provider, auto mode selects HTTP-01. With a provider, it selects
DNS-01. Domains are required; wildcard domains require DNS-01. Email is optional.
CertFile and KeyFile belong to manual mode and must be omitted in auto mode.

Certificate and ACME account storage is managed by the server under
<GOX_HOME>/.tls/, outside build outputs and toolchains. This state must persist
across restarts and container replacement.

Example imports:

```go
import (
    os "os"
    server "github.com/gox-lang/gox/server"
    cloudflare "github.com/libdns/cloudflare"
)
```

Example field within server.HTTPConfig:

```go
TLS: server.TLSConfig{
    Enabled: true,
    Type:    "auto",
    Domains: []string{"example.com", "*.example.com"},
    DNS: &cloudflare.Provider{
        APIToken: os.Getenv("CLOUDFLARE_API_TOKEN"),
    },
    RedirectHTTP: true,
},
```

DNSProvider composes libdns.RecordAppender and libdns.RecordDeleter. Compatible
providers can be imported by applications without a provider registry in GOX.
Provider versions must use compatible libdns interfaces. Server and compiled
configuration plugins must also share compatible dependency versions and builds.
A supplied provider must not be a typed nil pointer.

## Manual certificates

```go
TLS: server.TLSConfig{
    Enabled:      true,
    Type:         "manual",
    CertFile:     "/certs/fullchain.pem",
    KeyFile:      "/certs/privkey.pem",
    RedirectHTTP: true,
},
```

Both PEM paths are required. DNS is invalid in manual mode. Domains and Email
are unnecessary; the certificate contains its names. Manual certificates are
not renewed through ACME.

The runtime must reject unknown modes, invalid ports, conflicting mode-specific
fields, and incomplete enabled configurations. ACME HTTP validation must remain
available independently of RedirectHTTP.
