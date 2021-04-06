package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/valyala/fasthttp"
	"REDACTED/fastglue"
)

var (
	addr = flag.String("addr", ":8080", "TCP address to listen to")
)

func main() {
	flag.Parse()

	g := fastglue.New()
	g.GET("/", handleHelloWorld)

	s := &fasthttp.Server{
		Name:         "HelloWorld",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}
	if err := g.ListenAndServe(*addr, "", s); err != nil {
		log.Fatalf("Error in ListenAndServe: %s", err)
	}
}

func handleHelloWorld(r *fastglue.Request) error {
	fmt.Fprintf(r.RequestCtx, "Hello, world!\n\n")

	fmt.Fprintf(r.RequestCtx, "Request method is %q\n", r.RequestCtx.Method())
	fmt.Fprintf(r.RequestCtx, "RequestURI is %q\n", r.RequestCtx.RequestURI())
	fmt.Fprintf(r.RequestCtx, "Requested path is %q\n", r.RequestCtx.Path())
	fmt.Fprintf(r.RequestCtx, "Host is %q\n", r.RequestCtx.Host())
	fmt.Fprintf(r.RequestCtx, "Query string is %q\n", r.RequestCtx.QueryArgs())
	fmt.Fprintf(r.RequestCtx, "User-Agent is %q\n", r.RequestCtx.UserAgent())
	fmt.Fprintf(r.RequestCtx, "Connection has been established at %s\n", r.RequestCtx.ConnTime())
	fmt.Fprintf(r.RequestCtx, "Request has been started at %s\n", r.RequestCtx.Time())
	fmt.Fprintf(r.RequestCtx, "Serial request number for the current connection is %d\n", r.RequestCtx.ConnRequestNum())
	fmt.Fprintf(r.RequestCtx, "Your ip is %q\n\n", r.RequestCtx.RemoteIP())

	fmt.Fprintf(r.RequestCtx, "Raw request is:\n---CUT---\n%s\n---CUT---", &r.RequestCtx.Request)

	r.RequestCtx.SetContentType("text/plain; charset=utf8")

	// Set arbitrary headers
	r.RequestCtx.Response.Header.Set("X-My-Header", "my-header-value")

	// Set cookies
	var c fasthttp.Cookie
	c.SetKey("cookie-name")
	c.SetValue("cookie-value")
	r.RequestCtx.Response.Header.SetCookie(&c)
	return nil
}
