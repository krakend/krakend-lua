package decorator

import (
	"errors"
	"fmt"

	"github.com/clbanning/mxj/v2"
	lua "github.com/krakend/krakend-lua/v3"
	"golang.org/x/net/html/charset"
)

func RegisterXml(b *lua.Binder) {
	t := b.Table("xml")
	t.Static("marshal", toXmlHandler())
	t.Static("unmarshal", fromXmlHandler())
}

func fromXmlHandler() lua.Handler {
	return func(c *lua.Context) error {
		if c.Top() != 1 {
			return errNeedArguments
		}

		mxj.XmlCharsetReader = charset.NewReaderLabel
		data, err := mxj.NewMapXml([]byte(c.Arg(1).String()))
		if err != nil {
			return fmt.Errorf("error parsing xml: %s", err.Error())
		}

		c.Push().Data(&lua.Table{Data: data}, "luaTable")

		return nil
	}
}

func toXmlHandler() lua.Handler {
	return func(c *lua.Context) error {
		if c.Top() != 1 {
			return errNeedArguments
		}
		switch t := c.Arg(1).Any().(type) {
		case *lua.NativeTable:
			m, isSlice := lua.MapNativeTable(t)
			if isSlice {
				return marshalMap(c, map[string]interface{}{"data": m})
			}
			return marshalMap(c, m)
		case *lua.NativeUserData:
			if t.Value == nil {
				return marshalMap(c, nil)
			} else {
				switch v := t.Value.(type) {
				case *lua.Table:
					return marshalMap(c, v.Data)
				case *lua.List:
					return errors.New("unsupported type for xml conversion")
				}
			}
		default:
			return errors.New("unsupported type for xml conversion")
		}
		return nil
	}
}

func marshalMap(c *lua.Context, input interface{}) error {
	v, ok := input.(map[string]interface{})
	if !ok {
		return errInvalidType
	}

	b := mxj.Map(v)
	x, err := b.XmlIndent("", "\t")
	if err != nil {
		return err
	}
	c.Push().String(string(x))
	return nil
}
