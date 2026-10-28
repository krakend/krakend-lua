package decorator

import (
	"fmt"
	"sort"
	"strings"

	lua "github.com/krakend/krakend-lua/v3"
	"github.com/luraproject/lura/v3/logging"
)

type CustomParser func(t *lua.NativeUserData, udParser func(t *lua.NativeUserData) interface{}) interface{}

func defaultCustomParser(t *lua.NativeUserData, udParser func(t *lua.NativeUserData) interface{}) interface{} {
	return udParser(t)
}

func RegisterDebug(l logging.Logger, cp CustomParser, b *lua.Binder) {
	if cp == nil {
		cp = defaultCustomParser
	}
	t := b.Table("debug")
	t.Static("dump", dumpHandler(l, cp))
}

func dumpHandler(l logging.Logger, cp CustomParser) lua.Handler {
	return func(c *lua.Context) error {
		if c.Top() == 0 {
			return errNeedArguments
		}

		var dumpBuffer string

		switch t := c.Arg(1).Any().(type) {
		case lua.NativeBool:
			dumpBuffer = dump(c.Arg(1).Bool())
		case lua.NativeNumber:
			n := c.Arg(1).Number()
			trunc := float64(int(c.Arg(1).Number()))
			if n == trunc {
				dumpBuffer = dump(int(n))
			} else {
				dumpBuffer = dump(n)
			}
		case lua.NativeString:
			dumpBuffer = dump(c.Arg(1).String())
		case *lua.NativeTable:
			m, _ := lua.MapNativeTable(t)
			dumpBuffer = dump(m)
		case *lua.NativeUserData:
			dumpBuffer = dump(cp(t, parseNativeUserData))
		default:
			dumpBuffer = dump(t)
		}

		label := "[Lua]"
		if c.Top() == 2 {
			label += fmt.Sprintf(" Dumping %q", c.Arg(2).String())
		}

		l.Debug(label + "\n" + dumpBuffer)
		return nil
	}
}

func parseNativeUserData(t *lua.NativeUserData) interface{} {
	if t.Value != nil {
		switch v := t.Value.(type) {
		case *lua.Table:
			return v.Data
		case *lua.List:
			return v.Data
		case *lua.HttpResponse:
			return parseHttpResponse(v)
		default:
			return v
		}
	}
	// luaNil
	return nil
}

func parseHttpResponse(v *lua.HttpResponse) map[string]interface{} {
	request := map[string]interface{}{
		"method":  v.R.Request.Method,
		"headers": parseHeaders(v.R.Request.Header),
	}
	response := map[string]interface{}{
		"statusCode": float64(v.R.StatusCode),
		"body":       v.Body(),
		"headers":    parseHeaders(v.R.Header),
	}

	return map[string]interface{}{
		"request":  request,
		"response": response,
	}
}

func parseHeaders(headers map[string][]string) map[string]interface{} {
	h := make(map[string]interface{})
	for k, v := range headers {
		h[k] = v
	}
	return h
}

func dump(v interface{}) string {
	return write(v, 1)
}

func write(v interface{}, level int) string {
	var b strings.Builder
	indent := strings.Repeat(" ", level)
	switch value := v.(type) {
	case []interface{}:
		fmt.Fprint(&b, "(array)\n")
		for _, item := range value {
			fmt.Fprintf(&b, "%s%s- ", indent, indent)
			b.WriteString(write(item, level+1))
		}
	case map[string]interface{}:
		fmt.Fprint(&b, "(map)\n")
		var keys []string
		for k := range value {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, key := range keys {
			fmt.Fprintf(&b, "%s%s%s: ", indent, indent, key)
			b.WriteString(write(value[key], level+1))
		}
	default:
		if value == nil {
			fmt.Fprint(&b, "nil\n")
		} else {
			fmt.Fprintf(&b, "%v (%T)\n", value, value)
		}
	}
	return b.String()
}
