package fastglue

import (
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/valyala/fasthttp"
)

func TestMockServer(t *testing.T) {
	m := NewMockServer()

	// Register mock upstream handlers.
	m.Handle(fasthttp.MethodGet, "/test", MockResponse{Body: []byte("hello world")})
	m.Handle(fasthttp.MethodGet, "/test2", MockResponse{
		StatusCode: fasthttp.StatusInternalServerError,
		Body:       []byte("ouch")})

	// Create a fake request context and use it with the real handler.
	req := m.NewFastglueReq()
	req.RequestCtx.SetUserValue("mock_url", m.URL()+"/test")
	m.NewReq(t).AssertStatus(fasthttp.StatusOK).
		AssertBody([]byte("hello world")).
		Do(handleMockRequest, req)

	req = m.NewFastglueReq()
	req.RequestCtx.SetUserValue("mock_url", m.URL()+"/test2")
	m.NewReq(t).AssertStatus(fasthttp.StatusInternalServerError).
		AssertBody([]byte("ouch")).
		Do(handleMockRequest, req)
}

// handleMockRequest is a dummy HTTP handler that sends a request
// to the mock server URL and writes that response.
func handleMockRequest(r *Request) error {
	var (
		mockURL = r.RequestCtx.UserValue("mock_url").(string)
	)

	resp, err := http.Get(mockURL)
	if err != nil {
		r.SendErrorEnvelope(fasthttp.StatusInternalServerError,
			err.Error(), nil, "error")
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	r.RequestCtx.SetStatusCode(resp.StatusCode)
	r.RequestCtx.Write(body)
	return nil
}
