# GOX SDK

The official Go SDK for GOX.

`github.com/gox-lang/gox` defines the public Go API shared between GOX applications and GOX Server. It provides the types and runtime contracts used by pages, actions, sessions, WebSockets, server APIs, testing, configuration, and other GOX features.

## About GOX

GOX is a complete web server built on Go, designed to keep application development as close to Go as possible while integrating technologies, structures, and tooling from modern web development.

GOX keeps Go at the core of server-side development and complements its model with concepts inspired by web technologies such as JSX and ECMAScript Modules. It also provides frontend application integration and support for React, Preact, Solid, Vue, and Svelte, allowing Go backends and client-side code to coexist within the same architecture.

GOX applications use `.gox` files to create filesystem-based pages, server components, actions, client components, and other web application features while preserving Go's language, type system, packages, modules, and tooling.

A minimal GOX page looks like this:

```gox
module index

import "github.com/gox-lang/gox"

func GET(ctx *gox.Context) gox.Page {
    return <h1>Hello GOX</h1>
}
```

GOX Server compiles and runs GOX applications and provides the runtime implementation behind the APIs exposed by this module.

## Visual Studio Code

**GOX Tools** is the official VS Code extension for GOX. It provides syntax
highlighting, snippets, visibility hints, GOX Dark and GOX Light themes, and
language features powered by the `gox ls` language server: completion,
diagnostics, and navigation.

Install [GOX Tools from Visual Studio Marketplace](https://marketplace.visualstudio.com/items?itemName=gox-lang.gox-tools),
or run this command with the VS Code CLI:

```sh
code --install-extension gox-lang.gox-tools
```

With GOX Tools 0.1.4 or newer, the extension automatically locates GOX or
installs an official release for the extension host. No separate Go installation
is required: `gox ls` prepares its managed Go toolchain. Existing installations
and a custom `gox.languageServer.path` are respected.

New VS Code terminals receive the GOX executable directory in `PATH`. Managed
installations receive stable GOX updates through the language server; running
applications are not restarted. Use `gox update` for an explicit update.

VS Code 1.91 or newer is required. Windows supports the native language server;
use VS Code connected to WSL for application development and execution.

## Packages

This module currently exposes three public Go packages.

### `github.com/gox-lang/gox`

The main API for GOX applications.

It contains the public types and APIs used directly by GOX applications, including pages, actions, context, files, sessions, storage, WebSockets, server access, and related runtime functionality.

```go
import "github.com/gox-lang/gox"
```

### `github.com/gox-lang/gox/server`

Public server contracts and configuration types used by the GOX runtime and server integrations.

```go
import "github.com/gox-lang/gox/server"
```

### `github.com/gox-lang/gox/test`

Public APIs for testing GOX applications.

```go
import "github.com/gox-lang/gox/test"
```

## TLS configuration

TLS and DNS provider documentation is maintained in `gox-docs`, in
`config/http.md`. Runtime support is supplied by GOX Server.

## Installation

GOX Server automatically manages the GOX SDK as part of its toolchain. GOX projects do not need to install or manage the SDK manually.

To use the Go module directly:

```bash
go get github.com/gox-lang/gox@latest
```

## GOX Server

This repository contains the **GOX SDK**, not the GOX Server executable.

GOX Server provides the CLI, compiler, runtime, development server, client tooling, and infrastructure required to build and run GOX applications.

Official website:

[https://gox-lang.dev](https://gox-lang.dev)

Repository:

[github.com/gox-lang/gox-server](https://github.com/gox-lang/gox-server)

## Documentation

GOX SDK documentation is available in the official documentation repository:

[github.com/gox-lang/gox-docs](https://github.com/gox-lang/gox-docs)

or on the official website:

[https://gox-lang.dev/docs/sdk](https://gox-lang.dev/docs/sdk)

## Versioning

GOX follows Semantic Versioning.

GOX Server automatically manages its toolchain, including the version of the GOX SDK used by each project. Developers do not need to manually select, install, or maintain a compatible SDK version.

When the SDK needs to be updated, GOX keeps it aligned with the toolchain and the corresponding GOX Server version.

## Contributing

GOX currently accepts contributions primarily in two ways:

- **Bug reports:** reproducible and properly documented reports help identify and resolve issues in GOX. When a report directly contributes to the resolution of a bug, its author will be recognized as a contributor.
- **Proposals:** ideas for new features, APIs, or syntax may be submitted for consideration. Accepted proposals will be evaluated, designed, and implemented by the maintainers to ensure they fit the architecture, consistency, and philosophy of GOX. The author of the proposal will be recognized for their contribution.

See [CONTRIBUTING.md](CONTRIBUTING.md) for the current contribution process and available ways to contribute.

## Security

For information about reporting security vulnerabilities, see [SECURITY.md](SECURITY.md).

## License

GOX is released under the MIT License.

See [LICENSE](LICENSE) for details.
