package fastglue

import (
	"encoding/json"

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
	Message   *string     `json:"message"`
	Data      interface{} `json:"data"`
	ErrorType *ErrorType  `json:"error_type,omitempty"`
}

// NewGlue creates and returns a new instance of Fastglue with custom error
// handlers pre-bound.
func NewGlue() *Fastglue {
	f := New()
	f.Router.MethodNotAllowed = BadMethodHandler
	f.Router.NotFound = NotFoundHandler

	return f
}

// DecodeFail uses Decode() to unmarshal the Post body, but in addition to returning
// an error on failure, writes the error to the HTTP response directly. This helps
// avoid repeating read/parse/validate boilerplate inside every single HTTP handler.
func (r *Request) DecodeFail(v interface{}) error {
	if err := r.Decode(v); err != nil {
		r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid POST body: `"+err.Error()+"`", nil, excepBadRequest)

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

		r.RequestCtx.Write([]byte(`{"status": "` + statusSuccess + `", "message": null, "data": `))
		r.RequestCtx.Write(j)
		r.RequestCtx.Write([]byte(`}`))

		return nil
	}

	// Standard marshalled envelope.
	e := Envelope{
		Status:  statusSuccess,
		Message: nil,
		Data:    data,
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

// RequireParams is an (opinionated) middleware that checks if a given set of parameters are set in
// the GET or POST params. If not, it fails the request with an error envelop.
func RequireParams(h FastRequestHandler, fields []string) FastRequestHandler {
	return func(r *Request) error {
		for _, f := range fields {
			if (!r.RequestCtx.PostArgs().Has(f) && !r.RequestCtx.QueryArgs().Has(f)) &&
				(len(r.RequestCtx.PostArgs().Peek(f)) == 0 || len(r.RequestCtx.QueryArgs().Peek(f)) == 0) {
				r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Missing or empty field `"+f+"`", nil, excepBadRequest)
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
