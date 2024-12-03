package proxy

import (
	"bytes"
	"errors"
	"io"
	"net/http"

	"github.com/krakendio/binder"
	lua "github.com/krakendio/krakend-lua/v2"
	"github.com/luraproject/lura/v2/proxy"
)

func registerResponseTable(resp *proxy.Response, b *binder.Binder) {
	r := &ProxyResponse{resp}
	if r.Metadata.Headers == nil {
		r.Metadata.Headers = map[string][]string{}
	}
	if r.Data == nil {
		r.Data = map[string]interface{}{}
	}

	t := b.Table("response")

	t.Static("load", func(c *binder.Context) error {
		c.Push().Data(r, "response")
		return nil
	})

	t.Dynamic("isComplete", r.isComplete)
	t.Dynamic("statusCode", r.statusCode)
	t.Dynamic("data", r.data)
	t.Dynamic("headers", r.headers)
	t.Dynamic("body", r.body)
}

type ProxyResponse struct {
	*proxy.Response
}

var errResponseExpected = errors.New("response expected")

func (*ProxyResponse) isComplete(c *binder.Context) error {
	resp, ok := c.Arg(1).Data().(*ProxyResponse)
	if !ok {
		return errResponseExpected
	}

	if c.Top() == 1 {
		c.Push().Bool(resp.IsComplete)
	} else {
		resp.IsComplete = c.Arg(2).Bool()
	}

	return nil
}

func (*ProxyResponse) statusCode(c *binder.Context) error {
	resp, ok := c.Arg(1).Data().(*ProxyResponse)
	if !ok {
		return errResponseExpected
	}

	if c.Top() == 1 {
		c.Push().Number(float64(resp.Metadata.StatusCode))
	} else {
		resp.Metadata.StatusCode = int(c.Arg(2).Number())
	}

	return nil
}

func (*ProxyResponse) headers(c *binder.Context) error {
	resp, ok := c.Arg(1).Data().(*ProxyResponse)
	if !ok {
		return errResponseExpected
	}
	switch c.Top() {
	case 1:
		return errNeedsArguments
	case 2:
		headers := resp.Metadata.Headers[c.Arg(2).String()]
		if len(headers) == 0 {
			c.Push().String("")
		} else {
			c.Push().String(headers[0])
		}
	case 3:
		resp.Metadata.Headers[http.CanonicalHeaderKey(c.Arg(2).String())] = []string{c.Arg(3).String()}
	}

	return nil
}

func (*ProxyResponse) body(c *binder.Context) error {
	resp, ok := c.Arg(1).Data().(*ProxyResponse)
	if !ok {
		return errResponseExpected
	}

	if c.Top() == 2 {
		resp.Io = bytes.NewBufferString(c.Arg(2).String())
		return nil
	}

	var b []byte
	if resp.Io != nil {
		b, _ = io.ReadAll(resp.Io)
	}
	resp.Io = bytes.NewBuffer(b)
	c.Push().String(string(b))

	return nil
}

func (*ProxyResponse) data(c *binder.Context) error {
	resp, ok := c.Arg(1).Data().(*ProxyResponse)
	if !ok {
		return errResponseExpected
	}
	c.Push().Data(&lua.Table{Data: resp.Data}, "luaTable")

	return nil
}
