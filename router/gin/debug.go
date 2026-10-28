package gin

import (
	lua "github.com/krakend/krakend-lua/v3"
)

func parseNativeUserData(t *lua.NativeUserData, cp func(t *lua.NativeUserData) interface{}) interface{} {
	if t.Value != nil {
		switch v := t.Value.(type) {
		case *GinContext:
			return parseGinContext(v)
		}
	}
	// luaNil
	return cp(t)
}

func parseGinContext(v *GinContext) map[string]interface{} {
	params := map[string]interface{}{}
	for _, v := range v.Params {
		params[v.Key] = v.Value
	}
	var path string
	var query string
	var url string
	if v.Request.URL != nil {
		url = v.Request.URL.String()
		path = v.Request.URL.Path
		query = v.Request.URL.RawQuery
	}
	return map[string]interface{}{
		"url":     url,
		"method":  v.Request.Method,
		"path":    path,
		"query":   query,
		"params":  params,
		"headers": parseHeaders(v.Request.Header),
	}
}

func parseHeaders(headers map[string][]string) map[string]interface{} {
	h := make(map[string]interface{})
	for k, v := range headers {
		h[k] = v
	}
	return h
}
