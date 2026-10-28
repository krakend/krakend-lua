package decorator

import (
	lua "github.com/krakend/krakend-lua/v3"
	enc "gopkg.in/yaml.v3"
)

func RegisterYaml(b *lua.Binder) {
	t := b.Table("yaml")
	t.Static("marshal", toYamlHandler())
	t.Static("unmarshal", fromYamlHandler())
}

func fromYamlHandler() lua.Handler {
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

func toYamlHandler() lua.Handler {
	return func(c *lua.Context) error {
		if c.Top() != 1 {
			return errNeedArguments
		}
		switch t := c.Arg(1).Any().(type) {
		case lua.NativeString:
			return marshalYaml(c, c.Arg(1).String())
		case lua.NativeNumber:
			return marshalYaml(c, c.Arg(1).Number())
		case lua.NativeBool:
			return marshalYaml(c, c.Arg(1).Bool())
		case *lua.NativeTable:
			m, _ := lua.MapNativeTable(t)
			return marshalYaml(c, m)
		case *lua.NativeUserData:
			if t.Value == nil {
				return marshalYaml(c, nil)
			} else {
				switch v := t.Value.(type) {
				case *lua.Table:
					return marshalYaml(c, v.Data)
				case *lua.List:
					return marshalYaml(c, v.Data)
				}
			}
		}

		return nil
	}
}

func marshalYaml(c *lua.Context, v interface{}) error {
	b, err := enc.Marshal(v)
	if err != nil {
		return err
	}
	c.Push().String(string(b))
	return nil
}
