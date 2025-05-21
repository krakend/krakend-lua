package proxy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/textproto"
	"net/url"

	"github.com/krakendio/binder"
	lua "github.com/krakendio/krakend-lua/v2"
	"github.com/luraproject/lura/v2/proxy"
	glua "github.com/yuin/gopher-lua"
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
	t.Dynamic("headerList", r.headerList)
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
		c.Push().Data(lua.NewTableFromStringMap(req.Params), "luaTable")
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
		c.Push().Data(lua.NewTableFromStringSliceMap(req.Headers), "luaTable")
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

		_, isNil := c.Arg(3).Any().(*glua.LNilType)
		if isNil {
			delete(req.Headers, key)
			return nil
		}
		req.Headers[key] = []string{c.Arg(3).String()}
	}

	return nil
}

func (*ProxyRequest) headerList(c *binder.Context) error {
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
		req.Headers[key] = d
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
