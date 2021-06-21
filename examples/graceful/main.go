package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
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
		Name:         "Graceful shutdown",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}
	var (
		ch  = make(chan struct{}, 1)
		sig = make(chan os.Signal, 1)
	)

	signal.Notify(sig, os.Interrupt, syscall.SIGALRM, syscall.SIGABRT)

	go func() {
		<-sig
		ch <- struct{}{}
	}()

	if err := g.ListenServeAndWaitGracefully(*addr, "", s, ch); err != nil {
		log.Fatalf("Error in ListenAndServe: %s", err)
	}
}
