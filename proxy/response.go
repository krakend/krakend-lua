package proxy

import (
	"bytes"
	"errors"
	"io/ioutil"

	"github.com/alexeyco/binder"
	"github.com/devopsfaith/krakend/proxy"
	lua "github.com/yuin/gopher-lua"
)

func registerResponseTable(resp *proxy.Response, b *binder.Binder) {
	r := &response{resp}

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

type response struct {
	*proxy.Response
}

var errResponseExpected = errors.New("response expected")

func (r *response) isComplete(c *binder.Context) error {
	resp, ok := c.Arg(1).Data().(*response)
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

func (r *response) statusCode(c *binder.Context) error {
	resp, ok := c.Arg(1).Data().(*response)
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

func (r *response) headers(c *binder.Context) error {
	resp, ok := c.Arg(1).Data().(*response)
	if !ok {
		return errResponseExpected
	}
	switch c.Top() {
	case 1:
		return errNeedsArguments
	case 2:
		c.Push().String(resp.Metadata.Headers[c.Arg(2).String()][0])
	case 3:
		resp.Metadata.Headers[c.Arg(2).String()] = []string{c.Arg(3).String()}
	}

	return nil
}

func (r *response) body(c *binder.Context) error {
	resp, ok := c.Arg(1).Data().(*response)
	if !ok {
		return errResponseExpected
	}

	if c.Top() == 1 {
		resp.Io = bytes.NewBufferString(c.Arg(2).String())
		return nil
	}

	var b []byte
	if resp.Io != nil {
		b, _ = ioutil.ReadAll(resp.Io)
	}
	resp.Io = bytes.NewBuffer(b)
	c.Push().String(string(b))

	return nil
}

func (r *response) data(c *binder.Context) error {
	resp, ok := c.Arg(1).Data().(*response)
	if !ok {
		return errResponseExpected
	}
	switch c.Top() {
	case 1:
		return errNeedsArguments
	case 2:
		data := resp.Data[c.Arg(2).String()]
		switch t := data.(type) {
		case string:
			c.Push().String(t)
		case int:
			c.Push().Number(float64(t))
		case float64:
			c.Push().Number(t)
		case bool:
			c.Push().Bool(t)
		default:
			c.Push().Data(t, "table")
		}
	case 3:
		key := c.Arg(2).String()
		switch t := c.Arg(3).Any().(type) {
		case lua.LString:
			resp.Data[key] = c.Arg(3).String()
		case lua.LNumber:
			resp.Data[key] = c.Arg(3).Number()
		case lua.LBool:
			resp.Data[key] = c.Arg(3).Bool()
		case *lua.LTable:
			res := map[string]interface{}{}
			t.ForEach(func(k, v lua.LValue) {
				parseToTable(k, v, res)
			})
			resp.Data[key] = res
		case *lua.LUserData:
			resp.Data[key] = t.Value
		}
	}

	return nil
}

func parseToTable(k, v lua.LValue, acc map[string]interface{}) {
	switch v.Type() {
	case lua.LTString:
		acc[k.String()] = v.String()
	case lua.LTBool:
		acc[k.String()] = lua.LVAsBool(v)
	case lua.LTNumber:
		f := float64(v.(lua.LNumber))
		if f == float64(int64(v.(lua.LNumber))) {
			acc[k.String()] = int(v.(lua.LNumber))
		} else {
			acc[k.String()] = f
		}
	case lua.LTUserData:
		acc[k.String()] = v.(*lua.LUserData).Value
	case lua.LTTable:
		res := map[string]interface{}{}
		v.(*lua.LTable).ForEach(func(k, v lua.LValue) {
			parseToTable(k, v, res)
		})
		acc[k.String()] = res
	}
}
