# fastglue

Fastglue is a "glue" wrapper over [fasthttp](https://github.com/valyala/fasthttp) and [fasthttprouter](https://github.com/fasthttp/router).

Currently, fastglue is successfully used at [Zerodha](https://zerodha.com) in production serving millions of requests per second.

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
