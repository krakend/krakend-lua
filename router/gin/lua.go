package gin

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/krakendio/binder"
	lua "github.com/krakendio/krakend-lua/v2"
	"github.com/krakendio/krakend-lua/v2/decorator"
	"github.com/krakendio/krakend-lua/v2/router"
	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/logging"
	"github.com/luraproject/lura/v2/proxy"
	krakendgin "github.com/luraproject/lura/v2/router/gin"
	glua "github.com/yuin/gopher-lua"
)

func Register(l logging.Logger, extraConfig config.ExtraConfig, engine *gin.Engine) {
	logPrefix := "[SERVICE: Gin][Lua]"
	cfg, err := lua.Parse(l, extraConfig, router.Namespace)
	if err != nil {
		if err != lua.ErrNoExtraConfig {
			l.Debug(logPrefix, err.Error())
		}
		return
	}

	l.Debug(logPrefix, "Middleware is now ready")

	engine.Use(func(c *gin.Context) {
		if err := process(c, &cfg); err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		c.Next()
	})
}

func HandlerFactory(l logging.Logger, next krakendgin.HandlerFactory) krakendgin.HandlerFactory {
	return func(remote *config.EndpointConfig, p proxy.Proxy) gin.HandlerFunc {
		logPrefix := "[ENDPOINT: " + remote.Endpoint + "][Lua]"
		handlerFunc := next(remote, p)

		cfg, err := lua.Parse(l, remote.ExtraConfig, router.Namespace)
		if err != nil {
			if err != lua.ErrNoExtraConfig {
				l.Debug(logPrefix, err.Error())
			}
			return handlerFunc
		}

		l.Debug(logPrefix, "Middleware is now ready")

		return func(c *gin.Context) {
			if err := process(c, &cfg); err != nil {
				if errhttp, ok := err.(errHTTP); ok {
					if e, ok := err.(errHTTPWithContentType); ok {
						c.Writer.Header().Add("content-type", e.Encoding())
					}
					c.AbortWithError(errhttp.StatusCode(), err)
					return
				}
				c.AbortWithError(http.StatusInternalServerError, err)
				return
			}

			handlerFunc(c)
		}
	}
}

type errHTTP interface {
	error
	StatusCode() int
}

type errHTTPWithContentType interface {
	errHTTP
	Encoding() string
}

func process(c *gin.Context, cfg *lua.Config) error {
	b := lua.NewBinderWrapper(binder.Options{
		SkipOpenLibs:        !cfg.AllowOpenLibs,
		IncludeGoStackTrace: true,
	})
	defer b.GetBinder().Close()

	decorator.RegisterErrors(b.GetBinder())
	decorator.RegisterNil(b.GetBinder())
	decorator.RegisterLuaTable(b.GetBinder())
	decorator.RegisterLuaList(b.GetBinder())
	decorator.RegisterHTTPRequest(c, b.GetBinder())
	registerCtxTable(c, b.GetBinder())

	if err := b.WithConfig(cfg); err != nil {
		return err
	}

	return b.WithCode("pre-script", cfg.PreCode)
}

func registerCtxTable(c *gin.Context, b *binder.Binder) {
	r := &ginContext{c}

	t := b.Table("ctx")

	t.Static("load", func(c *binder.Context) error {
		c.Push().Data(r, "ctx")
		return nil
	})

	t.Dynamic("method", r.method)
	t.Dynamic("url", r.url)
	t.Dynamic("host", r.host)
	t.Dynamic("query", r.query)
	t.Dynamic("params", r.params)
	t.Dynamic("headers", r.requestHeaders)
	t.Dynamic("headerList", r.headerList)
	t.Dynamic("body", r.requestBody)
}

type ginContext struct {
	*gin.Context
}

func (*ginContext) method(c *binder.Context) error {
	req, ok := c.Arg(1).Data().(*ginContext)
	if !ok {
		return errContextExpected
	}

	if c.Top() == 1 {
		c.Push().String(req.Request.Method)
	} else {
		req.Request.Method = c.Arg(2).String()
	}

	return nil
}

