package decorator

import (
	enc "encoding/json"

	lua "github.com/krakend/krakend-lua/v3"
)

func RegisterJson(b *lua.Binder) {
	t := b.Table("json")
	t.Static("marshal", toJsonHandler())
	t.Static("unmarshal", fromJsonHandler())
}

func fromJsonHandler() lua.Handler {
	return func(c *lua.Context) error {
		if c.Top() != 1 {
			return errNeedArguments
		}
		data := new(interface{})
		err := enc.Unmarshal([]byte(c.Arg(1).String()), data)
		if err != nil {
			return err
		}

		switch v := (*data).(type) {
		case string:
			c.Push().String(v)
		case enc.Number:
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
}

func toJsonHandler() lua.Handler {
	return func(c *lua.Context) error {
		if c.Top() != 1 {
			return errNeedArguments
		}
		switch t := c.Arg(1).Any().(type) {
		case lua.NativeString:
			return marshalJson(c, c.Arg(1).String())
		case lua.NativeNumber:
			return marshalJson(c, c.Arg(1).Number())
		case lua.NativeBool:
			return marshalJson(c, c.Arg(1).Bool())
		case *lua.NativeTable:
			m, _ := lua.MapNativeTable(t)
			return marshalJson(c, m)
		case *lua.NativeUserData:
			if t.Value == nil {
				return marshalJson(c, nil)
			} else {
				switch v := t.Value.(type) {
				case *lua.Table:
					return marshalJson(c, v.Data)
				case *lua.List:
					return marshalJson(c, v.Data)
				}
			}
		}

		return nil
	}
}

func marshalJson(c *lua.Context, v interface{}) error {
	b, err := enc.MarshalIndent(v, "", "\t")
	if err != nil {
		return err
	}
	c.Push().String(string(b))
	return nil
}
