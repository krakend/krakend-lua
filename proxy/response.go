package proxy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/krakendio/binder"
	lua "github.com/krakendio/krakend-lua/v2"
	"github.com/luraproject/lura/v2/proxy"
	glua "github.com/yuin/gopher-lua"
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
	t.Dynamic("headerList", r.headerList)
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
		c.Push().Data(lua.NewTableFromStringSliceMap(resp.Metadata.Headers), "luaTable")
	case 2:
		headers := resp.Metadata.Headers[c.Arg(2).String()]
		if len(headers) == 0 {
			c.Push().String("")
		} else {
			c.Push().String(headers[0])
		}
	case 3:
		key := http.CanonicalHeaderKey(c.Arg(2).String())

		_, isNil := c.Arg(3).Any().(*glua.LNilType)
		if isNil {
			delete(resp.Metadata.Headers, key)
			return nil
		}
		resp.Metadata.Headers[key] = []string{c.Arg(3).String()}
	}

	return nil
}

func (*ProxyResponse) headerList(c *binder.Context) error {
	resp, ok := c.Arg(1).Data().(*ProxyResponse)
	if !ok {
		return errRequestExpected
	}
	switch c.Top() {
	case 1:
		return errNeedsArguments
	case 2:
		key := http.CanonicalHeaderKey(c.Arg(2).String())

		headers := resp.Metadata.Headers[key]
		d := make([]interface{}, len(headers))
		for i := range headers {
			d[i] = headers[i]
		}
		c.Push().Data(&lua.List{Data: d}, "luaList")
	case 3:
		key := http.CanonicalHeaderKey(c.Arg(2).String())

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
		resp.Metadata.Headers[key] = d
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
