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

type FastContext struct {
	*fasthttp.RequestCtx
	Body interface{}
}

type Fastwork struct {
	Router *fasthttprouter.Router
}

func New() *Fastwork {
	return &Fastwork{
		Router: fasthttprouter.New(),
	}
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

func (f *Fastwork) handler(h fasthttp.RequestHandler) func(*fasthttp.RequestCtx) {
	return func(ctx *fasthttp.RequestCtx) {
		h(ctx)
	}
}

func (f *Fastwork) GET(path string, h fasthttp.RequestHandler) {
	f.Router.GET(path, f.handler(h))
}
func (f *Fastwork) POST(path string, h fasthttp.RequestHandler) {
	f.Router.POST(path, f.handler(h))
}

func (f *Fastwork) Handler() func(*fasthttp.RequestCtx) {
	return f.Router.Handler
}

func Decode(ctx *fasthttp.RequestCtx, v interface{}) {
	var (
		err error
		c   = ctx.Request.Header.ContentType()
	)

	if bytes.Contains(c, constJSON) {
		err = json.Unmarshal(ctx.PostBody(), &v)
	} else if bytes.Contains(c, constXML) {
		err = xml.Unmarshal(ctx.PostBody(), &v)
	} else {
		ctx.WriteString("Unknown encoding " + string(c))
	}

	if err != nil {
		ctx.WriteString("Error decoding request body")
	}
}

func Respond(ctx *fasthttp.RequestCtx, v interface{}) error {
	var err error
	if j, err := json.Marshal(v); err == nil {
		_, err = ctx.Write(j)
	} else {

	}

	return err
}

func RespondError(ctx, v) {

}
