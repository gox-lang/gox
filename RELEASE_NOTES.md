# GOX SDK v1.1.0

This release prepares the SDK for automatic and manual HTTPS support and corrects
writer method signatures to follow Go's standard interfaces.

## TLS configuration

- Adds `server.HTTPConfig.TLS` and `server.TLSConfig`.
- Defines automatic and manual modes, certificate domains, an independent HTTPS
  port, optional ACME contact email, manual certificate/key paths, and HTTP redirects.
- Accepts DNS providers directly through `server.DNSProvider`, composed of
  `libdns.RecordAppender` and `libdns.RecordDeleter`. Applications can configure
  compatible providers without a provider registry in GOX.
- Uses `libdns v1.1.1`; the SDK does not bundle CertMagic or DNS providers.

**This release defines the configuration contract only.** HTTPS listeners,
certificate issuance, renewal, and scaffold support require the corresponding
gox-server integration. See [TLS configuration](https://github.com/gox-lang/gox/blob/v1.1.0/TLS.md).

## Writer fixes and migration

`Writer.WriteByte(byte)` now returns `error`, matching `io.ByteWriter`.
Custom writers must return nil on success and the write error on failure.
Internal client-island helpers now check byte-write errors and panic with the
original error so the server's request recovery can handle it.

```go
if err := writer.WriteByte(value); err != nil {
    panic(err) // Internal rendering path handled by request recovery.
}
```

`Body.WriteTo(io.Writer)` now returns `(int64, error)`, matching `io.WriterTo`.
It preserves the byte count on partial writes and close errors, and reports
`io.ErrShortWrite` for incomplete cached-body writes without an underlying error.

Update error-only calls as follows:

```go
if _, err := body.WriteTo(destination); err != nil {
    // Handle the error.
}
```

## Compatibility

These signature corrections are breaking API changes made before GOX's public
launch. Version v1.1.0 deliberately keeps the existing module path:

```go
import "github.com/gox-lang/gox"
```

This is a pre-launch exception to normal semantic-versioning compatibility
expectations. Direct SDK consumers must update affected calls and implementations.
GOX Server must pin the matching SDK version, adapt its writers, and rebuild
plugins before adopting this release. Existing servers should continue using
their compatible SDK version until that integration is complete.

The SDK's declared minimum remains Go 1.23. The planned server integration with
CertMagic v0.25.6 requires Go 1.26 or newer.

## Validation

- SDK build, tests, and race tests pass on Go 1.27.1.
- `go vet ./...` passes without warnings.
- Regression tests cover byte-write error propagation, cached and streamed body
  writes, short writes, close errors, empty bodies, and nil destinations.
- A local integration check compiled Cloudflare v0.2.2 configuration as a Go plugin,
  loaded it into a host using CertMagic v0.25.6, and verified DNS-provider
  compatibility. It did not request certificates or modify DNS records.
