package main

import (
	"log"

	"github.com/REDACTED/fastglue"
	"github.com/valyala/fasthttp"
)

// App is the global config "context" that'll be injected into every Request.
// Be extremely careful with this and make sure tha values are immutable
// and that all objects (eg: Redis, Postgres etc.) are goroutine safe.
type App struct {
	version string

	// Redis and other DB connection objects can go here.
}

// Custom ErrorTypes for Rainmatter's "error_type" JSON response field.
var (
	TokenExc fastglue.ErrorType = "TokenException"
	InputExc fastglue.ErrorType = "InputException"
)

// Person is a JSON data payload we'll accept.
type Person struct {
	Name    string `json:"name"`
	Age     int    `json:"age"`
	Comment string `json:"comment"`
	Version string `json:"version"`
}

// This "Before()" middleware checks if a 'token' param is set with the value '123'.
func checkToken(r *fastglue.Request) *fastglue.Request {
	if string(r.RequestCtx.FormValue("token")) != "123" {
		r.SendErrorEnvelope(fasthttp.StatusBadRequest, "You haven't sent the token with the value '123'!", nil, TokenExc)

		return nil
	}

	return r
}

func myPOSThandler(r *fastglue.Request) error {
	var p Person
	if err := r.DecodeFail(&p); err != nil {
		return err
	}

	if p.Age < 18 {
		r.SendErrorEnvelope(fasthttp.StatusBadRequest, "We only accept Persons above 18", struct {
			Random string `json:"random"`
		}{"Some random error payload"}, InputExc)

		return nil
	}

	p.Comment = "Here's a comment the server added!"

	// Get the version from the injected app context.
	p.Version = r.Context.(*App).version

	return r.SendEnvelope(p)
}

func myGEThandler(r *fastglue.Request) error {
	return r.SendEnvelope(struct {
		Something string `json:"something"`
	}{"You said your name is: " + string(r.RequestCtx.FormValue("name"))})
}

func main() {
	f := fastglue.NewGlue()
	f.SetContext(&App{version: "v3.0.0"})
	f.Before(checkToken)

	f.POST("/post", myPOSThandler)
	f.GET("/get", fastglue.RequireParams(myGEThandler, []string{"name"}))

	address := ":8080"
	log.Println("Listening on", address)
	f.ListenAndServe(address, "")

	// fasthttp can be invoked directly like this as well:
	// fasthttp.ListenAndServe(address, f.Handler())
}
