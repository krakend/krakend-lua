package mux

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/krakendio/binder"
	lua "github.com/krakendio/krakend-lua/v2"
	"github.com/krakendio/krakend-lua/v2/router"
	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/logging"
	"github.com/luraproject/lura/v2/proxy"
	mux "github.com/luraproject/lura/v2/router/mux"
)

func RegisterMiddleware(l logging.Logger, e config.ExtraConfig, pe mux.ParamExtractor, mws []mux.HandlerMiddleware) []mux.HandlerMiddleware {
	logPrefix := "[Service: Mux][Lua]"
	cfg, err := lua.Parse(l, e, router.Namespace)
	if err != nil {
		if err != lua.ErrNoExtraConfig {
			l.Debug(logPrefix, err.Error())
		}
		return mws
	}

	l.Debug(logPrefix, "Middleware is now ready")

	return append(mws, &middleware{pe: pe, cfg: cfg})
}

type middleware struct {
	pe  mux.ParamExtractor
	cfg lua.Config
}

func (hm *middleware) Handler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := process(r, hm.pe, &hm.cfg); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		h.ServeHTTP(w, r)
	})
}

func HandlerFactory(l logging.Logger, next mux.HandlerFactory, pe mux.ParamExtractor) mux.HandlerFactory {
	return func(remote *config.EndpointConfig, p proxy.Proxy) http.HandlerFunc {
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

		return func(w http.ResponseWriter, r *http.Request) {
			if err := process(r, pe, &cfg); err != nil {
				err = lua.ToError(err)
				if errhttp, ok := err.(errHTTP); ok {
					if e, ok := err.(errHTTPWithContentType); ok {
						fmt.Println(e.Encoding())
						w.Header().Add("content-type", e.Encoding())
					}
					w.WriteHeader(errhttp.StatusCode())
					w.Write([]byte(err.Error()))
					return
				}

				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			handlerFunc(w, r)
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

func process(r *http.Request, pe mux.ParamExtractor, cfg *lua.Config) error {
	b := binder.New(binder.Options{
		SkipOpenLibs:        !cfg.AllowOpenLibs,
		IncludeGoStackTrace: true,
	})

	lua.RegisterErrors(b)
	registerRequestTable(r, pe, b)

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

func registerRequestTable(r *http.Request, pe mux.ParamExtractor, b *binder.Binder) {
	mctx := &muxContext{
		Request: r,
		pe:      pe,
	}

	t := b.Table("ctx")

	t.Static("load", func(c *binder.Context) error {
		c.Push().Data(mctx, "ctx")
		return nil
	})

	t.Dynamic("method", mctx.method)
	t.Dynamic("url", mctx.url)
	t.Dynamic("query", mctx.query)
	t.Dynamic("params", mctx.params)
	t.Dynamic("headers", mctx.headers)
	t.Dynamic("body", mctx.body)
}

type muxContext struct {
	*http.Request
	pe mux.ParamExtractor
}

func (*muxContext) method(c *binder.Context) error {
	req, ok := c.Arg(1).Data().(*muxContext)
	if !ok {
		return errContextExpected
	}

	if c.Top() == 1 {
		c.Push().String(req.Method)
	} else {
		req.Method = c.Arg(2).String()
	}

	return nil
}

func (*muxContext) url(c *binder.Context) error {
	req, ok := c.Arg(1).Data().(*muxContext)
	if !ok {
		return errContextExpected
	}

	if c.Top() == 1 {
		c.Push().String(req.URL.String())
	} else {
		req.URL, _ = url.Parse(c.Arg(2).String())
	}

	return nil
}

func (*muxContext) query(c *binder.Context) error {
	req, ok := c.Arg(1).Data().(*muxContext)
	if !ok {
		return errContextExpected
	}

	switch c.Top() {
	case 1:
		return errNeedsArguments
	case 2:
		c.Push().String(req.URL.Query().Get(c.Arg(2).String()))
	case 3:
		q := req.URL.Query()
		q.Set(c.Arg(2).String(), c.Arg(3).String())
		req.URL.RawQuery = q.Encode()
	}

	return nil
}

func (*muxContext) params(_ *binder.Context) error {
	return nil
}

func (*muxContext) headers(c *binder.Context) error {
	req, ok := c.Arg(1).Data().(*muxContext)
	if !ok {
		return errContextExpected
	}
	switch c.Top() {
	case 1:
		return errNeedsArguments
	case 2:
		c.Push().String(req.Header.Get(c.Arg(2).String()))
	case 3:
		req.Header.Set(c.Arg(2).String(), c.Arg(3).String())
	}

	return nil
}

func (*muxContext) body(c *binder.Context) error {
	req, ok := c.Arg(1).Data().(*muxContext)
	if !ok {
		return errContextExpected
	}

	if c.Top() == 2 {
		req.Body = io.NopCloser(bytes.NewBufferString(c.Arg(2).String()))
		return nil
	}

	var b []byte
	if req.Body != nil {
		b, _ = io.ReadAll(req.Body)
		req.Body.Close()
	}
	req.Body = io.NopCloser(bytes.NewBuffer(b))
	c.Push().String(string(b))

	return nil
}

var (
	errNeedsArguments  = errors.New("need arguments")
	errContextExpected = errors.New("muxContext expected")
)
