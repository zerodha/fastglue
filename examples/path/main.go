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
	g.GET("/", handleIndex)
	g.GET("/{name:^[a-zA-Z]+$}", handleIndex)

	s := &fasthttp.Server{
		Name:         "Path params",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}
	if err := g.ListenAndServe(*addr, "", s); err != nil {
		log.Fatalf("Error in ListenAndServe: %s", err)
	}
}

func handleIndex(r *fastglue.Request) error {
	var (
		name, _ = r.RequestCtx.UserValue("name").(string)
	)

	if name == "" {
		name = "world!"
	}

	return r.SendString(http.StatusOK, fmt.Sprintf("Hello %s!", name))
}
