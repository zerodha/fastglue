package fastglue

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"

	"github.com/buaazp/fasthttprouter"
	"github.com/valyala/fasthttp"
)

var (
	constJSON = []byte("json")
	constXML  = []byte("xml")
)

const (
	// JSON is an alias for the JSON content type
	JSON = "application/json"

	// XML is an alias for the XML content type
	XML = "application/xml"

	// PLAINTEXT is an alias for the plaintext content type
	PLAINTEXT = "text/plain"
)

// FastRequestHandler is the fastglue HTTP request handler function
// that wraps over the fasthttp handler.
type FastRequestHandler func(*Request) error

// FastMiddleware is the fastglue middleware handler function
// that can be registered using Before() and After() functions.
type FastMiddleware func(*Request) *Request

// Request is a wrapper over fasthttp's RequestCtx that's injected
// into request handlers.
type Request struct {
	RequestCtx *fasthttp.RequestCtx
	Context    interface{}
}

// Fastglue is the "glue" wrapper over fasthttp and fasthttprouter.
type Fastglue struct {
	Router      *fasthttprouter.Router
	context     interface{}
	contentType string

	before []FastMiddleware
	after  []FastMiddleware
}

// New creates and returns a new instance of Fastglue.
func New() *Fastglue {
	return &Fastglue{
		Router: fasthttprouter.New(),
		before: make([]FastMiddleware, 0),
		after:  make([]FastMiddleware, 0),
	}
}

// ListenAndServe is a wrapper for fasthttp.ListenAndServe. It takes a TCP address
// and an optional UNIX socket file path and starts listeners.
func (f *Fastglue) ListenAndServe(address string, socket string) error {
	if address == "" || (address == "" && socket == "") {
		panic("Either a TCP address with an a optional UNIX socket path are required to start the server")
	}

	if socket != "" {
		go func() {
			err := fasthttp.ListenAndServeUNIX(socket, 0666, f.Handler())
			if err != nil {
				panic(fmt.Sprintf("Error opening socket: %v", err))
			}
		}()
	}

	return fasthttp.ListenAndServe(address, f.Handler())
}

// hanlder is the "proxy" abstraction that converts a fastglue handler into
// a fasthttp handler and passes execution in and out.
func (f *Fastglue) handler(h FastRequestHandler) func(*fasthttp.RequestCtx) {
	return func(ctx *fasthttp.RequestCtx) {
		req := &Request{
			RequestCtx: ctx,
			Context:    f.context,
		}

		// Apply "before" middleware.
		for _, p := range f.before {
			if p(req) == nil {
				return
			}
		}

		h(req)

		// Apply "after" middleware.
		for _, p := range f.after {
			if p(req) == nil {
				return
			}
		}

	}
}

// Handler returns fastglue's central fasthttp handler that can be registered
// to a fasthttp server instance.
func (f *Fastglue) Handler() func(*fasthttp.RequestCtx) {
	return f.Router.Handler
}

// SetContext sets a "context" which is shared and made available in every HTTP request.
// This is useful for injecting dependencies such as config structs, DB connections etc.
// Be very careful to only include immutable variables and thread-safe objects.
func (f *Fastglue) SetContext(c interface{}) {
	f.context = c
}

// Before registers a fastglue middleware that's executed before an HTTP request
// is handed over to the registered handler. This is useful for doing "global"
// checks, for instance, session and cookies.
func (f *Fastglue) Before(h FastMiddleware) {
	f.before = append(f.before, h)
}

// After registers a fastglue middleware that's executed after a registered handler
// has finished executing. This is useful to do things like central request logging.
func (f *Fastglue) After(h FastMiddleware) {
	f.after = append(f.after, h)
}

// POST is fastglue's wrapper over fasthttprouter's handler.
func (f *Fastglue) POST(path string, h FastRequestHandler) {
	f.Router.POST(path, f.handler(h))
}

// GET is fastglue's wrapper over fasthttprouter's handler.
func (f *Fastglue) GET(path string, h FastRequestHandler) {
	f.Router.GET(path, f.handler(h))
}

// PUT is fastglue's wrapper over fasthttprouter's handler.
func (f *Fastglue) PUT(path string, h FastRequestHandler) {
	f.Router.PUT(path, f.handler(h))
}

// DELETE is fastglue's wrapper over fasthttprouter's handler.
func (f *Fastglue) DELETE(path string, h FastRequestHandler) {
	f.Router.DELETE(path, f.handler(h))
}

// OPTIONS is fastglue's wrapper over fasthttprouter's handler.
func (f *Fastglue) OPTIONS(path string, h FastRequestHandler) {
	f.Router.OPTIONS(path, f.handler(h))
}

// HEAD is fastglue's wrapper over fasthttprouter's handler.
func (f *Fastglue) HEAD(path string, h FastRequestHandler) {
	f.Router.HEAD(path, f.handler(h))
}

// Decode unmarshals the Post body of a fasthttp request based on the ContentType header
// into value pointed to by v, as long as the content is JSON or XML.
func (r *Request) Decode(v interface{}) error {
	var (
		err error
		ct  = r.RequestCtx.Request.Header.ContentType()
	)

	if bytes.Contains(ct, constJSON) {
		err = json.Unmarshal(r.RequestCtx.PostBody(), &v)
	} else if bytes.Contains(ct, constXML) {
		err = xml.Unmarshal(r.RequestCtx.PostBody(), &v)
	} else {
		return errors.New("Unknown encoding: " + string(ct))
	}

	if err != nil {
		return errors.New("Error decoding: " + err.Error())
	}

	return nil
}

// SendBytes writes a []byte payload to the HTTP response and also
// sets a given ContentType header.
func (r *Request) SendBytes(code int, ctype string, v []byte) error {
	r.RequestCtx.SetStatusCode(code)
	r.RequestCtx.SetContentType(ctype)
	r.RequestCtx.Write(v)

	return nil
}

// SendString writes a string payload to the HTTP response.
// It implicitly sets ContentType to plain/text.
func (r *Request) SendString(code int, v string) error {
	r.RequestCtx.SetStatusCode(code)
	r.RequestCtx.SetContentType("text/plain")
	r.RequestCtx.WriteString(v)

	return nil
}

// SendJSON takes an interface, marshals it to JSON, and writes the
// result to the HTTP response. It implicitly sets ContentType to application/json.
func (r *Request) SendJSON(code int, v interface{}) error {
	r.RequestCtx.SetStatusCode(code)
	r.RequestCtx.SetContentType(JSON)

	var (
		b   []byte
		err error
	)

	if b, err = json.Marshal(v); err == nil {
		r.RequestCtx.Write(b)
		return nil
	}

	return err
}
