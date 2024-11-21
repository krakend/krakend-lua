package decorator

import (
	"encoding/json"

	"github.com/krakendio/binder"
	lua "github.com/krakendio/krakend-lua/v2"
)

func RegisterJson(b *binder.Binder) {
	tab := b.Table("json")
	tab.Static("unmarshal", fromJson)
	tab.Static("marshal", toJson)
}

func fromJson(c *binder.Context) error {
	if c.Top() != 1 {
		return ErrNeedsArguments
	}
	data := new(interface{})
	err := json.Unmarshal([]byte(c.Arg(1).String()), data)
	if err != nil {
		return err
	}

	switch v := (*data).(type) {
	case string:
		c.Push().String(v)
	case json.Number:
		n, _ := v.Float64()
		c.Push().Number(n)
	case int:
		c.Push().Number(float64(v))
	case float64:
		c.Push().Number(v)
	case bool:
		c.Push().Bool(v)
	case []interface{}:
		c.Push().Data(&lua.List{Data: v}, "luaList")
	case map[string]interface{}:
		c.Push().Data(&lua.Table{Data: v}, "luaTable")
	}
	return nil
}

func toJson(c *binder.Context) error {
	if c.Top() != 1 {
		return ErrNeedsArguments
	}
	switch t := c.Arg(1).Any().(type) {
	case lua.NativeString:
		return marshal(c, c.Arg(1).String())
	case lua.NativeNumber:
		return marshal(c, c.Arg(1).Number())
	case lua.NativeBool:
		return marshal(c, c.Arg(1).Bool())
	case *lua.NativeTable:
		res := map[string]interface{}{}
		t.ForEach(func(k, v lua.NativeValue) {
			lua.ParseToTable(k, v, res)
		})
		return marshal(c, res)
	case *lua.NativeUserData:
		if t.Value == nil {
			return marshal(c, nil)
		} else {
			switch v := t.Value.(type) {
			case *lua.Table:
				return marshal(c, v.Data)
			case *lua.List:
				return marshal(c, v.Data)
			}
		}
	}

	return nil
}

func marshal(c *binder.Context, v interface{}) error {
	b, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		return err
	}
	c.Push().String(string(b))
	return nil
}
