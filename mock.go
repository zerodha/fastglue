package fastglue

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"
)

// MockServer is a mock HTTP server. It uses an httptest.Server mock server
// that can take an HTTP request and respond with a mock response.
type MockServer struct {
	Server  *httptest.Server
	handles map[string]MockResponse
}

// MockResponse represents a mock response produced by the mock server.
type MockResponse struct {
	StatusCode  int
	ContentType string
	Body        []byte
}

// MockRequest represents a single mock request.
type MockRequest struct {
	server           *MockServer
	assert           *assert.Assertions
	assertStatusCode int
	assertBody       []byte
	assertJSON       bool
}

// NewMockServer initializes a mock HTTP server against which any request be sent,
// and the request can be responded to with a mock response.
func NewMockServer() *MockServer {
	m := &MockServer{
		handles: make(map[string]MockResponse),
	}
	s := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if the URI is registered.
			if _, ok := m.handles[r.RequestURI]; !ok {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte("not found"))
				return
			}

			// Check if the method+URI is registered.
			out, ok := m.handles[r.RequestURI]
			if !ok {
				w.WriteHeader(http.StatusMethodNotAllowed)
				w.Write([]byte("method not allowed"))
				return
			}

			// Write the status code.
			if out.StatusCode == 0 {
				w.WriteHeader(200)
			} else {
				w.WriteHeader(out.StatusCode)
			}
			if out.ContentType != "" {
				w.Header().Set("Content-Type", out.ContentType)
			}
			if len(out.Body) > 0 {
				w.Write(out.Body)
			}
			return
		}),
	)
	m.Server = s
	return m
}

// Handle registers a mock response handler.
func (m *MockServer) Handle(method, uri string, r MockResponse) {
	key := method + uri
	_, ok := m.handles[key]
	if ok {
		panic(fmt.Sprintf("handle already registered: %v:%v", method, uri))
	}

	m.handles[key] = r
	m.handles[uri] = r
}

// URL returns the URL of the mock server that can be used as the mock
// upstream server.
func (m *MockServer) URL() string {
	return m.Server.URL
}

// NewFastglueReq returns an empty fastglue.Request that can be filled
// and used to pass to actual fastglue handlers to mock incoming HTTP requests.
func (m *MockServer) NewFastglueReq() *Request {
	return &Request{
		RequestCtx: &fasthttp.RequestCtx{Request: *fasthttp.AcquireRequest()},
	}
}

// NewReq returns a new request handler with which a mock request can be made.
func (m *MockServer) NewReq(t *testing.T) *MockRequest {
	return &MockRequest{
		server: m,
		assert: assert.New(t),
	}
}

// SetAssert allows the setting of an optional assert instance instance.
func (r *MockRequest) SetAssert(a *assert.Assertions) *MockRequest {
	r.assert = a
	return r
}

// AssertStatus asserts the response code of the request against the given code.
func (r *MockRequest) AssertStatus(code int) *MockRequest {
	r.assertStatusCode = code
	return r
}

// AssertBody asserts the response body of the request against the given body.
func (r *MockRequest) AssertBody(body []byte) *MockRequest {
	r.assertBody = body
	return r
}

// AssertJSON asserts the JSON response of the body of the request against the given body.
func (r *MockRequest) AssertJSON(body []byte) *MockRequest {
	r.assertBody = body
	r.assertJSON = true
	return r
}

// Do takes an HTTP handler and executes it against the given request. In addition,
// it runs assertions, if there are any set.
//
// Since there's no real HTTP server+handler that originates the request to the
// handler, an artificial request (req) context has to be provided.
//
// upstreamResp is the body with which the mock server should respond to the
// request.
//
// Example:
// req := &fastglue.Request{
// 	RequestCtx: &fasthttp.RequestCtx{Request: *fasthttp.AcquireRequest()},
// 	Context:    app,
// }
// req.RequestCtx.Request.SetRequestURI("/fake/path/to/simulate")
// req.RequestCtx.SetUserValue("user", authUser{
// 	UserID: testUser,
// 	AppID:  1,
// })
//
func (r *MockRequest) Do(h FastRequestHandler, req *Request) *Request {
	// Execute the request.
	r.assert.NoError(h(req), "error executing mock request")

	// Run assertions.
	if r.assertStatusCode != 0 {
		r.assert.Equal(r.assertStatusCode, req.RequestCtx.Response.StatusCode(),
			"status code doesn't match")
	}
	if len(r.assertBody) > 0 {
		if r.assertJSON {
			r.assert.JSONEq(string(r.assertBody), string(req.RequestCtx.Response.Body()),
				"JSON response doesnt match")
		} else {
			r.assert.Equal(r.assertBody, req.RequestCtx.Response.Body(),
				"response body doesnt match")
		}
	}

	return req
}
