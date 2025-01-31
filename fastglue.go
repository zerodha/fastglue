package fastglue

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"strings"

	fasthttprouter "github.com/fasthttp/router"
	"github.com/valyala/fasthttp"
)

const (
	// JSON is an alias for the JSON content type
	JSON = "application/json"

	// XML is an alias for the XML content type
	XML = "application/xml"

	// PLAINTEXT is an alias for the plaintext content type
	PLAINTEXT = "text/plain"

	// AuthBasic represents HTTP BasicAuth scheme.
	AuthBasic = 1 << iota
	// AuthToken represents the key:value Token auth scheme.
	AuthToken = 2
)

var (
	constJSON = []byte("json")
	constXML  = []byte("xml")

	// Authorization schemes.
	authBasic = []byte("Basic")
	authToken = []byte("token")
)

// CompressionType is the type of otuput compression available in fasthttp
// for the response body.
type CompressionType string

const (
	CompressionZstd    CompressionType = "zstd"
	CompressionBr      CompressionType = "br"
	CompressionGzip    CompressionType = "gzip"
	CompressionDeflate CompressionType = "deflate"
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

// CompressionOpt is the configuration for enabling and controlling compression.
// Enabling this will automatically compress the response based on the client's
// Accept-Encoding header by internally invoking fasthttp.CompressHandler()
// gzip|deflate|br|zstd are the supported compression types.
type CompressionOpt struct {
	Enabled bool

	// Type of compression to support (std, gzip, deflate, br). If no type is specified
	// then fasthttp's default compression types and its internal order of priority are used.
	//
	// Important: The first type in the list is the preferred type
	// irrespective of the ordering of types in the incoming client's Accept-Encoding header.
	// That is because fasthttp's CompressHandler() internally uses arbitrary
	// type ordering to compress the response. fastglue thus overrides the
	// Accept-Encoding header to only have the first type in this list.
	// For instance, if the list here is [zstd, br], and the incoming header is
	// [gzip, deflate, br, zstd], fastglue will overwrite the header to [zstd],
	// forcing fasthttp to compress the response using the preferred type here.
	Types []CompressionType
}

// Fastglue is the "glue" wrapper over fasthttp and fasthttprouter.
type Fastglue struct {
	Router                *fasthttprouter.Router
	Server                *fasthttp.Server
	MatchedRoutePathParam string

	context interface{}
	before  []FastMiddleware
	after   []FastMiddleware

	opt Options
}

type Options struct {
	CompressionOpt CompressionOpt
}

// New creates and returns a new instance of Fastglue.
func New(o Options) *Fastglue {
	f := &Fastglue{
		Router: fasthttprouter.New(),
		opt:    o,
	}

	return f
}

// ListenAndServe is a wrapper for fasthttp.ListenAndServe. It takes a TCP address,
// an optional UNIX socket file path and starts listeners, and an optional fasthttp.Server.
func (f *Fastglue) ListenAndServe(address string, socket string, s *fasthttp.Server) error {
	if address == "" && socket == "" {
		return errors.New("specify either a TCP address or a UNIX socket")
	}
	if address != "" && socket != "" {
		return errors.New("specify either a TCP address or a UNIX socket, not both")
	}

	// No server passed, create a default one.
	if s == nil {
		s = &fasthttp.Server{}
	}
	f.Server = s

	if s.Handler == nil {
		s.Handler = f.Handler()
	}

	if socket != "" {
		return s.ListenAndServeUNIX(socket, 0666)
	}

	return s.ListenAndServe(address)
}

// ListenServeAndWaitGracefully accepts the same parameters
// as ListenAndServe along with a channel which can receive
// a signal to shutdown the server.
func (f *Fastglue) ListenServeAndWaitGracefully(address string, socket string, s *fasthttp.Server, shutdownServer chan struct{}) error {
	errChan := make(chan error, 1)
	// Listen for signal on shutdownServer channel
	go func() {
		for range shutdownServer {
			errChan <- s.Shutdown()
		}
	}()
	// Start the http server
	go func() {
		err := f.ListenAndServe(address, socket, s)
		if err != nil {
			// Only if the err was nil, we want to send to the errChan
			// else we will keep waiting for shutdownServer to
			// send an error complete.
			errChan <- err
		}
	}()

	// Wait for an error/nil, till then keep running.
	for err := range errChan {
		close(shutdownServer)
		return err
	}

	return nil
}

// Shutdown gracefully shuts down the server without interrupting any active connections.
// It accepts a fasthttp.Server instance along with an error channel, to which it
// sends a signal with a nil/error after shutdown is complete. It is safe to exit
// the program after receiving this signal.
//
// The following is taken from the fasthttp docs and applies to fastglue shutdown.
// Shutdown works by first closing all open listeners and then waiting indefinitely for
// all connections to return to idle and then shut down.
//
// When Shutdown is called, Serve, ListenAndServe, and ListenAndServeTLS
// immediately return nil.
// Make sure the program doesn't exit and waits instead for Shutdown to return.
//
// Shutdown does not close keepalive connections so its recommended
// to set ReadTimeout to something else than 0.
func (f *Fastglue) Shutdown(s *fasthttp.Server, shutdownComplete chan error) {
	shutdownComplete <- f.Server.Shutdown()
}

// handler is the "proxy" abstraction that converts a fastglue handler into
// a fasthttp handler and passes execution in and out.
func (f *Fastglue) handler(h FastRequestHandler) func(*fasthttp.RequestCtx) {
	handler := func(ctx *fasthttp.RequestCtx) {
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

		_ = h(req)

		// Apply "after" middleware.
		for _, p := range f.after {
			if p(req) == nil {
				return
			}
		}

		// If compression is enabled, override the response header to
		// the preferred type in the config.
		if f.opt.CompressionOpt.Enabled {
			for _, typ := range f.opt.CompressionOpt.Types {
				t := string(typ)
				// If the preferred type is in the client's Accept-Encoding header,
				// overwrite the request header to only have this type, forcing
				// fasthttp.CompressHandler() to compress the response using it.
				// This is because fasthttp internally does not respect the order
				// in the client header and uses arbitrary ordering to compress the response.
				if ctx.Request.Header.HasAcceptEncoding(t) {
					ctx.Request.Header.Set("Accept-Encoding", t)
					break
				}
			}
		}
	}

	// If compression is enabled, wrap the handler with fasthttp's CompressHandler
	// which automatically handles the compression logic.
	if f.opt.CompressionOpt.Enabled {
		// fasthttp's compression handlers are pretty bad. This particular handler
		// is the one that supports gzip|br|zstd|deflate.
		return fasthttp.CompressHandlerBrotliLevel(handler,
			fasthttp.CompressBrotliDefaultCompression,
			fasthttp.CompressDefaultCompression)
	}

	return handler
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
func (f *Fastglue) Before(fm ...FastMiddleware) {
	f.before = append(f.before, fm...)
}

// After registers a fastglue middleware that's executed after a registered handler
// has finished executing. This is useful to do things like central request logging.
func (f *Fastglue) After(fm ...FastMiddleware) {
	f.after = append(f.after, fm...)
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

// Any is fastglue's wrapper over fasthttprouter's handler
// that attaches a FastRequestHandler to all
// GET, POST, PUT, DELETE methods.
func (f *Fastglue) Any(path string, h FastRequestHandler) {
	f.Router.GET(path, f.handler(h))
	f.Router.POST(path, f.handler(h))
	f.Router.PUT(path, f.handler(h))
	f.Router.DELETE(path, f.handler(h))
}

// NotFound is fastglue's wrapper over fasthttprouter's `router.NotFound` handler.
func (f *Fastglue) NotFound(h FastRequestHandler) {
	f.Router.NotFound = f.handler(h)
}

// ServeStatic serves static files under `rootPath` on `path` urls.
// The `path` must end with "/{filepath:*}", files are then served from the local
// path /defined/root/dir/{filepath:*}. For example `path` can be
// "/static/{filepath:*}" and `rootPath` as "./dist/static/" to serve all the
// files "./dist/static/*" as "/static/*".
// `listDirectory` option enables or disables directory listing.
func (f *Fastglue) ServeStatic(path string, rootPath string, listDirectory bool) {
	// Create a request handler serving static files from the given `rootPath` folder.
	// The request handler created automatically generates index pages
	// for directories without index.html.
	//
	// The request handler caches requested file handles
	// for FSHandlerCacheDuration. Make sure your program has enough 'max open files' limit aka
	// 'ulimit -n' if root folder contains many files.
	//
	// Do not create multiple request handler instances for the same
	// (root, stripSlashes) arguments - just reuse a single instance.
	// Otherwise goroutine leak will occur.
	fs := &fasthttp.FS{
		Root:               rootPath,
		IndexNames:         []string{"index.html"},
		GenerateIndexPages: listDirectory,
		AcceptByteRange:    true,
	}
	f.Router.ServeFilesCustom(path, fs)
}

// Decode unmarshals the Post body of a fasthttp request based on the ContentType header
// into value pointed to by v, as long as the content is JSON or XML.
func (r *Request) Decode(v interface{}, tag string) error {
	var (
		err error
		ct  = r.RequestCtx.Request.Header.ContentType()
	)

	// Validate compulsory fields in JSON body. The struct to be unmarshaled into needs a struct tag with required=true for enforcing presence.
	if bytes.Contains(ct, constJSON) {
		err = json.Unmarshal(r.RequestCtx.PostBody(), &v)
	} else if bytes.Contains(ct, constXML) {
		err = xml.Unmarshal(r.RequestCtx.PostBody(), &v)
	} else {
		_, err = ScanArgs(r.RequestCtx.PostArgs(), v, tag)
	}
	if err != nil {
		return fmt.Errorf("error decoding request: %v", err)
	}
	return nil
}

// SendBytes writes a []byte payload to the HTTP response and also
// sets a given ContentType header.
func (r *Request) SendBytes(code int, ctype string, v []byte) error {
	r.RequestCtx.SetStatusCode(code)
	r.RequestCtx.SetContentType(ctype)
	if _, err := r.RequestCtx.Write(v); err != nil {
		return err
	}

	return nil
}

// SendString writes a string payload to the HTTP response.
// It implicitly sets ContentType to plain/text.
func (r *Request) SendString(code int, v string) error {
	r.RequestCtx.SetStatusCode(code)
	r.RequestCtx.SetContentType("text/plain")
	if _, err := r.RequestCtx.WriteString(v); err != nil {
		return err
	}

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

	if b, err = json.Marshal(v); err != nil {
		return err
	}

	if _, err := r.RequestCtx.Write(b); err != nil {
		return err
	}

	return nil
}

// Redirect redirects to the given URL.
// Accepts optional query args and anchor tags.
// Test : curl -I -L -X GET "localhost:8000/redirect"
func (r *Request) Redirect(url string, code int, args map[string]interface{}, anchor string) error {
	var redirectURI string

	// Copy current url before mutating.
	rURI := &fasthttp.URI{}
	r.RequestCtx.URI().CopyTo(rURI)
	rURI.Update(url)

	// This avoids a redirect vulnerability when `uri` is relative and contains double slash.
	// For example: if the `uri` is `/bing.com//` which is a relative path passed from client side,
	// `rURI.Update(uri)` doesn't set the hostname hence the updated uri becomes `http:///bing.com/`.
	// Most browser strips the additional forward slash and redirects to `http://bing.com`.
	// To avoid is this, we check if the updated hostname is empty and if empty then we set current request
	// hostname as the redirect hostname. In above example, ``/bing.com//`` will now become `http://request_hostname/bing.com/`.
	if len(rURI.Host()) == 0 {
		rURI.SetHostBytes(r.RequestCtx.URI().Host())
	}

	// Fill query args.
	for k, v := range args {
		rURI.QueryArgs().Add(k, fmt.Sprintf("%v", v))
	}

	// With layered proxies and loadbalancers, redirect
	// to relative URLs may not work correctly, that is,
	// the load balancer entry point was https but at the
	// end of the proxy chain, it's http.
	// So we check if the incoming hostname and the outgoing
	// redirect URL's hostname are the same, and if yes,
	// check for common scheme headers and overwrite the
	// scheme if they are set.
	if bytes.Equal(r.RequestCtx.Host(), rURI.Host()) {
		s := r.RequestCtx.Request.Header.Peek("X-Forwarded-Proto")
		if len(s) > 0 {
			rURI.SetScheme(string(s))
		}
	}

	redirectURI = rURI.String()
	// If anchor is sent, append to the URI.
	if anchor != "" {
		redirectURI += "#" + anchor
	}

	// Redirect
	r.RequestCtx.Redirect(redirectURI, code)
	return nil
}

// RedirectURI redirects to the given URI. If URI contains hostname, scheme etc
// then its stripped and only path is used for the redirect.
// Used for internal app redirect and to prevent open redirect vulnerabilities.
func (r *Request) RedirectURI(uri string, code int, args map[string]interface{}, anchor string) error {
	// Parse URI to check if its a valid URI.
	u := &fasthttp.URI{}
	err := u.Parse(nil, []byte(uri))
	if err != nil {
		return err
	}

	// Use only the rquest URI from the parsed URL.
	// This makes sure we only redirect to relative path.
	rURI := string(u.RequestURI())
	hash := string(u.Hash())
	if len(hash) > 0 {
		rURI = rURI + "#" + hash
	}

	// If path starts with more than one forward slash then its considerd
	// as full URL and leads to open redirect vulnerability.
	// So here we strip out all leading forward slashes and replace it
	// with one forward slash so its always considered as a path.
	fURI := "/" + strings.TrimLeft(rURI, "/")

	return r.Redirect(fURI, code, args, anchor)
}

// ParseAuthHeader parses the Authorization header and returns an api_key and access_token
// based on the auth schemes passed as bit flags (eg: AuthBasic, AuthBasic | AuthToken etc.).
func (r *Request) ParseAuthHeader(schemes uint8) ([]byte, []byte, error) {
	var (
		h     = r.RequestCtx.Request.Header.Peek("Authorization")
		pair  [][]byte
		delim = []byte(":")
	)

	// Basic auth scheme.
	if schemes&AuthBasic != 0 && bytes.HasPrefix(h, authBasic) {
		payload, err := base64.StdEncoding.DecodeString(string(bytes.Trim(h[len(authBasic):], " ")))
		if err != nil {
			return nil, nil, errors.New("invalid Base64 value in Basic Authorization header")
		}

		pair = bytes.SplitN(payload, delim, 2)
	} else if schemes&AuthToken != 0 && bytes.HasPrefix(h, authToken) {
		pair = bytes.SplitN(bytes.Trim(h[len(authToken):], " "), delim, 2)
	} else {
		return nil, nil, errors.New("unknown authorization scheme")
	}

	if len(pair) != 2 {
		return nil, nil, errors.New("authorization value should be `key`:`token`")
	}

	return pair[0], pair[1], nil
}
