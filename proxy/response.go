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
	tab := b.Table("luaTable")
	tab.Dynamic("get", tableGet)
	tab.Dynamic("set", tableSet)
	tab.Dynamic("len", tableLen)
	list := b.Table("luaList")
	list.Dynamic("get", listGet)
	list.Dynamic("set", listSet)
	list.Dynamic("len", listLen)

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

	if c.Top() == 2 {
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
	c.Push().Data(&luaTable{data: resp.Data}, "luaTable")

	return nil
}

func tableLen(c *binder.Context) error {
	tab, ok := c.Arg(1).Data().(*luaTable)
	if !ok {
		return errResponseExpected
	}
	c.Push().Number(float64(len(tab.data)))
	return nil
}

func listLen(c *binder.Context) error {
	list, ok := c.Arg(1).Data().(*luaList)
	if !ok {
		return errResponseExpected
	}
	c.Push().Number(float64(len(list.data)))
	return nil
}

func tableGet(c *binder.Context) error {
	if c.Top() != 2 {
		return errNeedsArguments
	}
	tab, ok := c.Arg(1).Data().(*luaTable)
	if !ok {
		return errResponseExpected
	}
	data, ok := tab.data[c.Arg(2).String()]
	if !ok {
		return nil
	}
	switch t := data.(type) {
	case string:
		c.Push().String(t)
	case int:
		c.Push().Number(float64(t))
	case float64:
		c.Push().Number(t)
	case bool:
		c.Push().Bool(t)
	case []interface{}:
		c.Push().Data(&luaList{data: t}, "luaList")
	case map[string]interface{}:
		c.Push().Data(&luaTable{data: t}, "luaTable")
	}

	return nil
}

func listGet(c *binder.Context) error {
	if c.Top() != 2 {
		return errNeedsArguments
	}
	tab, ok := c.Arg(1).Data().(*luaList)
	if !ok {
		return errResponseExpected
	}
	index := int(c.Arg(2).Number())
	if index < 0 || index >= len(tab.data) {
		return nil
	}
	switch t := tab.data[index].(type) {
	case string:
		c.Push().String(t)
	case int:
		c.Push().Number(float64(t))
	case float64:
		c.Push().Number(t)
	case bool:
		c.Push().Bool(t)
	case []interface{}:
		c.Push().Data(&luaList{data: t}, "luaList")
	case map[string]interface{}:
		c.Push().Data(&luaTable{data: t}, "luaTable")
	}

	return nil
}

func tableSet(c *binder.Context) error {
	if c.Top() != 3 {
		return errNeedsArguments
	}
	tab, ok := c.Arg(1).Data().(*luaTable)
	if !ok {
		return errResponseExpected
	}
	key := c.Arg(2).String()
	switch t := c.Arg(3).Any().(type) {
	case lua.LString:
		tab.data[key] = c.Arg(3).String()
	case lua.LNumber:
		tab.data[key] = c.Arg(3).Number()
	case lua.LBool:
		tab.data[key] = c.Arg(3).Bool()
	case *lua.LTable:
		res := map[string]interface{}{}
		t.ForEach(func(k, v lua.LValue) {
			parseToTable(k, v, res)
		})
		tab.data[key] = res
	case *lua.LUserData:
		switch v := t.Value.(type) {
		case *luaTable:
			tab.data[key] = v.data
		case *luaList:
			tab.data[key] = v.data
		}
	}

	return nil
}

func listSet(c *binder.Context) error {
	if c.Top() != 3 {
		return errNeedsArguments
	}
	tab, ok := c.Arg(1).Data().(*luaList)
	if !ok {
		return errResponseExpected
	}
	key := int(c.Arg(2).Number())
	if key < 0 || key >= len(tab.data) {
		return nil
	}
	switch t := c.Arg(3).Any().(type) {
	case lua.LString:
		tab.data[key] = c.Arg(3).String()
	case lua.LNumber:
		tab.data[key] = c.Arg(3).Number()
	case lua.LBool:
		tab.data[key] = c.Arg(3).Bool()
	case *lua.LTable:
		res := map[string]interface{}{}
		t.ForEach(func(k, v lua.LValue) {
			parseToTable(k, v, res)
		})
		tab.data[key] = res
	case *lua.LUserData:
		switch v := t.Value.(type) {
		case *luaTable:
			tab.data[key] = v.data
		case *luaList:
			tab.data[key] = v.data
		}
	}

	return nil
}

type luaTable struct {
	data map[string]interface{}
}

func (l *luaTable) get(k string) interface{} {
	return l.data[k]
}

func (l *luaTable) set(k string, v interface{}) {
	l.data[k] = v
}

type luaList struct {
	data []interface{}
}

func (l *luaList) get(k int) interface{} {
	return l.data[k]
}

func (l *luaList) set(k int, v interface{}) {
	l.data[k] = v
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
		switch v := v.(*lua.LUserData).Value.(type) {
		case *luaTable:
			acc[k.String()] = v.data
		case *luaList:
			acc[k.String()] = v.data
		}
	case lua.LTTable:
		res := map[string]interface{}{}
		v.(*lua.LTable).ForEach(func(k, v lua.LValue) {
			parseToTable(k, v, res)
		})
		acc[k.String()] = res
	}
}
