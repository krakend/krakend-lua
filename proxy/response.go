package proxy

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/krakendio/binder"
	"github.com/luraproject/lura/v2/proxy"
	"github.com/luraproject/lura/v2/transport/http/client"
	lua "github.com/yuin/gopher-lua"
)

func registerResponseTable(resp *proxy.Response, b *binder.Binder) {
	tab := b.Table("luaTable")
	tab.Static("new", func(c *binder.Context) error {
		c.Push().Data(&Table{Data: map[string]interface{}{}}, "luaTable")
		return nil
	})
	tab.Dynamic("get", tableGet)
	tab.Dynamic("set", tableSet)
	tab.Dynamic("len", tableLen)
	tab.Dynamic("del", tableDel)
	tab.Dynamic("keys", tableKeys)
	tab.Dynamic("keyExists", tableKeyExists)

	list := b.Table("luaList")
	list.Static("new", func(c *binder.Context) error {
		c.Push().Data(&List{Data: []interface{}{}}, "luaList")
		return nil
	})
	list.Dynamic("get", listGet)
	list.Dynamic("set", listSet)
	list.Dynamic("len", listLen)
	list.Dynamic("del", listDel)

	r := &response{resp}
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

type response struct {
	*proxy.Response
}

var errResponseExpected = errors.New("response expected")

func (*response) isComplete(c *binder.Context) error {
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

func (*response) statusCode(c *binder.Context) error {
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

func (*response) headers(c *binder.Context) error {
	resp, ok := c.Arg(1).Data().(*response)
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
		resp.Metadata.Headers[c.Arg(2).String()] = []string{c.Arg(3).String()}
	}

	return nil
}

func (*response) body(c *binder.Context) error {
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
		b, _ = io.ReadAll(resp.Io)
	}
	resp.Io = bytes.NewBuffer(b)
	c.Push().String(string(b))

	return nil
}

func (*response) data(c *binder.Context) error {
	resp, ok := c.Arg(1).Data().(*response)
	if !ok {
		return errResponseExpected
	}
	c.Push().Data(&Table{Data: resp.Data}, "luaTable")

	return nil
}

func tableKeyExists(c *binder.Context) error {
	if c.Top() != 2 {
		return errNeedsArguments
	}
	tab, ok := c.Arg(1).Data().(*Table)
	if !ok {
		return errResponseExpected
	}
	_, ok = tab.Data[c.Arg(2).String()]
	c.Push().Bool(ok)
	return nil
}

func tableKeys(c *binder.Context) error {
	tab, ok := c.Arg(1).Data().(*Table)
	if !ok {
		return errResponseExpected
	}
	var l []string
	for k := range tab.Data {
		l = append(l, k)
	}
	sort.Strings(l)
	keys := make([]interface{}, len(l))
	for k, v := range l {
		keys[k] = v
	}
	c.Push().Data(&List{Data: keys}, "luaList")
	return nil
}

func tableLen(c *binder.Context) error {
	tab, ok := c.Arg(1).Data().(*Table)
	if !ok {
		return errResponseExpected
	}
	c.Push().Number(float64(len(tab.Data)))
	return nil
}

func listLen(c *binder.Context) error {
	list, ok := c.Arg(1).Data().(*List)
	if !ok {
		return errResponseExpected
	}
	c.Push().Number(float64(len(list.Data)))
	return nil
}

func tableGet(c *binder.Context) error {
	if c.Top() != 2 {
		return errNeedsArguments
	}
	tab, ok := c.Arg(1).Data().(*Table)
	if !ok {
		return errResponseExpected
	}
	data, ok := tab.Data[c.Arg(2).String()]
	if !ok {
		return nil
	}
	if data == nil {
		c.Push().Data(nil, "luaNil")
		return nil
	}

	switch t := data.(type) {
	case string:
		c.Push().String(t)
	case json.Number:
		n, _ := t.Float64()
		c.Push().Number(n)
	case int:
		c.Push().Number(float64(t))
	case float64:
		c.Push().Number(t)
	case bool:
		c.Push().Bool(t)
	case []interface{}:
		c.Push().Data(&List{Data: t}, "luaList")
	case map[string]interface{}:
		c.Push().Data(&Table{Data: t}, "luaTable")
	case client.HTTPResponseError:
		c.Push().Data(&Table{Data: clientErrorToMap(t)}, "luaTable")
	case client.NamedHTTPResponseError:
		d := clientErrorToMap(t.HTTPResponseError)
		d["name"] = t.Name()
		c.Push().Data(&Table{Data: d}, "luaTable")
	default:
		return fmt.Errorf("unknown type (%T) %v", t, t)
	}

	return nil
}

func listGet(c *binder.Context) error {
	if c.Top() != 2 {
		return errNeedsArguments
	}
	tab, ok := c.Arg(1).Data().(*List)
	if !ok {
		return errResponseExpected
	}
	index := int(c.Arg(2).Number())
	if index < 0 || index >= len(tab.Data) {
		return nil
	}
	if tab.Data[index] == nil {
		c.Push().Data(nil, "luaNil")
		return nil
	}

	switch t := tab.Data[index].(type) {
	case string:
		c.Push().String(t)
	case json.Number:
		n, _ := t.Float64()
		c.Push().Number(n)
	case int:
		c.Push().Number(float64(t))
	case float64:
		c.Push().Number(t)
	case bool:
		c.Push().Bool(t)
	case []interface{}:
		c.Push().Data(&List{Data: t}, "luaList")
	case map[string]interface{}:
		c.Push().Data(&Table{Data: t}, "luaTable")
	}

	return nil
}

func tableSet(c *binder.Context) error {
	if c.Top() != 3 {
		return errNeedsArguments
	}
	tab, ok := c.Arg(1).Data().(*Table)
	if !ok {
		return errResponseExpected
	}
	key := c.Arg(2).String()
	switch t := c.Arg(3).Any().(type) {
	case lua.LString:
		tab.Data[key] = c.Arg(3).String()
	case lua.LNumber:
		tab.Data[key] = c.Arg(3).Number()
	case lua.LBool:
		tab.Data[key] = c.Arg(3).Bool()
	case *lua.LTable:
		res := map[string]interface{}{}
		t.ForEach(func(k, v lua.LValue) {
			parseToTable(k, v, res)
		})
		tab.Data[key] = res
	case *lua.LUserData:
		if t.Value == nil {
			tab.Data[key] = nil
		} else {
			switch v := t.Value.(type) {
			case *Table:
				tab.Data[key] = v.Data
			case *List:
				tab.Data[key] = v.Data
			}
		}
	}

	return nil
}

func listSet(c *binder.Context) error {
	if c.Top() != 3 {
		return errNeedsArguments
	}
	tab, ok := c.Arg(1).Data().(*List)
	if !ok {
		return errResponseExpected
	}
	key := int(c.Arg(2).Number())
	if key < 0 {
		return nil
	}
	if key >= len(tab.Data) {
		if cap(tab.Data) > key {
			for i := len(tab.Data); i < key; i++ {
				tab.Data = append(tab.Data, nil)
			}
		} else {
			newData := make([]interface{}, key+1)
			copy(newData, tab.Data)
			tab.Data = newData
		}
	}
	switch t := c.Arg(3).Any().(type) {
	case lua.LString:
		tab.Data[key] = c.Arg(3).String()
	case lua.LNumber:
		tab.Data[key] = c.Arg(3).Number()
	case lua.LBool:
		tab.Data[key] = c.Arg(3).Bool()
	case *lua.LTable:
		res := map[string]interface{}{}
		t.ForEach(func(k, v lua.LValue) {
			parseToTable(k, v, res)
		})
		tab.Data[key] = res
	case *lua.LUserData:
		if t.Value == nil {
			tab.Data[key] = nil
		} else {
			switch v := t.Value.(type) {
			case *Table:
				tab.Data[key] = v.Data
			case *List:
				tab.Data[key] = v.Data
			}
		}
	}

	return nil
}

func tableDel(c *binder.Context) error {
	if c.Top() != 2 {
		return errNeedsArguments
	}
	tab, ok := c.Arg(1).Data().(*Table)
	if !ok {
		return errResponseExpected
	}
	delete(tab.Data, c.Arg(2).String())
	return nil
}

func listDel(c *binder.Context) error {
	if c.Top() != 2 {
		return errNeedsArguments
	}
	tab, ok := c.Arg(1).Data().(*List)
	if !ok {
		return errResponseExpected
	}
	key := int(c.Arg(2).Number())
	if key < 0 || key >= len(tab.Data) {
		return nil
	}

	last := len(tab.Data) - 1
	if key < last {
		copy(tab.Data[key:], tab.Data[key+1:])
	}
	tab.Data[last] = nil
	tab.Data = tab.Data[:last]
	return nil
}

type Table struct {
	Data map[string]interface{}
}

type List struct {
	Data []interface{}
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
		userV := v.(*lua.LUserData)
		if userV.Value == nil {
			acc[k.String()] = nil
		} else {
			switch v := userV.Value.(type) {
			case *Table:
				acc[k.String()] = v.Data
			case *List:
				acc[k.String()] = v.Data
			}
		}
	case lua.LTTable:
		res := map[string]interface{}{}
		v.(*lua.LTable).ForEach(func(k, v lua.LValue) {
			parseToTable(k, v, res)
		})
		acc[k.String()] = res
	}
}

func clientErrorToMap(err client.HTTPResponseError) map[string]interface{} {
	return map[string]interface{}{
		"http_status_code":   err.StatusCode(),
		"http_body":          err.Error(),
		"http_body_encoding": err.Encoding(),
	}
}
