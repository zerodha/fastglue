package main

import (
	"flag"
	"log"
	"net/http"
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
	g.GET("/", handleIndex)

	s := &fasthttp.Server{
		Name:         "Decode",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}
	if err := g.ListenAndServe(*addr, "", s); err != nil {
		log.Fatalf("Error in ListenAndServe: %s", err)
	}
}

type request struct {
	Name     string `json:"name" url:"name"`
	EMail    string `json:"email" url:"email"`
	DOB      string `json:"date_of_birth" url:"dob"`
	Location string `json:"location" url:"location"`
}

func handleIndex(r *fastglue.Request) error {
	var (
		req request
	)

	if err := r.Decode(&req, "url"); err != nil {
		return r.SendErrorEnvelope(http.StatusBadRequest, "decode failed", err.Error(), "InputError")
	}

	return r.SendEnvelope(req)
}
