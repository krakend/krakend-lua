package decorator

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/krakendio/binder"
	lua "github.com/krakendio/krakend-lua/v2"
	"github.com/luraproject/lura/v2/transport/http/client"
)

func RegisterLuaTable(b *binder.Binder) {
	tab := b.Table("luaTable")
	tab.Static("new", func(c *binder.Context) error {
		c.Push().Data(&lua.Table{Data: map[string]interface{}{}}, "luaTable")
		return nil
	})
	tab.Dynamic("get", tableGet)
	tab.Dynamic("set", tableSet)
	tab.Dynamic("len", tableLen)
	tab.Dynamic("del", tableDel)
	tab.Dynamic("keys", tableKeys)
	tab.Dynamic("keyExists", tableKeyExists)
}

func tableGet(c *binder.Context) error {
	if c.Top() != 2 {
		return ErrNeedsArguments
	}
	tab, ok := c.Arg(1).Data().(*lua.Table)
	if !ok {
		return ErrResponseExpected
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
		c.Push().Data(&lua.List{Data: t}, "luaList")
	case map[string]interface{}:
		c.Push().Data(&lua.Table{Data: t}, "luaTable")
	case client.HTTPResponseError:
		c.Push().Data(&lua.Table{Data: clientErrorToMap(t)}, "luaTable")
	case client.NamedHTTPResponseError:
		d := clientErrorToMap(t.HTTPResponseError)
		d["name"] = t.Name()
		c.Push().Data(&lua.Table{Data: d}, "luaTable")
	default:
		return errors.New(fmt.Sprintf("unknown type (%T) %v", t, t))
	}

	return nil
}

func tableSet(c *binder.Context) error {
	if c.Top() != 3 {
		return ErrNeedsArguments
	}
	tab, ok := c.Arg(1).Data().(*lua.Table)
	if !ok {
		return ErrResponseExpected
	}
	key := c.Arg(2).String()
	switch t := c.Arg(3).Any().(type) {
	case lua.NativeString:
		tab.Data[key] = c.Arg(3).String()
	case lua.NativeNumber:
		tab.Data[key] = c.Arg(3).Number()
	case lua.NativeBool:
		tab.Data[key] = c.Arg(3).Bool()
	case *lua.NativeTable:
		res := map[string]interface{}{}
		t.ForEach(func(k, v lua.NativeValue) {
			lua.ParseToTable(k, v, res)
		})
		tab.Data[key] = res
	case *lua.NativeUserData:
		if t.Value == nil {
			tab.Data[key] = nil
		} else {
			switch v := t.Value.(type) {
			case *lua.Table:
				tab.Data[key] = v.Data
			case *lua.List:
				tab.Data[key] = v.Data
			}
		}
	}

	return nil
}

func tableKeys(c *binder.Context) error {
	tab, ok := c.Arg(1).Data().(*lua.Table)
	if !ok {
		return ErrResponseExpected
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
	c.Push().Data(&lua.List{Data: keys}, "luaList")
	return nil
}

func tableKeyExists(c *binder.Context) error {
	if c.Top() != 2 {
		return ErrNeedsArguments
	}
	tab, ok := c.Arg(1).Data().(*lua.Table)
	if !ok {
		return ErrResponseExpected
	}
	_, ok = tab.Data[c.Arg(2).String()]
	c.Push().Bool(ok)
	return nil
}

func tableLen(c *binder.Context) error {
	tab, ok := c.Arg(1).Data().(*lua.Table)
	if !ok {
		return ErrResponseExpected
	}
	c.Push().Number(float64(len(tab.Data)))
	return nil
}

func tableDel(c *binder.Context) error {
	if c.Top() != 2 {
		return ErrNeedsArguments
	}
	tab, ok := c.Arg(1).Data().(*lua.Table)
	if !ok {
		return ErrResponseExpected
	}
	delete(tab.Data, c.Arg(2).String())
	return nil
}

func clientErrorToMap(err client.HTTPResponseError) map[string]interface{} {
	return map[string]interface{}{
		"http_status_code":   err.StatusCode(),
		"http_body":          err.Error(),
		"http_body_encoding": err.Encoding(),
	}
}