func (*ginContext) url(c *binder.Context) error {
	req, ok := c.Arg(1).Data().(*ginContext)
	if !ok {
		return errContextExpected
	}

	if c.Top() == 1 {
		c.Push().String(req.Request.URL.String())
	} else {
		req.Request.URL, _ = url.Parse(c.Arg(2).String())
	}

	return nil
}

func (*ginContext) host(c *binder.Context) error {
	req, ok := c.Arg(1).Data().(*ginContext)
	if !ok {
		return errContextExpected
	}

	if c.Top() == 1 {
		c.Push().String(req.Request.Host)
	} else {
		req.Request.Host = c.Arg(2).String()
	}

	return nil
}

func (*ginContext) query(c *binder.Context) error {
	req, ok := c.Arg(1).Data().(*ginContext)
	if !ok {
		return errContextExpected
	}

	switch c.Top() {
	case 1:
		return errNeedsArguments
	case 2:
		c.Push().String(req.Query(c.Arg(2).String()))
	case 3:
		q := req.Request.URL.Query()
		q.Set(c.Arg(2).String(), c.Arg(3).String())
		req.Request.URL.RawQuery = q.Encode()
	}

	return nil
}

func (*ginContext) params(c *binder.Context) error {
	req, ok := c.Arg(1).Data().(*ginContext)
	if !ok {
		return errContextExpected
	}
	switch c.Top() {
	case 1:
		return errNeedsArguments
	case 2:
		c.Push().String(req.Params.ByName(c.Arg(2).String()))
	case 3:
		key := c.Arg(2).String()
		for i, p := range req.Params {
			if p.Key == key {
				req.Params[i].Value = c.Arg(3).String()
				return nil
			}
		}
		req.Params = append(req.Params, gin.Param{Key: c.Arg(2).String(), Value: c.Arg(3).String()})
	}

	return nil
}

func (*ginContext) requestHeaders(c *binder.Context) error {
	req, ok := c.Arg(1).Data().(*ginContext)
	if !ok {
		return errContextExpected
	}
	switch c.Top() {
	case 1:
		return errNeedsArguments
	case 2:
		c.Push().String(req.Request.Header.Get(c.Arg(2).String()))
	case 3:
		_, isNil := c.Arg(3).Any().(*glua.LNilType)
		if isNil {
			req.Request.Header.Del(c.Arg(2).String())
			return nil
		}
		req.Request.Header.Set(c.Arg(2).String(), c.Arg(3).String())
	}

	return nil
}

func (*ginContext) headerList(c *binder.Context) error {
	req, ok := c.Arg(1).Data().(*ginContext)
	if !ok {
		return errContextExpected
	}
	switch c.Top() {
	case 1:
		return errNeedsArguments
	case 2:
		key := textproto.CanonicalMIMEHeaderKey(c.Arg(2).String())

		headers := req.Request.Header.Values(key)
		d := make([]interface{}, len(headers))
		for i := range headers {
			d[i] = headers[i]
		}
		c.Push().Data(&lua.List{Data: d}, "luaList")
	case 3:
		key := textproto.CanonicalMIMEHeaderKey(c.Arg(2).String())

		v, isUserData := c.Arg(3).Any().(*glua.LUserData)
		if !isUserData {
			return errInvalidLuaList
		}

		list, isList := v.Value.(*lua.List)
		if !isList {
			return errInvalidLuaList
		}

		d := make([]string, len(list.Data))
		for i := range list.Data {
			d[i] = fmt.Sprintf("%s", list.Data[i])
		}
		req.Request.Header[key] = d
	}

	return nil
}

func (*ginContext) requestBody(c *binder.Context) error {
	req, ok := c.Arg(1).Data().(*ginContext)
	if !ok {
		return errContextExpected
	}

	if c.Top() == 2 {
		req.Request.Body = io.NopCloser(bytes.NewBufferString(c.Arg(2).String()))
		return nil
	}

	var b []byte
	if req.Request.Body != nil {
		b, _ = io.ReadAll(req.Request.Body)
		req.Request.Body.Close()
	}
	req.Request.Body = io.NopCloser(bytes.NewBuffer(b))
	c.Push().String(string(b))

	return nil
}

var (
	errNeedsArguments  = errors.New("need arguments")
	errContextExpected = errors.New("ginContext expected")
	errInvalidLuaList  = errors.New("invalid header value, must be a luaList")
)
