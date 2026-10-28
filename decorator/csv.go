package decorator

import (
	"bytes"
	enc "encoding/csv"
	"errors"
	"fmt"
	"sort"
	"strings"

	lua "github.com/krakend/krakend-lua/v3"
)

const defaultDelimiter = ","

func RegisterCsv(b *lua.Binder) {
	t := b.Table("csv")
	t.Static("marshal", toCsvHandler())
	t.Static("unmarshal", fromCsvHandler())
}

func fromCsvHandler() lua.Handler {
	return func(c *lua.Context) error {
		if c.Top() == 0 {
			return errNeedArguments
		}

		r := enc.NewReader(strings.NewReader(c.Arg(1).String()))

		delimiter := defaultDelimiter
		if c.Top() == 2 {
			arg := c.Arg(2).String()
			if len(arg) > 0 {
				delimiter = arg
			}
		}
		r.Comma = rune(delimiter[0])

		raw, err := r.ReadAll()
		if err != nil {
			return err
		}

		if len(raw) == 0 {
			return nil
		}

		var records []interface{}

		keys := raw[0]
		for _, v := range raw[1:] {
			rec := map[string]interface{}{}
			for nkey, key := range keys {
				rec[key] = v[nkey]
			}
			records = append(records, rec)
		}
		c.Push().Data(&lua.List{Data: records}, "luaList")

		return nil
	}
}

func toCsvHandler() lua.Handler { // skipcq: GO-R1005
	return func(c *lua.Context) error {
		if c.Top() == 0 {
			return errNeedArguments
		}
		buffer := new(bytes.Buffer)
		w := enc.NewWriter(buffer)

		delimiter := defaultDelimiter
		if c.Top() == 2 {
			arg := c.Arg(2).String()
			if len(arg) > 0 {
				delimiter = arg
			}
		}
		w.Comma = rune(delimiter[0])

		switch t := c.Arg(1).Any().(type) {
		case *lua.NativeTable:
			var records [][]string

			t.ForEach(func(_, v lua.NativeValue) {
				var rec []string

				if e, ok := v.(*lua.NativeTable); ok {
					e.ForEach(func(_, p lua.NativeValue) {
						rec = append(rec, p.String())
					})
				}

				records = append(records, rec)
			})
			w.WriteAll(records)
		case *lua.NativeUserData:
			switch r := t.Value.(type) {
			case *lua.List:
				var records [][]string

				if len(r.Data) == 0 {
					break
				}
				switch first := r.Data[0].(type) {
				case []interface{}:
					for _, row := range r.Data {
						row := row.([]interface{})
						s := make([]string, len(row))
						for i, val := range row {
							s[i] = fmt.Sprint(val)
						}
						records = append(records, s)
					}
				case map[string]interface{}:
					var keys []string
					for k := range first {
						keys = append(keys, k)
					}
					sort.Strings(keys)
					records = append(records, keys)

					for _, row := range r.Data {
						row := row.(map[string]interface{})
						s := make([]string, len(keys))
						for i, key := range keys {
							s[i] = fmt.Sprint(row[key])
						}
						records = append(records, s)
					}
				}

				w.WriteAll(records)
			default:
				return errors.New("unsupported type for csv conversion")
			}
		default:
			return errors.New("unsupported type for csv conversion")
		}

		c.Push().String(buffer.String())
		return nil
	}
}
