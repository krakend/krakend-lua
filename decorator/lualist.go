package decorator

import (
	"encoding/json"

	"github.com/krakend/binder"
	lua "github.com/krakend/krakend-lua/v2"
)

func RegisterLuaList(b *binder.Binder) {
	list := b.Table("luaList")
	list.Static("new", func(c *binder.Context) error {
		c.Push().Data(&lua.List{Data: []interface{}{}}, "luaList")
		return nil
	})
	list.Dynamic("get", listGet)
	list.Dynamic("set", listSet)
	list.Dynamic("len", listLen)
	list.Dynamic("del", listDel)
}

func listLen(c *binder.Context) error {
	list, ok := c.Arg(1).Data().(*lua.List)
	if !ok {
		return ErrResponseExpected
	}
	c.Push().Number(float64(len(list.Data)))
	return nil
}

func listGet(c *binder.Context) error {
	if c.Top() != 2 {
		return ErrNeedsArguments
	}
	tab, ok := c.Arg(1).Data().(*lua.List)
	if !ok {
		return ErrResponseExpected
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
		c.Push().Data(&lua.List{Data: t}, "luaList")
	case map[string]interface{}:
		c.Push().Data(&lua.Table{Data: t}, "luaTable")
	}

	return nil
}

func listSet(c *binder.Context) error {
	if c.Top() != 3 {
		return ErrNeedsArguments
	}
	tab, ok := c.Arg(1).Data().(*lua.List)
	if !ok {
		return ErrResponseExpected
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

func listDel(c *binder.Context) error {
	if c.Top() != 2 {
		return ErrNeedsArguments
	}
	tab, ok := c.Arg(1).Data().(*lua.List)
	if !ok {
		return ErrResponseExpected
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
