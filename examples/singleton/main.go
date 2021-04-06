package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/valyala/fasthttp"
	"REDACTED/fastglue"
)

var (
	addr = flag.String("addr", ":8080", "TCP address to listen to")
)

// App singleton.
type App struct {
	version string
	log     *log.Logger
}

func main() {
	flag.Parse()

	app := &App{
		version: "0.1.0",
		log:     log.New(os.Stdout, "SINGLETON", log.Llongfile),
	}

	g := fastglue.New()
	g.SetContext(app)
	g.GET("/", handleIndex)

	s := &fasthttp.Server{
		Name:         "Singleton",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}
	if err := g.ListenAndServe(*addr, "", s); err != nil {
		log.Fatalf("Error in ListenAndServe: %s", err)
	}
}

func handleIndex(r *fastglue.Request) error {
	var (
		app = r.Context.(*App)
	)

	app.log.Printf(fmt.Sprintf("index called. user-agent:%s, args: %q", r.RequestCtx.UserAgent(), r.RequestCtx.QueryArgs()))

	return r.SendEnvelope(map[string]string{
		"version": app.version,
	})
}
