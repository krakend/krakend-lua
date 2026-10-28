package mux

import (
	lua "github.com/krakend/krakend-lua/v3"
)

func parseNativeUserData(t *lua.NativeUserData, cp func(t *lua.NativeUserData) interface{}) interface{} {
	if t.Value != nil {
		switch v := t.Value.(type) {
		case *muxContext:
			return parseRequest(v)
		}
	}
	// luaNil
	return cp(t)
}

func parseRequest(v *muxContext) map[string]interface{} {
	params := map[string]interface{}{}
	for k, v := range v.pe(v.Request) {
		params[k] = v
	}
	var url, query, path string
	if v.URL != nil {
		url = v.URL.String()
		path = v.URL.Path
		query = v.URL.RawQuery
	}
	return map[string]interface{}{
		"url":     url,
		"method":  v.Method,
		"path":    path,
		"query":   query,
		"params":  params,
		"headers": parseHeaders(v.Header),
	}
}

func parseHeaders(headers map[string][]string) map[string]interface{} {
	h := make(map[string]interface{})
	for k, v := range headers {
		h[k] = v
	}
	return h
}
