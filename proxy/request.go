package proxy

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/url"

	"github.com/alexeyco/binder"
	"github.com/devopsfaith/krakend/proxy"
)

func registerRequestTable(req *proxy.Request, b *binder.Binder) {
	r := &request{req}

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

type request struct {
	*proxy.Request
}

var errRequestExpected = errors.New("request expected")

func (r *request) method(c *binder.Context) error {
	req, ok := c.Arg(1).Data().(*request)
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

func (r *request) path(c *binder.Context) error {
	req, ok := c.Arg(1).Data().(*request)
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

func (r *request) query(c *binder.Context) error {
	req, ok := c.Arg(1).Data().(*request)
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

func (r *request) url(c *binder.Context) error {
	req, ok := c.Arg(1).Data().(*request)
	if !ok {
		return errRequestExpected
	}

	if c.Top() == 1 {
		c.Push().String(req.URL.String())
	} else {
		req.URL, _ = url.Parse(c.Arg(2).String())
	}

	return nil
}

func (r *request) params(c *binder.Context) error {
	req, ok := c.Arg(1).Data().(*request)
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

func (r *request) headers(c *binder.Context) error {
	req, ok := c.Arg(1).Data().(*request)
	if !ok {
		return errRequestExpected
	}
	switch c.Top() {
	case 1:
		return errNeedsArguments
	case 2:
		c.Push().String(req.Headers[c.Arg(2).String()][0])
	case 3:
		req.Headers[c.Arg(2).String()] = []string{c.Arg(3).String()}
	}

	return nil
}

func (r *request) body(c *binder.Context) error {
	req, ok := c.Arg(1).Data().(*request)
	if !ok {
		return errRequestExpected
	}

	if c.Top() == 2 {
		req.Body = ioutil.NopCloser(bytes.NewBufferString(c.Arg(2).String()))
		return nil
	}

	var b []byte
	if req.Body != nil {
		b, _ = ioutil.ReadAll(req.Body)
		req.Body.Close()
	}
	req.Body = ioutil.NopCloser(bytes.NewBuffer(b))
	c.Push().String(string(b))

	return nil
}
