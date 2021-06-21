package main

import (
	"crypto/subtle"
	"flag"
	"log"
	"net/http"
	"regexp"
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
	g.GET("/", auth(validateAll(handleGetAll)))
	g.PUT("/", auth(fastglue.ReqLenParams(validate(handleMiddleware), map[string]int{"a": 5, "b": 5})))
	g.POST("/", auth(fastglue.ReqParams(validate(handleMiddleware), []string{"a", "b"})))

	s := &fasthttp.Server{
		Name:         "Middleware",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}
	if err := g.ListenAndServe(*addr, "", s); err != nil {
		log.Fatalf("Error in ListenAndServe: %s", err)
	}
}

var (
	rxAlphaNum    = regexp.MustCompile("^([a-zA-Z0-9])+$")
	rxAlphaNumLen = regexp.MustCompile("^[a-zA-Z0-9]{4,100}$")
)

func isAlphanum(input string) bool {
	return rxAlphaNum.MatchString(input)
}
func isAlphanumLen(input string) bool {
	return rxAlphaNumLen.MatchString(input)
}

func validate(h fastglue.FastRequestHandler) fastglue.FastRequestHandler {
	return func(r *fastglue.Request) error {
		a := string(r.RequestCtx.PostArgs().Peek("a"))
		b := string(r.RequestCtx.PostArgs().Peek("b"))
		if !isAlphanum(a) || !isAlphanum(b) {
			return r.SendErrorEnvelope(http.StatusBadRequest, "validation failed", nil, "ValidationError")
		}

		return h(r)
	}
}

func validateAll(h fastglue.FastRequestHandler) fastglue.FastRequestHandler {
	return func(r *fastglue.Request) error {
		var invalid bool
		r.RequestCtx.QueryArgs().VisitAll(func(k, v []byte) {
			if !isAlphanum(string(k)) || !isAlphanumLen(string(v)) {
				invalid = true
			}
		})
		if invalid {
			return r.SendErrorEnvelope(http.StatusBadRequest, "validation failed", nil, "ValidationError")
		}

		return h(r)
	}
}

func handleMiddleware(r *fastglue.Request) error {
	var out = map[string]interface{}{
		"a": string(r.RequestCtx.PostArgs().Peek("a")),
		"b": string(r.RequestCtx.PostArgs().Peek("b")),
	}

	return r.SendEnvelope(out)
}

func handleGetAll(r *fastglue.Request) error {
	var out = map[string]interface{}{"name": "fastglue"}

	r.RequestCtx.QueryArgs().VisitAll(func(k, v []byte) {
		out[string(k)] = string(v)
	})

	return r.SendJSON(http.StatusOK, out)
}

var (
	username = []byte("admin")
	password = []byte("pass")
	realm    = "Please enter your username and password for this site"
)

func auth(h fastglue.FastRequestHandler) fastglue.FastRequestHandler {
	return func(r *fastglue.Request) error {
		un, pw, err := r.ParseAuthHeader(fastglue.AuthBasic)
		if err != nil {
			r.RequestCtx.Response.Header.Set("WWW-Authenticate", `Basic realm="`+realm+`"`)
			return r.SendBytes(http.StatusUnauthorized, fastglue.PLAINTEXT, []byte("Unauthorizaed\n"))
		}
		if subtle.ConstantTimeCompare(un, []byte(username)) != 1 || subtle.ConstantTimeCompare(pw, []byte(password)) != 1 {
			r.RequestCtx.Response.Header.Set("WWW-Authenticate", `Basic realm="`+realm+`"`)
			return r.SendBytes(http.StatusUnauthorized, fastglue.PLAINTEXT, []byte("Unauthorizaed\n"))
		}

		return h(r)
	}
}
