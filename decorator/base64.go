package decorator

import (
	encoder "encoding/base64"

	lua "github.com/krakend/krakend-lua/v3"
)

func RegisterBase64(b *lua.Binder) {
	t := b.Table("base64")
	t.Static("encode", encodeHandler(encoder.StdEncoding))
	t.Static("decode", decodeHandler(encoder.StdEncoding))
	t.Static("urlencode", encodeHandler(encoder.URLEncoding))
	t.Static("urldecode", decodeHandler(encoder.URLEncoding))
}

func encodeHandler(enc *encoder.Encoding) lua.Handler {
	return func(c *lua.Context) error {
		if c.Top() != 1 {
			return errNeedArguments
		}

		var data []byte
		switch t := c.Arg(1).Any().(type) {
		case lua.NativeString:
			data = []byte(t)
		case lua.NativeNumber:
			data = []byte(t.String())
		default:
			return errInvalidType
		}

		c.Push().String(enc.EncodeToString(data))
		return nil
	}
}

func decodeHandler(enc *encoder.Encoding) lua.Handler {
	return func(c *lua.Context) error {
		if c.Top() != 1 {
			return errNeedArguments
		}

		t := c.Arg(1).String()
		enc, err := enc.DecodeString(t)
		if err != nil {
			return err
		}

		c.Push().String(string(enc))
		return nil
	}
}
