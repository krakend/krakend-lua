package proxy

import (
	lua "github.com/krakend/krakend-lua/v3"
)

func parseNativeUserData(t *lua.NativeUserData, cp func(t *lua.NativeUserData) interface{}) interface{} {
	if t.Value != nil {
		switch v := t.Value.(type) {
		case *ProxyRequest:
			return parseProxyRequest(v)
		case *ProxyResponse:
			return parseProxyResponse(v)
		}
	}
	// luaNil
	return cp(t)
}

func parseProxyRequest(v *ProxyRequest) map[string]interface{} {
	params := map[string]interface{}{}
	for k, v := range v.Params {
		params[k] = v
	}
	var url string
	if v.URL != nil {
		url = v.URL.String()
	}
	return map[string]interface{}{
		"url":     url,
		"method":  v.Method,
		"path":    v.Path,
		"query":   v.Query.Encode(),
		"params":  params,
		"headers": parseHeaders(v.Headers),
	}
}

func parseProxyResponse(v *ProxyResponse) map[string]interface{} {
	return map[string]interface{}{
		"isComplete": v.IsComplete,
		"statusCode": float64(v.Metadata.StatusCode),
		"data":       v.Data,
		"headers":    parseHeaders(v.Metadata.Headers),
	}
}

func parseHeaders(headers map[string][]string) map[string]interface{} {
	h := make(map[string]interface{})
	for k, v := range headers {
		h[k] = v
	}
	return h
}
