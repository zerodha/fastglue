package fastglue

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/valyala/fasthttp"
)

var (
	srv        = NewGlue()
	srvAddress = ":8080"
	srvRoot    = "http://127.0.0.1:8080"
	sck        = "/tmp/fastglue-test.sock"
)

type App struct {
	version string

	// Redis and other DB connection objects can go here.
}

type Person struct {
	Name    *string `json:"name" required:"true" schema:"name,required"`
	Age     *int    `json:"age" schema:"age,required" required:"true"`
	Comment string  `json:"comment"`
	Version string  `json:"version"`
}

type PersonEnvelope struct {
	Status    string     `json:"status"`
	Message   *string    `json:"message"`
	Person    Person     `json:"data"`
	ErrorType *ErrorType `json:"error_type,omitempty"`
}

// Setup a mock server to test.
func init() {
	srv.SetContext(&App{version: "xxx"})
	srv.Before(getParamMiddleware)

	srv.GET("/get", myGEThandler)
	srv.DELETE("/delete", myGEThandler)
	srv.POST("/post", myPOSThandler)
	srv.PUT("/put", myPOSThandler)
	srv.POST("/post_json", myPOSTJsonhandler)
	srv.GET("/raw_json", myRawJSONhandler)
	srv.GET("/required", ReqParams(myGEThandler, []string{"name"}))

	log.Println("Listening on Test Server", srvAddress)
	go (func() {
		log.Fatal(srv.ListenAndServe(srvAddress, sck, nil))
	})()

	time.Sleep(time.Second * 2)
}

func GETrequest(url string, t *testing.T) *http.Response {
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("Failed GET request: %v", err)
	}

	return resp
}

func POSTrequest(url string, form url.Values, t *testing.T) *http.Response {
	req, _ := http.NewRequest("POST", url, strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	c := http.Client{}
	resp, err := c.Do(req)

	if err != nil {
		t.Fatalf("Failed POST request: %v", err)
	}

	return resp
}

func DELETErequest(url string, t *testing.T) *http.Response {
	req, _ := http.NewRequest("DELETE", url, nil)
	c := http.Client{}
	resp, err := c.Do(req)

	if err != nil {
		t.Fatalf("Failed DELETE request: %v", err)
	}

	return resp
}

func POSTJsonRequest(url string, j []byte, t *testing.T) *http.Response {
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(j))
	req.Header.Set("Content-Type", "application/json")
	c := http.Client{}
	resp, err := c.Do(req)

	if err != nil {
		t.Fatalf("Failed POST Json request: %v", err)
	}

	return resp
}

func decodeEnvelope(resp *http.Response, t *testing.T) (Envelope, string) {
	defer resp.Body.Close()

	// JSON envelope body.
	var e Envelope
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Couldn't read response body: %v: %s", err, b)
	}

	err = json.Unmarshal(b, &e)
	if err != nil {
		t.Fatalf("Couldn't unmarshal envelope: %v: %s", err, b)
	}

	return e, string(b)
}

func getParamMiddleware(r *Request) *Request {
	if string(r.RequestCtx.FormValue("param")) != "123" {
		r.SendErrorEnvelope(fasthttp.StatusBadRequest, "You haven't sent `param` with the value '123'", nil, "InputException")

		return nil
	}

	return r
}

func myGEThandler(r *Request) error {
	return r.SendEnvelope(struct {
		Something string `json:"out"`
	}{"name=" + string(r.RequestCtx.FormValue("name"))})
}

func myPOSThandler(r *Request) error {
	var p Person
	if err := r.DecodeFail(&p); err != nil {
		return err
	}

	if *(p.Age) < 18 {
		r.SendErrorEnvelope(fasthttp.StatusBadRequest, "We only accept Persons above 18", struct {
			Random string `json:"random"`
		}{"Some random error payload"}, "InputException")

		return nil
	}

	p.Comment = "Here's a comment the server added!"

	// Get the version from the injected app context.
	p.Version = r.Context.(*App).version

	return r.SendEnvelope(p)
}

func myRawJSONhandler(r *Request) error {
	j := []byte(`{"raw":"json"}`)

	return r.SendEnvelope(json.RawMessage(j))
}

