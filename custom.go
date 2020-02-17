package fastglue

import (
	"encoding/json"
	"fmt"

	fasthttprouter "github.com/fasthttp/router"
	"github.com/valyala/fasthttp"
)

const (
	statusSuccess = "success"
	statusError   = "error"

	excepBadRequest = "InputException"
	excepGeneral    = "GeneralException"
)

// ErrorType defines string error constants (eg: TokenException)
// to be sent with JSON responses.
type ErrorType string

// Envelope is a highly opinionated, "standardised", JSON response
// structure.
type Envelope struct {
	Status    string      `json:"status"`
	Message   *string     `json:"message,omitempty"`
	Data      interface{} `json:"data"`
	ErrorType *ErrorType  `json:"error_type,omitempty"`
}

// NewGlue creates and returns a new instance of Fastglue with custom error
// handlers pre-bound.
func NewGlue() *Fastglue {
	f := New()
	f.Router.MethodNotAllowed = BadMethodHandler
	f.Router.NotFound = NotFoundHandler
	f.Router.SaveMatchedRoutePath = true
	f.MatchedRoutePathParam = fasthttprouter.MatchedRoutePathParam
	return f
}

// DecodeFail uses Decode() to unmarshal the Post body, but in addition to returning
// an error on failure, writes the error to the HTTP response directly. This helps
// avoid repeating read/parse/validate boilerplate inside every single HTTP handler.
func (r *Request) DecodeFail(v interface{}, tag string) error {
	if err := r.Decode(v, tag); err != nil {
		r.SendErrorEnvelope(fasthttp.StatusBadRequest,
			"Error unmarshalling request: `"+err.Error()+"`", nil, excepBadRequest)

		return err
	}

	return nil
}

// SendEnvelope is a highly opinionated method that sends success responses in a predefined
// structure which has become customary at Rainmatter internally.
func (r *Request) SendEnvelope(data interface{}) error {
	// If data is json.RawMessage, we're getting a pre-formatted JSON byte array.
	// Skip the marshaller, fake the envelope and send it right away.
	if j, ok := data.(json.RawMessage); ok {
		r.RequestCtx.SetStatusCode(fasthttp.StatusOK)
		r.RequestCtx.SetContentType(JSON)

		r.RequestCtx.Write([]byte(`{"status": "` + statusSuccess + `", "data": `))
		r.RequestCtx.Write(j)
		r.RequestCtx.Write([]byte(`}`))

		return nil
	}

	// Standard marshalled envelope.
	e := Envelope{
		Status: statusSuccess,
		Data:   data,
	}

	if err := r.SendJSON(fasthttp.StatusOK, e); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Couldn't marshal JSON: `"+err.Error()+"`", nil, excepGeneral)
	}

	return nil
}

// SendErrorEnvelope is a highly opinionated method that sends error responses in a predefined
// structure which has become customary at Rainmatter internally.
func (r *Request) SendErrorEnvelope(code int, message string, data interface{}, et ErrorType) error {
	var e Envelope
	if et == "" {
		e = Envelope{
			Status:  statusError,
			Message: &message,
			Data:    data,
		}
	} else {
		e = Envelope{
			Status:    statusError,
			Message:   &message,
			Data:      data,
			ErrorType: &et,
		}
	}

	return r.SendJSON(code, e)
}

// ReqParams is an (opinionated) middleware that checks if a given set of parameters are set in
// the GET or POST params. If not, it fails the request with an error envelope.
func ReqParams(h FastRequestHandler, fields []string) FastRequestHandler {
	return func(r *Request) error {
		var args *fasthttp.Args

		if r.RequestCtx.IsPost() || r.RequestCtx.IsPut() {
			args = r.RequestCtx.PostArgs()
		} else {
			args = r.RequestCtx.QueryArgs()
		}

		for _, f := range fields {
			if !args.Has(f) || len(args.Peek(f)) == 0 {
				r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Missing or empty field `"+f+"`", nil, excepBadRequest)
				return nil
			}
		}

		return h(r)
	}
}

// ReqLenParams is an (opinionated) middleware that checks if a given set of parameters are set in
// the GET or POST params and if each of them meets a minimum length criteria.
// If not, it fails the request with an error envelop.
func ReqLenParams(h FastRequestHandler, fields map[string]int) FastRequestHandler {
	return func(r *Request) error {
		var args *fasthttp.Args

		if r.RequestCtx.IsPost() || r.RequestCtx.IsPut() {
			args = r.RequestCtx.PostArgs()
		} else {
			args = r.RequestCtx.QueryArgs()
		}

		for f, ln := range fields {
			if !args.Has(f) || len(args.Peek(f)) < ln {
				r.SendErrorEnvelope(fasthttp.StatusBadRequest,
					fmt.Sprintf("`%s` should be minimum %d characters in length.", f, ln), nil, excepBadRequest)

				return nil
			}
		}

		return h(r)
	}
}

// ReqLenRangeParams is an (opinionated) middleware that checks if a given set of parameters are set in
// the GET or POST params and if each of them meets a minimum and maximum length range criteria.
// If not, it fails the request with an error envelop.
func ReqLenRangeParams(h FastRequestHandler, fields map[string][2]int) FastRequestHandler {
	return func(r *Request) error {
		var args *fasthttp.Args

		if r.RequestCtx.IsPost() || r.RequestCtx.IsPut() {
			args = r.RequestCtx.PostArgs()
		} else {
			args = r.RequestCtx.QueryArgs()
		}

		for f, ln := range fields {
			if !args.Has(f) || len(args.Peek(f)) < ln[0] || len(args.Peek(f)) > ln[1] {
				r.SendErrorEnvelope(fasthttp.StatusBadRequest,
					fmt.Sprintf("`%s` should be %d to %d in length", f, ln[0], ln[1]), nil, excepBadRequest)

				return nil
			}
		}

		return h(r)
	}
}

// NotFoundHandler produces an enveloped JSON response for 404 errors.
func NotFoundHandler(r *fasthttp.RequestCtx) {
	req := &Request{
		RequestCtx: r,
	}

	req.SendErrorEnvelope(fasthttp.StatusNotFound, "Route not found", nil, excepGeneral)
}

// BadMethodHandler produces an enveloped JSON response for 405 errors.
func BadMethodHandler(r *fasthttp.RequestCtx) {
	req := &Request{
		RequestCtx: r,
	}

	req.SendErrorEnvelope(fasthttp.StatusMethodNotAllowed, "Request method not allowed", nil, excepGeneral)
}
