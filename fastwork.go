package fastwork

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"strings"

	"github.com/buaazp/fasthttprouter"
	"github.com/valyala/fasthttp"
)

var (
	constJSON = []byte("json")
	constXML  = []byte("xml")
)

const (
	JSON = "application/json"
	XML  = "xml"
)

type Test *fasthttp.RequestCtx

type FastRequestHandler func(*Context)

type Context struct {
	RequestCtx *fasthttp.RequestCtx
}

type Fastwork struct {
	Router *fasthttprouter.Router
}

func New() *Fastwork {
	return &Fastwork{
		Router: fasthttprouter.New(),
	}
}

func (c *Context) Decode(v interface{}) {
	var (
		err error
		ct  = c.RequestCtx.Request.Header.ContentType()
	)

	if bytes.Contains(ct, constJSON) {
		err = json.Unmarshal(c.RequestCtx.PostBody(), &v)
	} else if bytes.Contains(ct, constXML) {
		err = xml.Unmarshal(c.RequestCtx.PostBody(), &v)
	} else {
		c.respond(fasthttp.StatusBadRequest, "error", "Unknown encoding "+string(ct), nil)
		return
	}

	if err != nil {
		c.respond(fasthttp.StatusBadRequest, "error", "Error decoding request body", nil)
		return
	}
}

func (c *Context) respond(code int, status string, message string, v interface{}) {
	c.RequestCtx.SetStatusCode(code)

	if j, err := json.Marshal(struct {
		Status  string      `json:"status"`
		Message string      `json:"message"`
		Data    interface{} `json:"data"`
	}{status, message, v}); err == nil {
		_, err = c.RequestCtx.Write(j)
	} else {
		c.respond(fasthttp.StatusInternalServerError, "errpr", "Could not encode respone", nil)
		return
	}
}

func (c *Context) Respond(v interface{}) {
	c.respond(fasthttp.StatusOK, "success", "", v)
}

func (c *Context) RespondError(code int, message string, v interface{}) {
	c.respond(code, "error", message, v)
}

func HasFields(h fasthttp.RequestHandler, fields []string) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		for _, f := range fields {
			if !ctx.Request.PostArgs().Has(f) && !ctx.QueryArgs().Has(f) {
				ctx.WriteString("Missing required field " + f)
				return
			}
		}

		h(ctx)
	}
}

func IsType(h fasthttp.RequestHandler, typ string) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		if !strings.Contains(string(ctx.Request.Header.ContentType()), typ) {
			ctx.WriteString("Invalid content-type. Expecting " + typ)
			return
		}
	}
}

func IsJSON(h fasthttp.RequestHandler) fasthttp.RequestHandler {
	return IsType(h, JSON)
}

func AsJSON(h fasthttp.RequestHandler, obj interface{}) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		if !bytes.Contains(ctx.Request.Header.ContentType(), constJSON) {
			ctx.WriteString("Invalid content-type. Expecting JSON.")
			return
		}

		if err := json.Unmarshal(ctx.Request.Body(), obj); err != nil {
			ctx.WriteString("Error parsing JSON:" + err.Error())
			return
		}

		h(ctx)
	}
}

func (f *Fastwork) handler(h FastRequestHandler) func(*fasthttp.RequestCtx) {
	return func(ctx *fasthttp.RequestCtx) {
		c := &Context{
			RequestCtx: ctx,
		}
		ctx.SetUserValue("Fast", c)

		h(c)
	}
}

func (f *Fastwork) POST(path string, h FastRequestHandler) {
	f.Router.POST(path, f.handler(h))
}

func (f *Fastwork) GET(path string, h FastRequestHandler) {
	f.Router.GET(path, f.handler(h))
}

func (f *Fastwork) Handler() func(*fasthttp.RequestCtx) {
	return f.Router.Handler
}