func myPOSTJsonhandler(r *Request) error {
	var p Person
	if err := r.DecodeFail(&p); err != nil {
		return err
	}

	if *(p.Age) < 18 {
		r.SendErrorEnvelope(fasthttp.StatusBadRequest, "We only accept Persons above 18", struct {
			Random string `json:"random"`
		}{"Some random error payload"}, "InputException")

		return nil
	}

	p.Comment = "Here's a comment the server added!"

	// Get the version from the injected app context.
	p.Version = r.Context.(*App).version

	return r.SendEnvelope(p)
}

func TestSocketConnection(t *testing.T) {
	c, err := net.Dial("unix", sck)
	if err != nil {
		t.Fatalf("Can't connect via socket %s: %v", sck, err)
	}
	defer c.Close()
}

func Test404Response(t *testing.T) {
	resp := GETrequest(srvRoot+"/404", t)

	if resp.StatusCode != fasthttp.StatusNotFound {
		t.Fatalf("Expected status %d != %d", fasthttp.StatusNotFound, resp.StatusCode)
	}

	// JSON envelope body.
	var e Envelope
	b, err := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(b, &e)
	if err != nil {
		t.Fatalf("Couldn't unmarshal envelope: %v: %s", err, b)
	}

	if e.ErrorType == nil || *e.ErrorType != "GeneralException" || e.Status != "error" {
		t.Fatalf("Incorrect status or error_type fields: %s", b)
	}
}

func Test405Response(t *testing.T) {
	resp := GETrequest(srvRoot+"/post", t)
	if resp.StatusCode != fasthttp.StatusMethodNotAllowed {
		t.Fatalf("Expected status %d != %d", fasthttp.StatusMethodNotAllowed, resp.StatusCode)
	}

	e, b := decodeEnvelope(resp, t)
	if e.ErrorType == nil || *e.ErrorType != "GeneralException" || e.Status != "error" {
		t.Fatalf("Incorrect status or error_type fields: %s", b)
	}
}

func TestBadGetRequest(t *testing.T) {
	resp := GETrequest(srvRoot+"/get", t)

	if resp.StatusCode != fasthttp.StatusBadRequest {
		t.Fatalf("Expected status %d != %d", fasthttp.StatusBadRequest, resp.StatusCode)
	}

	e, _ := decodeEnvelope(resp, t)
	if e.Status != "error" {
		t.Fatalf("Expected `status` field error != %s", e.Status)
	}

	if e.ErrorType == nil || *e.ErrorType != "InputException" {
		t.Fatalf("Expected `error_type` field InputException != %s", *e.ErrorType)
	}
}

func TestGetRequest(t *testing.T) {
	resp := GETrequest(srvRoot+"/get?param=123&name=test", t)

	if resp.StatusCode != fasthttp.StatusOK {
		t.Fatalf("Expected status %d != %d", fasthttp.StatusOK, resp.StatusCode)
	}

	e, _ := decodeEnvelope(resp, t)
	if e.Status != "success" {
		t.Fatalf("Expected `status` field success != %s", e.Status)
	}

	if e.ErrorType != nil {
		t.Fatalf("Expected `error_type` field nil != %s", *e.ErrorType)
	}

	out := "map[out:name=test]"
	if fmt.Sprintf("%v", e.Data) != out {
		t.Fatalf("Expected `data` field %s != %v", out, e.Data)
	}
}

func TestRawJSONrequest(t *testing.T) {
	resp := GETrequest(srvRoot+"/raw_json?param=123&name=test", t)

	e, _ := decodeEnvelope(resp, t)
	if e.Status != "success" {
		t.Fatalf("Expected `status` field success != %s", e.Status)
	}

	out := "map[raw:json]"
	if fmt.Sprintf("%v", e.Data) != out {
		t.Fatalf("Expected `data` field %s != %v", out, e.Data)
	}
}

func TestDeleteRequest(t *testing.T) {
	resp := DELETErequest(srvRoot+"/delete?param=123&name=test", t)

	if resp.StatusCode != fasthttp.StatusOK {
		t.Fatalf("Expected status %d != %d", fasthttp.StatusOK, resp.StatusCode)
	}

	e, _ := decodeEnvelope(resp, t)
	if e.Status != "success" {
		t.Fatalf("Expected `status` field success != %s", e.Status)
	}

	if e.ErrorType != nil {
		t.Fatalf("Expected `error_type` field nil != %s", *e.ErrorType)
	}

	out := "map[out:name=test]"
	if fmt.Sprintf("%v", e.Data) != out {
		t.Fatalf("Expected `data` field %s != %v", out, e.Data)
	}
}

