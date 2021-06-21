package main

import (
	"flag"
	"log"
	"time"

	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

var (
	addr = flag.String("addr", ":8080", "TCP address to listen to")
)

func main() {
	flag.Parse()

	g := fastglue.New()
	g.ServeStatic("/{filepath:*}", ".", true)

	s := &fasthttp.Server{
		Name:         "Static File",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}
	if err := g.ListenAndServe(*addr, "", s); err != nil {
		log.Fatalf("Error in ListenAndServe: %s", err)
	}
}
