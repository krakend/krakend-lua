package gin

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/alexeyco/binder"
	lua "github.com/devopsfaith/krakend-lua"
	"github.com/devopsfaith/krakend-lua/router"
	"github.com/devopsfaith/krakend/config"
	"github.com/devopsfaith/krakend/logging"
	"github.com/devopsfaith/krakend/proxy"
	krakendgin "github.com/devopsfaith/krakend/router/gin"
	"github.com/gin-gonic/gin"
)

func Register(l logging.Logger, extraConfig config.ExtraConfig, engine *gin.Engine) {
	cfg, err := lua.Parse(l, extraConfig, router.Namespace)
	if err != nil {
		l.Debug("lua:", err.Error())
		return
	}

	engine.Use(func(c *gin.Context) {
		if err := process(c, cfg); err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		c.Next()
	})
}

func HandlerFactory(l logging.Logger, next krakendgin.HandlerFactory) krakendgin.HandlerFactory {
	return func(remote *config.EndpointConfig, p proxy.Proxy) gin.HandlerFunc {
		handlerFunc := next(remote, p)

		cfg, err := lua.Parse(l, remote.ExtraConfig, router.Namespace)
		if err != nil {
			l.Debug("lua:", err.Error())
			return handlerFunc
		}

		return func(c *gin.Context) {
			if err := process(c, cfg); err != nil {
				c.AbortWithError(http.StatusInternalServerError, err)
				return
			}

			handlerFunc(c)
		}
	}
}

func process(c *gin.Context, cfg lua.Config) error {
	b := binder.New(binder.Options{
		SkipOpenLibs:        !cfg.AllowOpenLibs,
		IncludeGoStackTrace: true,
	})

	registerCtxTable(c, b)

	for _, source := range cfg.Sources {
		src, ok := cfg.Get(source)
		if !ok {
			return lua.ErrUnknownSource(source)
		}
		if err := b.DoString(src); err != nil {
			return err
		}
	}

	return b.DoString(cfg.PreCode)
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
	t.Dynamic("query", r.query)
	t.Dynamic("params", r.params)
	t.Dynamic("headers", r.requestHeaders)
	t.Dynamic("body", r.requestBody)
}

type ginContext struct {
	*gin.Context
}

func (r *ginContext) method(c *binder.Context) error {
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

func (r *ginContext) url(c *binder.Context) error {
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

func (r *ginContext) query(c *binder.Context) error {
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

func (r *ginContext) params(c *binder.Context) error {
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

func (r *ginContext) requestHeaders(c *binder.Context) error {
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
		req.Request.Header.Set(c.Arg(2).String(), c.Arg(3).String())
	}

	return nil
}

func (r *ginContext) requestBody(c *binder.Context) error {
	req, ok := c.Arg(1).Data().(*ginContext)
	if !ok {
		return errContextExpected
	}

	if c.Top() == 2 {
		req.Request.Body = ioutil.NopCloser(bytes.NewBufferString(c.Arg(2).String()))
		return nil
	}

	var b []byte
	if req.Request.Body != nil {
		b, _ = ioutil.ReadAll(req.Request.Body)
		req.Request.Body.Close()
	}
	req.Request.Body = ioutil.NopCloser(bytes.NewBuffer(b))
	c.Push().String(string(b))

	return nil
}

var (
	errNeedsArguments  = errors.New("need arguments")
	errContextExpected = errors.New("ginContext expected")
)
