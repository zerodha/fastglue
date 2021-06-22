# fastglue

## Overview [![Go Reference](https://pkg.go.dev/badge/github.com/zerodha/fastglue.svg)](https://pkg.go.dev/github.com/zerodha/fastglue) [![Zerodha Tech](https://zerodha.tech/static/images/github-badge.svg)](https://zerodha.tech)

fastglue is an opinionated, bare bones wrapper that glues together [fasthttp](https://github.com/valyala/fasthttp)
and [fasthttprouter](https://github.com/fasthttp/router) to act as a micro HTTP framework. It helps eliminate
boilerplate that would otherwise be required when using these two libraries to
write HTTP servers. It enables:

- Performance benefits of fasthttp + fasthttprouter.
- Pre/post middleware hooks on HTTP handlers.
- Simple middlewares for validating (existence, length range) of params in HTTP
  requests.
- Functions for unmarshalling request payloads (Form encoding, JSON, XML) into
  arbitrary structs.
- Shortcut functions for registering handlers, `GET()`, `POST()` etc.
- Shortcut for fasthttp listening on TCP and Unix sockets.
- Shortcut for graceful shutdown hook on the fasthttp server.
- Opinionated JSON API response and error structures.
- Shortcut functions for sending strings, bytes, JSON in the envelope structure
  without serialization or allocation.

## Install

```bash
go get -u github.com/zerodha/fastglue
```

## Usage

```go
import "github.com/zerodha/fastglue"
```

## Examples

- [HelloWorld Server](examples/helloworld)
- [Middleware](examples/middleware)
- [Before-after middleware](examples/before-after)
- [Decode](examples/decode)
- [Path params](examples/path)
- [Serve static file](examples/statiic-file)
- [Singleton](examples/singleton)
- [Graceful shutdown](examples/graceful)