func TestRequiredParams(t *testing.T) {
	// Skip the required params.
	resp := GETrequest(srvRoot+"/required?param=123", t)

	if resp.StatusCode != fasthttp.StatusBadRequest {
		t.Fatalf("Expected status %d != %d", fasthttp.StatusBadRequest, resp.StatusCode)
	}

	e, _ := decodeEnvelope(resp, t)
	if e.Status != "error" {
		t.Fatalf("Expected `status` field error != %s", e.Status)
	}

	if e.ErrorType == nil || *e.ErrorType != "InputException" {
		t.Fatalf("Expected `error_type` field InputException != %s", *e.ErrorType)
	}

	// Pass a value.
	resp = GETrequest(srvRoot+"/required?param=123&name=testxxx", t)
	if resp.StatusCode != fasthttp.StatusOK {
		t.Fatalf("Expected status %d != %d", fasthttp.StatusOK, resp.StatusCode)
	}

	e, _ = decodeEnvelope(resp, t)
	if e.Status != "success" {
		t.Fatalf("Expected `status` field success != %s", e.Status)
	}

	out := "map[out:name=testxxx]"
	if fmt.Sprintf("%v", e.Data) != out {
		t.Fatalf("Expected `data` field %s != %v", out, e.Data)
	}
}

func TestBadPOSTJsonRequest(t *testing.T) {
	// Struct that we'll marshal to JSON and post.
	resp := POSTJsonRequest(srvRoot+"/post_json?param=123&name=test", []byte{0}, t)
	if resp.StatusCode != fasthttp.StatusBadRequest {
		t.Fatalf("Expected status %d != %d", fasthttp.StatusBadRequest, resp.StatusCode)
	}

	e, b := decodeEnvelope(resp, t)
	if e.Status != "error" {
		t.Fatalf("Expected `status` field error != %s: %s", e.Status, b)
	}

	if e.ErrorType == nil || *e.ErrorType != "InputException" {
		t.Fatalf("Expected `error_type` field InputException != %s", *e.ErrorType)
	}
}

func TestPOSTJsonRequest(t *testing.T) {
	pData := []byte(`
			{
				"name": "tester",
				"age" : 30
			}`)

	resp := POSTJsonRequest(srvRoot+"/post_json?param=123&name=test", pData, t)
	b, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != fasthttp.StatusOK {
		t.Fatalf("Expected status %d != %d: %s", fasthttp.StatusOK, resp.StatusCode, b)
	}

	var pe PersonEnvelope
	err := json.Unmarshal(b, &pe)
	if err != nil {
		t.Fatalf("Couldn't unmarshal JSON response: %v = %s", err, b)
	}

	if *pe.Person.Age != 30 || pe.Person.Version != "xxx" || len(pe.Person.Comment) < 1 {
		t.Fatalf("Unexpected enveloped response: (age: 30, version: xxx) != %s", b)
	}
}

func TestValidationJsonRequest(t *testing.T) {
	personData := []byte(`
				{
					"name": "test"
				}`)

	resp := POSTJsonRequest(srvRoot+"/post_json?param=123&name=test", personData, t)
	b, _ := ioutil.ReadAll(resp.Body)
	// This should fail with error message, `age is invalid`.
	if resp.StatusCode != fasthttp.StatusBadRequest {
		t.Fatalf("Expected status %d != %d: %s", fasthttp.StatusOK, resp.StatusCode, b)
	}
	var pe Person
	err := json.Unmarshal(b, &pe)
	if err != nil {
		t.Fatalf("Couldn't unmarshal JSON response: %v = %s", err, b)
	}
}

func TestPOSTFormRequest(t *testing.T) {
	form := url.Values{}
	form.Add("name", "test")

	resp := POSTrequest(srvRoot+"/post?param=123", form, t)
	b, _ := ioutil.ReadAll(resp.Body)
	// This should fail with error message `age is empty`.
	if resp.StatusCode != fasthttp.StatusBadRequest {
		t.Fatalf("Expected status %d != %d: %s", fasthttp.StatusOK, resp.StatusCode, b)
	}

	var e Envelope
	err := json.Unmarshal(b, &e)
	if err != nil {
		t.Fatalf("Couldn't unmarshal JSON response: %v = %s", err, b)
	}

}
