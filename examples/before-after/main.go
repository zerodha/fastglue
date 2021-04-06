package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
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
	g.Before(setTime)
	g.After(calculateTime)
	g.GET("/", handleIndex)

	s := &fasthttp.Server{
		Name:         "Before After",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	if err := g.ListenAndServe(*addr, "", s); err != nil {
		log.Fatalf("Error in ListenAndServe: %s", err)
	}
}

func handleIndex(r *fastglue.Request) error {
	name := "world!"

	return r.SendString(http.StatusOK, fmt.Sprintf("Hello %s!", name))
}

func setTime(r *fastglue.Request) *fastglue.Request {
	r.RequestCtx.SetUserValue("now", time.Now())
	return r
}

func calculateTime(r *fastglue.Request) *fastglue.Request {
	now, ok := r.RequestCtx.UserValue("now").(time.Time)
	if !ok {
		return r
	}

	log.Print("time taken", time.Since(now))

	return r
}
