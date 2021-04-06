# fastglue
Fastglue is a "glue" wrapper over fasthttp and fasthttprouter. 

Currently, fastglue is successfully being used at [Zerodha](https://REDACTED) in production serving millions of requests per second.

## Install
```bash
go get -u REDACTED/fastglue
```

## Usage
```go
import "REDACTED/fastglue"
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
