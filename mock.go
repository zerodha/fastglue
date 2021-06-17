package fastglue

import (
	"fmt"
	"log"
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
	server *MockServer
	req    *Request
	assert *assert.Assertions
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
				logerr(w.Write([]byte("not found")))
				return
			}

			// Check if the method+URI is registered.
			out, ok := m.handles[r.Method+r.RequestURI]
			if !ok {
				w.WriteHeader(http.StatusMethodNotAllowed)
				logerr(w.Write([]byte("method not allowed")))
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
				logerr(w.Write(out.Body))
			}
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

// Reset resets existing registered mock response handlers.
func (m *MockServer) Reset() {
	m.handles = make(map[string]MockResponse)
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

// Do returns a new request handler with which a mock request is made.
// It takes an HTTP handler and executes it against the given request.
// The assert.Assertions is optional.
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
func (m *MockServer) Do(h FastRequestHandler, req *Request, t *testing.T) *MockRequest {
	mr := &MockRequest{
		req:    req,
		server: m,
		assert: assert.New(t),
	}
	mr.assert.NoError(h(req), "error executing mock request")
	return mr
}

// GetReq returns the underlying fastglue.Request that's set on the MockRequest.
func (mr *MockRequest) GetReq() *Request {
	return mr.req
}

// AssertStatus asserts the response code of the request against the given code.
func (mr *MockRequest) AssertStatus(code int) {
	mr.assert.Equal(code, mr.req.RequestCtx.Response.StatusCode(),
		"status code doesn't match")
}

// AssertBody asserts the response body of the request against the given body.
func (mr *MockRequest) AssertBody(body []byte) {
	mr.assert.Equal(body, mr.req.RequestCtx.Response.Body(),
		"response body doesn't match")
}

// AssertJSON asserts the JSON response of the body of the request against the given body.
func (mr *MockRequest) AssertJSON(body []byte) {
	mr.assert.JSONEq(string(body), string(mr.req.RequestCtx.Response.Body()),
		"response body doesn't match")
}

func logerr(n int, err error) {
	if err != nil {
		log.Printf("Write failed: %v", err)
	}
}
