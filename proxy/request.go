package proxy

import (
	"bytes"
	"errors"
	"io"
	"net/textproto"
	"net/url"

	"github.com/krakendio/binder"
	"github.com/luraproject/lura/v2/proxy"
)

func registerRequestTable(req *proxy.Request, b *binder.Binder) {
	r := &ProxyRequest{req}

	t := b.Table("request")

	t.Static("load", func(c *binder.Context) error {
		c.Push().Data(r, "request")
		return nil
	})

	t.Dynamic("method", r.method)
	t.Dynamic("path", r.path)
	t.Dynamic("query", r.query)
	t.Dynamic("url", r.url)
	t.Dynamic("params", r.params)
	t.Dynamic("headers", r.headers)
	t.Dynamic("body", r.body)
}

type ProxyRequest struct {
	*proxy.Request
}

var errRequestExpected = errors.New("request expected")

func (*ProxyRequest) method(c *binder.Context) error {
	req, ok := c.Arg(1).Data().(*ProxyRequest)
	if !ok {
		return errRequestExpected
	}

	if c.Top() == 1 {
		c.Push().String(req.Method)
	} else {
		req.Method = c.Arg(2).String()
	}

	return nil
}

func (*ProxyRequest) path(c *binder.Context) error {
	req, ok := c.Arg(1).Data().(*ProxyRequest)
	if !ok {
		return errRequestExpected
	}

	if c.Top() == 1 {
		c.Push().String(req.Path)
	} else {
		req.Path = c.Arg(2).String()
	}

	return nil
}

func (*ProxyRequest) query(c *binder.Context) error {
	req, ok := c.Arg(1).Data().(*ProxyRequest)
	if !ok {
		return errRequestExpected
	}

	if c.Top() == 1 {
		c.Push().String(req.Query.Encode())
	} else {
		req.Query, _ = url.ParseQuery(c.Arg(2).String())
	}

	return nil
}

func (*ProxyRequest) url(c *binder.Context) error {
	req, ok := c.Arg(1).Data().(*ProxyRequest)
	if !ok {
		return errRequestExpected
	}

	if c.Top() > 1 {
		req.URL, _ = url.Parse(c.Arg(2).String())
		return nil
	}

	if req.URL == nil {
		c.Push().String("")
		return nil
	}

	c.Push().String(req.URL.String())
	return nil
}

func (*ProxyRequest) params(c *binder.Context) error {
	req, ok := c.Arg(1).Data().(*ProxyRequest)
	if !ok {
		return errRequestExpected
	}
	switch c.Top() {
	case 1:
		return errNeedsArguments
	case 2:
		c.Push().String(req.Params[c.Arg(2).String()])
	case 3:
		req.Params[c.Arg(2).String()] = c.Arg(3).String()
	}

	return nil
}

func (*ProxyRequest) headers(c *binder.Context) error {
	req, ok := c.Arg(1).Data().(*ProxyRequest)
	if !ok {
		return errRequestExpected
	}
	switch c.Top() {
	case 1:
		return errNeedsArguments
	case 2:
		key := textproto.CanonicalMIMEHeaderKey(c.Arg(2).String())
		headers := req.Headers[key]
		if len(headers) == 0 {
			c.Push().String("")
		} else {
			c.Push().String(headers[0])
		}
	case 3:
		key := textproto.CanonicalMIMEHeaderKey(c.Arg(2).String())
		req.Headers[key] = []string{c.Arg(3).String()}
	}

	return nil
}

func (*ProxyRequest) body(c *binder.Context) error {
	req, ok := c.Arg(1).Data().(*ProxyRequest)
	if !ok {
		return errRequestExpected
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
