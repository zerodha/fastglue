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
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/valyala/fasthttp"
)

var (
	srv        = NewGlue()
	srvAddress = ":10200"
	srvRoot    = "http://127.0.0.1:10200"
	sck        = "/tmp/fastglue-test.sock"
)

type App struct {
	version string

	// Redis and other DB connection objects can go here.
}

type Person struct {
	Name    string `json:"name"`
	Age     int    `json:"age"`
	Comment string `json:"comment"`
	Version string `json:"version"`
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
	srv.GET("/redirect", myRedirectHandler)
	srv.DELETE("/delete", myGEThandler)
	srv.POST("/post", myPOSThandler)
	srv.PUT("/put", myPOSThandler)
	srv.POST("/post_json", myPOSTJsonhandler)
	srv.GET("/raw_json", myRawJSONhandler)
	srv.GET("/required", ReqParams(myGEThandler, []string{"name"}))
	srv.POST("/required", ReqParams(myGEThandler, []string{"name"}))
	srv.GET("/required_length", ReqLenParams(myGEThandler, map[string]int{"name": 5}))
	srv.POST("/required_length", ReqLenParams(myGEThandler, map[string]int{"name": 5}))
	srv.GET("/required_length_range", ReqLenRangeParams(myGEThandler, map[string][2]int{"name": {5, 10}}))
	srv.POST("/required_length_range", ReqLenRangeParams(myGEThandler, map[string][2]int{"name": {5, 10}}))
	srv.Any("/any", myAnyHandler)
	srv.ServeStatic("/dir-examples/*filepath", "./examples", true)
	srv.ServeStatic("/no-dir-examples/*filepath", "./examples", false)

	log.Println("Listening on Test Server", srvAddress)
	go (func() {
		log.Fatal(srv.ListenAndServe(srvAddress, "", nil))
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

func myAnyHandler(r *Request) error {
	// Write the incoming method name to the body.
	return r.SendBytes(http.StatusOK, "text/plain", r.RequestCtx.Method())
}

func myRedirectHandler(r *Request) error {
	return r.Redirect("/get", fasthttp.StatusFound, map[string]interface{}{
		"name":  "Redirected" + string(r.RequestCtx.FormValue("name")),
		"param": "123",
	}, "")
}

func myRedirectExternalHandler(r *Request) error {
	return r.Redirect("http://localhost:12345/redirect", fasthttp.StatusFound, map[string]interface{}{
		"name":  "Redirected" + string(r.RequestCtx.FormValue("name")),
		"param": "123",
	}, "")
}

func myPOSThandler(r *Request) error {
	var p Person
	if err := r.DecodeFail(&p, "json"); err != nil {
		return err
	}

	if p.Age < 18 {
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
	if err := r.DecodeFail(&p, "json"); err != nil {
		return err
	}

	if p.Age < 18 {
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
	log.Println("Listening on Test Server", sck)
	go (func() {
		log.Fatal(NewGlue().ListenAndServe("", sck, nil))
	})()
	time.Sleep(time.Second * 2)

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

	// Test POST.
	form := url.Values{}
	form.Add("param", "123")
	form.Add("name", "testxxx")

	resp = POSTrequest(srvRoot+"/required", form, t)
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

func TestRequiredParamsLen(t *testing.T) {
	// Skip the required params.
	resp := GETrequest(srvRoot+"/required_length?param=123&name=a", t)

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

	// Test POST.
	form := url.Values{}
	form.Add("param", "123")
	form.Add("name", "testxxx")

	resp = POSTrequest(srvRoot+"/required_length", form, t)
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

func TestRequiredParamsLenRange(t *testing.T) {
	// Skip the required params.
	resp := GETrequest(srvRoot+"/required_length_range?param=123", t)
	if resp.StatusCode != fasthttp.StatusBadRequest {
		t.Fatalf("Expected status %d != %d", fasthttp.StatusBadRequest, resp.StatusCode)
	}

	// Short.
	resp = GETrequest(srvRoot+"/required_length_range?param=123&name=a", t)
	if resp.StatusCode != fasthttp.StatusBadRequest {
		t.Fatalf("Expected status %d != %d", fasthttp.StatusBadRequest, resp.StatusCode)
	}

	// Too long.
	resp = GETrequest(srvRoot+"/required_length_range?param=123&name=aaaaaaaaaaaa", t)
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

	// Test POST.
	form := url.Values{}
	form.Add("param", "123")
	form.Add("name", "testxxx")

	resp = POSTrequest(srvRoot+"/required_length", form, t)
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

	if pe.Person.Age != 30 || pe.Person.Version != "xxx" || len(pe.Person.Comment) < 1 {
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
	form.Add("age", "6")

	resp := POSTrequest(srvRoot+"/post?param=123", form, t)
	b, _ := ioutil.ReadAll(resp.Body)
	// This should fail with error message `age is empty`.
	if resp.StatusCode != fasthttp.StatusBadRequest {
		t.Fatalf("Expected status %d != %d: %s", fasthttp.StatusBadRequest, resp.StatusCode, b)
	}

	var e Envelope
	err := json.Unmarshal(b, &e)
	if err != nil {
		t.Fatalf("Couldn't unmarshal JSON response: %v = %s", err, b)
	}
}

func TestRedirectRequest(t *testing.T) {
	resp := GETrequest(srvRoot+"/redirect?param=123&name=test", t)

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

	out := "map[out:name=Redirectedtest]"
	if fmt.Sprintf("%v", e.Data) != out {
		t.Fatalf("Expected `data` field %s != %v", out, e.Data)
	}
}

func TestRedirectScheme(t *testing.T) {
	req, _ := http.NewRequest("GET", srvRoot+"/redirect?param=123&name=test", nil)

	// This should force the redirect to an https URL
	// which should then timeout.
	req.Header.Add("X-Forwarded-Proto", "https")
	c := http.Client{
		Timeout: time.Second * 1,
	}
	_, err := c.Do(req)
	if err == nil {
		t.Fatal("Automatic https redirect should have time out but no error occurred")
	}
	if tErr, ok := err.(net.Error); !ok {
		t.Fatalf("Expected timeout error on https redirect but got: %v", err)
	} else if !tErr.Timeout() {
		t.Fatalf("Expected timeout error on https redirect but got: %v", err)
	}
}

func TestAnyHandler(t *testing.T) {
	c := http.Client{
		Timeout: time.Second * 3,
	}

	methods := []string{"GET", "POST", "PUT", "DELETE"}
	for _, m := range methods {
		req, _ := http.NewRequest(m, srvRoot+"/any?param=123&name=test", nil)
		resp, err := c.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			// The response body should be the method that was sent.
			t.Fatalf("any (%s) request failed (status: %d): %v",
				m, resp.StatusCode, err)
		}

		// Response body should match the method name.
		b, _ := ioutil.ReadAll(resp.Body)
		respMethod := strings.ToUpper(string(b))
		if respMethod != m {
			t.Fatalf("any handler's response doesn't match method name: %s != %v",
				respMethod, m)
		}
	}
}

func TestScanArgs(t *testing.T) {
	type test struct {
		Str1            string `url:"str1"`
		StrBlock        string `url:"-"`
		StrNoTag        *string
		Strings         []string `url:"str"`
		Bytes           []byte   `url:"bytes"`
		Int1            int      `url:"int1"`
		Ints            []int    `url:"int"`
		NonExistentInts []int    `url:"nonint"`
		Bool1           bool     `url:"bool1"`
		Bools           []bool   `url:"bool"`
		NonExistent     []string `url:"non"`
		BadNum          int      `url:"badnum"`
		BadNumSlice     []int    `url:"badnumslice"`
		OtherTag        string   `form:"otherval"`
		OmitEmpty       string   `form:"otherval,omitempty"`
		OtherTags       string   `url:"othertags" json:"othertags"`
	}
	var o test

	args := fasthttp.AcquireArgs()
	args.Add("str1", "string1")
	args.Add("str", "str1")
	args.Add("str", "str2")
	args.Add("str", "str3")
	args.Add("bytes", "manybytes")
	args.Add("int1", "123")
	args.Add("int", "456")
	args.Add("int", "789")
	args.Add("bool1", "true")
	args.Add("bool", "true")
	args.Add("bool", "false")
	args.Add("bool", "f")
	args.Add("bool", "t")
	args.Add("badnum", "abc")
	args.Add("badnumslice", "abc")
	args.Add("badnumslice", "def")

	ScanArgs(args, &o, "url")
	exp := test{
		Str1:            "string1",
		Strings:         []string{"str1", "str2", "str3"},
		Bytes:           []byte("manybytes"),
		Int1:            123,
		Ints:            []int{456, 789},
		NonExistentInts: nil,
		Bool1:           true,
		Bools:           []bool{true, false, false, true},
		BadNum:          0,
		BadNumSlice:     []int{0, 0},
	}
	if !reflect.DeepEqual(exp, o) {
		t.Error("scan structs don't match. expected != scanned")
		fmt.Println(exp)
		fmt.Println(o)
	}
}

func TestServeStatic(t *testing.T) {
	// Get file from non-directory listed path.
	resp := GETrequest(srvRoot+"/no-dir-examples/example.go", t)
	if resp.StatusCode != fasthttp.StatusOK {
		t.Fatalf("Expected status %d != %d", fasthttp.StatusOK, resp.StatusCode)
	}

	// Get directory from non-directory listed path.
	resp = GETrequest(srvRoot+"/no-dir-examples/", t)
	if resp.StatusCode != fasthttp.StatusOK {
		t.Fatalf("Expected status %d != %d", fasthttp.StatusOK, resp.StatusCode)
	}

	// Get not found file from non-directory listed path.
	resp = GETrequest(srvRoot+"/no-dir-examples/filenotfound", t)
	if resp.StatusCode != fasthttp.StatusNotFound {
		t.Fatalf("Expected status %d != %d", fasthttp.StatusNotFound, resp.StatusCode)
	}

	// Get file from directory listed path.
	resp = GETrequest(srvRoot+"/dir-examples/example.go", t)
	if resp.StatusCode != fasthttp.StatusOK {
		t.Fatalf("Expected status %d != %d", fasthttp.StatusOK, resp.StatusCode)
	}

	// Get directory from directory listed path.
	resp = GETrequest(srvRoot+"/dir-examples/", t)
	if resp.StatusCode != fasthttp.StatusOK {
		t.Fatalf("Expected status %d != %d", fasthttp.StatusOK, resp.StatusCode)
	}

	// Get not found file from directory listed path.
	resp = GETrequest(srvRoot+"/dir-examples/filenotfound", t)
	if resp.StatusCode != fasthttp.StatusNotFound {
		t.Fatalf("Expected status %d != %d", fasthttp.StatusNotFound, resp.StatusCode)
	}
}
