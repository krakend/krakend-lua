package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"testing"

	lua "github.com/krakendio/krakend-lua/v2"
	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/encoding"
	"github.com/luraproject/lura/v2/logging"
	"github.com/luraproject/lura/v2/proxy"
	"github.com/luraproject/lura/v2/transport/http/client"
)

func TestProxyFactory_luaError(t *testing.T) {
	var luaErrorTestTable = []struct {
		Name          string
		Cfg           map[string]interface{}
		ExpectedError string
	}{
		{
			Name: "Pre: Syntax error",
			Cfg: map[string]interface{}{
				"pre": "local req = request.load()\nlokal a = 1()\nlocal b = 2",
			},
			ExpectedError: "'a': parse error (pre-script:L2)",
		},
		{
			Name: "Pre: Inline syntax error",
			Cfg: map[string]interface{}{
				"pre": "local req = request.load();lokal a = 1();local b = 2",
			},
			ExpectedError: "'a': parse error (pre-script:L1)",
		},
		{
			Name: "Pre: Inline semicolon separated",
			Cfg: map[string]interface{}{
				"pre": "local req = request.load();method_does_not_exist();local test = 1",
			},
			ExpectedError: "attempt to call a non-function object (pre-script:L1)",
		},
		{
			Name: "Pre: Inline",
			Cfg: map[string]interface{}{
				"pre": "local req = request.load()\nmethod_does_not_exist()\nlocal test = 1",
			},
			ExpectedError: "attempt to call a non-function object (pre-script:L2)",
		},
		{
			Name: "Pre: Multiline",
			Cfg: map[string]interface{}{
				"pre": `local req = request.load()
						req:method("POST")
						req:params("foo", "some_new_value")
						req:headers("Accept", "application/xml")
						req:url(req:url() .. "&more=true")
						reqw:body(req:body() .. " foo" .. req:headers("unknown")) -- fat-fingered`,
			},
			ExpectedError: "attempt to index a non-table object(nil) with key 'body' (pre-script:L6)",
		},
		{
			Name: "Pre: Empty custom_error",
			Cfg: map[string]interface{}{
				"pre": "custom_error()",
			},
			ExpectedError: "need arguments (pre-script:L1)",
		},
		{
			Name: "Pre: Single source with bad code",
			Cfg: map[string]interface{}{
				"sources": []interface{}{
					"../lua/bad-code.lua",
				},
				"pre": "custom_error(\"wont reach here\")",
			},
			ExpectedError: "attempt to index a non-table object(function) with key 'really_bad' (bad-code.lua:L5)",
		},
		{
			Name: "Pre: Single source with bad method implementation",
			Cfg: map[string]interface{}{
				"sources": []interface{}{
					"../lua/bad-func.lua",
				},
				"pre": "badfunc(1)",
			},
			ExpectedError: "attempt to call a non-function object (bad-func.lua:L3)",
		},
		{
			Name: "Pre: Multiple sources",
			Cfg: map[string]interface{}{
				"sources": []interface{}{
					"../lua/factorial.lua",
					"../lua/bad-code.lua",
					"../lua/add.lua",
				},
				"pre": "custom_error(\"wont reach here\")",
			},
			ExpectedError: "attempt to index a non-table object(function) with key 'really_bad' (bad-code.lua:L5)",
		},
		{
			Name: "Pre: Multiple sources, bad function call",
			Cfg: map[string]interface{}{
				"sources": []interface{}{
					"../lua/env.lua",
					"../lua/factorial.lua",
					"../lua/bad-func.lua",
					"../lua/add.lua",
				},
				"pre": "badfunc(1)",
			},
			ExpectedError: "attempt to call a non-function object (bad-func.lua:L3)",
		},
		{
			Name: "Post: Inline syntax error",
			Cfg: map[string]interface{}{
				"post": "local req = request.load();lokal a = 1();local b = 2",
			},
			ExpectedError: "'a': parse error (post-script:L1)",
		},
		{
			Name: "Post: Inline semicolon separated",
			Cfg: map[string]interface{}{
				"post": "local req = request.load();method_does_not_exist();local test = 1",
			},
			ExpectedError: "attempt to call a non-function object (post-script:L1)",
		},
		{
			Name: "Post: Inline",
			Cfg: map[string]interface{}{
				"post": "local req = request.load()\nmethod_does_not_exist()\nlocal test = 1",
			},
			ExpectedError: "attempt to call a non-function object (post-script:L2)",
		},
		{
			Name: "Post: Multiline",
			Cfg: map[string]interface{}{
				"post": `local resp = response.load()
						local responseData = resp:data()
						local data = {}
						local col = responseDataBad:get("items")`,
			},
			ExpectedError: "attempt to index a non-table object(nil) with key 'get' (post-script:L4)",
		},
		{
			Name: "Post: Empty custom_error",
			Cfg: map[string]interface{}{
				"post": "custom_error()",
			},
			ExpectedError: "need arguments (post-script:L1)",
		},
		{
			Name: "Post: Single source with bad code",
			Cfg: map[string]interface{}{
				"sources": []interface{}{
					"../lua/bad-code.lua",
				},
				"post": "custom_error(\"wont reach here\")",
			},
			ExpectedError: "attempt to index a non-table object(function) with key 'really_bad' (bad-code.lua:L5)",
		},
		{
			Name: "Post: Single source with bad method implementation",
			Cfg: map[string]interface{}{
				"sources": []interface{}{
					"../lua/bad-func.lua",
				},
				"post": "badfunc(1)",
			},
			ExpectedError: "attempt to call a non-function object (bad-func.lua:L3)",
		},
		{
			Name: "Post: Multiple sources",
			Cfg: map[string]interface{}{
				"sources": []interface{}{
					"../lua/factorial.lua",
					"../lua/bad-code.lua",
					"../lua/add.lua",
				},
				"post": "custom_error(\"wont reach here\")",
			},
			ExpectedError: "attempt to index a non-table object(function) with key 'really_bad' (bad-code.lua:L5)",
		},
		{
			Name: "Post: Multiple sources, bad function call",
			Cfg: map[string]interface{}{
				"sources": []interface{}{
					"../lua/env.lua",
					"../lua/factorial.lua",
					"../lua/bad-func.lua",
					"../lua/add.lua",
				},
				"post": "badfunc(1)",
			},
			ExpectedError: "attempt to call a non-function object (bad-func.lua:L3)",
		},
	}

	for _, test := range luaErrorTestTable {
		t.Run(test.Name, func(t *testing.T) {
			logger, err := logging.NewLogger("ERROR", bytes.NewBuffer(make([]byte, 1024)), "pref")
			if err != nil {
				t.Error("building the logger:", err.Error())
				return
			}

			explosive := proxy.FactoryFunc(func(_ *config.EndpointConfig) (proxy.Proxy, error) {
				return func(_ context.Context, _ *proxy.Request) (*proxy.Response, error) {
					return &proxy.Response{}, nil
				}, nil
			})

			prxy, err := ProxyFactory(logger, explosive).New(&config.EndpointConfig{
				Endpoint: "/",
				ExtraConfig: config.ExtraConfig{
					ProxyNamespace: test.Cfg,
				},
			})

			if err != nil {
				t.Error(err)
			}

			URL, _ := url.Parse("https://some.host.tld/path/to/resource?and=querystring")

			resp, err := prxy(context.Background(), &proxy.Request{
				Method:  "GET",
				Path:    "/some-path",
				Params:  map[string]string{"Id": "42"},
				Headers: map[string][]string{},
				URL:     URL,
				Body:    io.NopCloser(strings.NewReader("initial req content")),
			})

			if resp != nil {
				t.Errorf("unexpected response: %v", resp)
				return
			}

			if e := err.Error(); e != test.ExpectedError {
				t.Errorf("unexpected error, have: '%s', want: '%s' (%T)", e, test.ExpectedError, err)
				return
			}
		})
	}
}

func TestProxyFactory_error(t *testing.T) {
	testProxyFactoryError(t, `custom_error('expect me')`, "expect me", "", false, 0)
	testProxyFactoryPostError(t, `custom_error('expect me')`, "expect me", "", false, 0)
}

func TestProxyFactory_errorHTTP(t *testing.T) {
	testProxyFactoryError(t, `custom_error('expect me', 404)`, "expect me", "", true, 404)
	testProxyFactoryPostError(t, `custom_error('expect me', 404)`, "expect me", "", true, 404)
}

func TestProxyFactory_errorHTTPJson(t *testing.T) {
	testProxyFactoryError(t, `custom_error('{"msg":"expect me"}', 404)`, `{"msg":"expect me"}`, "", true, 404)
	testProxyFactoryPostError(t, `custom_error('{"msg":"expect me"}', 404)`, `{"msg":"expect me"}`, "", true, 404)
}

func TestProxyFactory_errorHTTPWithContentType(t *testing.T) {
	testProxyFactoryError(t, `custom_error('{"msg":"expect me"}', 404, 'application/json')`, `{"msg":"expect me"}`, "application/json", true, 404)
	testProxyFactoryPostError(t, `custom_error('{"msg":"expect me"}', 404, 'application/json')`, `{"msg":"expect me"}`, "application/json", true, 404)
}

func testProxyFactoryError(t *testing.T, code, errMsg, contentType string, isHTTP bool, statusCode int) {
	buff := bytes.NewBuffer(make([]byte, 1024))
	logger, err := logging.NewLogger("ERROR", buff, "pref")
	if err != nil {
		t.Error("building the logger:", err.Error())
		return
	}

	unexpectedErr := errors.New("never seen")

	explosive := proxy.FactoryFunc(func(_ *config.EndpointConfig) (proxy.Proxy, error) {
		return func(_ context.Context, _ *proxy.Request) (*proxy.Response, error) {
			return nil, unexpectedErr
		}, nil
	})

	prxy, err := ProxyFactory(logger, explosive).New(&config.EndpointConfig{
		Endpoint: "/",
		ExtraConfig: config.ExtraConfig{
			ProxyNamespace: map[string]interface{}{
				"pre": code,
			},
		},
	})

	if err != nil {
		t.Error(err)
	}

	URL, _ := url.Parse("https://some.host.tld/path/to/resource?and=querystring")

	resp, err := prxy(context.Background(), &proxy.Request{
		Method:  "GET",
		Path:    "/some-path",
		Params:  map[string]string{"Id": "42"},
		Headers: map[string][]string{},
		URL:     URL,
		Body:    io.NopCloser(strings.NewReader("initial req content")),
	})

	if resp != nil {
		t.Errorf("unexpected response: %v", resp)
		return
	}

	if err == unexpectedErr {
		t.Errorf("the script did not stop the pipe execution: %v", err)
		return
	}

	switch err := err.(type) {
	case lua.ErrInternalHTTP:
		if !isHTTP {
			t.Errorf("unexpected http error: %v (%T)", err, err)
			return
		}
		if sc := err.StatusCode(); sc != statusCode {
			t.Errorf("unexpected http status code: %d", sc)
			return
		}
	case lua.ErrInternal:
		if isHTTP {
			t.Errorf("unexpected internal error: %v (%T)", err, err)
			return
		}
	case lua.ErrInternalHTTPWithContentType:
		if !isHTTP {
			t.Errorf("unexpected http error: %v (%T)", err, err)
			return
		}
		if sc := err.StatusCode(); sc != statusCode {
			t.Errorf("unexpected http status code: %d", sc)
			return
		}
		if ct := err.Encoding(); ct != contentType {
			t.Errorf("unexpected content type: %s", ct)
			return
		}
	default:
		t.Errorf("unexpected error: %v (%T)", err, err)
		return
	}

	if e := err.Error(); e != errMsg {
		t.Errorf("unexpected error. have: '%s', want: '%s' (%T)", e, errMsg, err)
		return
	}
}

func testProxyFactoryPostError(t *testing.T, code, errMsg, contentType string, isHTTP bool, statusCode int) {
	buff := bytes.NewBuffer(make([]byte, 1024))
	logger, err := logging.NewLogger("ERROR", buff, "pref")
	if err != nil {
		t.Error("building the logger:", err.Error())
		return
	}

	explosive := proxy.FactoryFunc(func(_ *config.EndpointConfig) (proxy.Proxy, error) {
		return func(_ context.Context, _ *proxy.Request) (*proxy.Response, error) {
			return &proxy.Response{}, nil
		}, nil
	})

	prxy, err := ProxyFactory(logger, explosive).New(&config.EndpointConfig{
		Endpoint: "/",
		ExtraConfig: config.ExtraConfig{
			ProxyNamespace: map[string]interface{}{
				"post": code,
			},
		},
	})

	if err != nil {
		t.Error(err)
	}

	URL, _ := url.Parse("https://some.host.tld/path/to/resource?and=querystring")

	resp, err := prxy(context.Background(), &proxy.Request{
		Method:  "GET",
		Path:    "/some-path",
		Params:  map[string]string{"Id": "42"},
		Headers: map[string][]string{},
		URL:     URL,
		Body:    io.NopCloser(strings.NewReader("initial req content")),
	})

	if resp != nil {
		t.Errorf("unexpected response: %v", resp)
		return
	}

	if err == nil {
		t.Error("the script did not return an error")
		return
	}

	switch err := err.(type) {
	case lua.ErrInternalHTTP:
		if !isHTTP {
			t.Errorf("unexpected http error: %v (%T)", err, err)
			return
		}
		if sc := err.StatusCode(); sc != statusCode {
			t.Errorf("unexpected http status code: %d", sc)
			return
		}
	case lua.ErrInternal:
		if isHTTP {
			t.Errorf("unexpected internal error: %v (%T)", err, err)
			return
		}
	case lua.ErrInternalHTTPWithContentType:
		if !isHTTP {
			t.Errorf("unexpected http error: %v (%T)", err, err)
			return
		}
		if sc := err.StatusCode(); sc != statusCode {
			t.Errorf("unexpected http status code: %d", sc)
			return
		}
		if ct := err.Encoding(); ct != contentType {
			t.Errorf("unexpected content type: %s", ct)
			return
		}
	default:
		t.Errorf("unexpected error: %v (%T)", err, err)
		return
	}

	if e := err.Error(); e != errMsg {
		t.Errorf("unexpected error. have: '%s', want: '%s' (%T)", e, errMsg, err)
		return
	}
}

func TestProxyFactory(t *testing.T) {
	buff := bytes.NewBuffer(make([]byte, 1024))
	logger, err := logging.NewLogger("ERROR", buff, "pref")
	if err != nil {
		t.Error("building the logger:", err.Error())
		return
	}

	expectedResponse := &proxy.Response{
		Data: map[string]interface{}{
			"ok": true,
			"collection": []interface{}{
				map[string]interface{}{
					"id":      1,
					"comment": "none",
				},
				map[string]interface{}{
					"id":      42,
					"comment": "some",
				},
				map[string]interface{}{
					"id":      99,
					"comment": "to be removed",
				},
				map[string]interface{}{
					"id":      0,
					"comment": "last",
				},
			},
			"to_be_removed": 123456,
		},
		Metadata: proxy.Metadata{
			Headers: map[string][]string{
				"X-Not-Needed": {"deleteme"},
			},
		},
		Io: strings.NewReader("initial resp content"),
	}

	dummyProxyFactory := proxy.FactoryFunc(func(_ *config.EndpointConfig) (proxy.Proxy, error) {
		return func(_ context.Context, req *proxy.Request) (*proxy.Response, error) {
			if req.Method != "POST" {
				t.Errorf("unexpected method %s", req.Method)
			}
			if req.Params["foo"] != "some_new_value" {
				t.Errorf("unexpected param foo %s", req.Params["foo"])
			}
			if req.Headers["Accept"][0] != "application/xml" {
				t.Errorf("unexpected header 'Accept' %v", req.Headers["Accept"])
			}
			if _, found := req.Headers["X-To-Delete"]; found {
				t.Error("unexpected header 'X-To-Delete', should have been deleted")
			}
			if req.URL.String() != "https://some.host.tld/path/to/resource?and=querystring&more=true" {
				t.Errorf("unexpected URL: %s", req.URL.String())
			}
			b, err := io.ReadAll(req.Body)
			if err != nil {
				t.Error(err)
			}
			if string(b) != "initial req content foo" {
				t.Errorf("unexpected body: %s", string(b))
			}
			return expectedResponse, nil
		}, nil
	})

	URL, _ := url.Parse("https://some.host.tld/path/to/resource?and=querystring")

	prxy, err := ProxyFactory(logger, dummyProxyFactory).New(&config.EndpointConfig{
		Endpoint: "/",
		ExtraConfig: config.ExtraConfig{
			ProxyNamespace: map[string]interface{}{
				"sources": []interface{}{
					"../lua/factorial.lua",
				},
				"allow_open_libs": true,

				"pre": `local req = request.load()
		req:method("POST")
		req:params("foo", "some_new_value")
		req:headers("Accept", "application/xml")
		req:headers("X-To-Delete", nil)
		req:url(req:url() .. "&more=true")
		req:body(req:body() .. " foo" .. req:headers("unknown"))`,

				"post": `local resp = response.load()
		resp:isComplete(true)
		local responseData = resp:data()
		responseData:set("foo", "some_new_value")

		data = {}
		data["bar"] = fact(5)
		data["foobar"] = true
		data["supu"] = {}
		data["supu"]["tupu"] = "some"
		data["supu"]["original"] = responseData:get("ok")

		local original_collection = responseData:get("collection")
		original_collection:del(2)
		responseData:set("collection", original_collection)
		local col = responseData:get("collection")
		data["collection_size"] = col:len()

		local id_list = {}
		for i=0,data["collection_size"]-1 do
			local element = col:get(i)
			local id = element:get("id")
			table.insert(id_list, id)
		end
		data["ids"] = id_list

		responseData:set("more", data)
		local bar = string.find('banana', 'an')
		responseData:set("bar", bar)
		responseData:set("keys", responseData:keys())
		responseData:del("to_be_removed")

		if responseData:keyExists("to_be_removed")
		then
			custom_error("unexpected key")
		end

		if not responseData:keyExists("bar")
		then
			custom_error("missing required key")
		end

		resp:headers("Content-Type", "application/xml")
		resp:headers("X-Not-Needed", nil)
		resp:statusCode(200)
		resp:body(resp:body() .. " bar" .. resp:headers("unknown"))`,
			},
		},
	})

	if err != nil {
		t.Error(err)
	}

	resp, err := prxy(context.Background(), &proxy.Request{
		Method: "GET",
		Path:   "/some-path",
		Params: map[string]string{"Id": "42"},
		Headers: map[string][]string{
			"X-To-Delete": {"deleteme"},
		},
		URL:  URL,
		Body: io.NopCloser(strings.NewReader("initial req content")),
	})
	if err != nil {
		t.Errorf("unexpected error %s", err.Error())
		return
	}
	if resp.Metadata.StatusCode != 200 {
		t.Errorf("unexpected status code %d", resp.Metadata.StatusCode)
		return
	}
	if _, found := resp.Metadata.Headers["X-Not-Needed"]; found {
		t.Error("unexpected response header 'X-Not-Needed', should have been deleted")
		return
	}
	if !resp.IsComplete {
		t.Error("incomplete response")
		return
	}
	if resp.Metadata.Headers["Content-Type"][0] != "application/xml" {
		t.Errorf("unexpected Content-Type %v", resp.Metadata.Headers["Content-Type"])
		return
	}
	if v, ok := resp.Data["foo"].(string); !ok || v != "some_new_value" {
		t.Errorf("unexpected response data %v, %T", resp.Data["foo"], resp.Data["foo"])
		return
	}
	v, ok := resp.Data["more"].(map[string]interface{})
	if !ok {
		t.Errorf("unexpected field 'more': %T, %+v", resp.Data["more"], v)
		return
	}
	if bar, ok := v["bar"].(int); !ok || bar != 120 {
		t.Errorf("unexpected field 'more.bar': %v", v["bar"])
	}

	b, _ := json.MarshalIndent(resp.Data, "", "\t")

	expectedResponseString := `{
	"bar": 2,
	"collection": [
		{
			"comment": "none",
			"id": 1
		},
		{
			"comment": "some",
			"id": 42
		},
		{
			"comment": "last",
			"id": 0
		}
	],
	"foo": "some_new_value",
	"keys": [
		"bar",
		"collection",
		"foo",
		"more",
		"ok",
		"to_be_removed"
	],
	"more": {
		"bar": 120,
		"collection_size": 3,
		"foobar": true,
		"ids": [
			1,
			42,
			0
		],
		"supu": {
			"original": true,
			"tupu": "some"
		}
	},
	"ok": true
}`
	if expectedResponseString != string(b) {
		t.Errorf("unexpected response %s", string(b))
	}

	b, err = io.ReadAll(resp.Io)
	if err != nil {
		t.Error(err)
	}
	if string(b) != "initial resp content bar" {
		t.Errorf("unexpected body: %s", string(b))
	}
}

func Test_Issue7(t *testing.T) {
	buff := bytes.NewBuffer(make([]byte, 1024))
	logger, err := logging.NewLogger("ERROR", buff, "pref")
	if err != nil {
		t.Error("building the logger:", err.Error())
		return
	}

	response := `{
  "items": [
    {"id": 1, "name": "foo", "funny_property": true, "long_name": "foo bar"},
    {"id": 2, "name": "foo2", "funny_property": false, "long_name": "foo2 bar2"}
  ]
}`
	r := map[string]interface{}{}
	json.Unmarshal([]byte(response), &r)

	dummyProxyFactory := proxy.FactoryFunc(func(_ *config.EndpointConfig) (proxy.Proxy, error) {
		return func(_ context.Context, _ *proxy.Request) (*proxy.Response, error) {
			return &proxy.Response{
				Data: r,
				Metadata: proxy.Metadata{
					Headers: map[string][]string{},
				},
			}, nil
		}, nil
	})

	prxy, err := ProxyFactory(logger, dummyProxyFactory).New(&config.EndpointConfig{
		Endpoint: "/",
		ExtraConfig: config.ExtraConfig{
			ProxyNamespace: map[string]interface{}{
				"post": `
	local resp = response.load()
	local responseData = resp:data()
	local data = {}
	local col = responseData:get("items")

	local size = col:len()
	responseData:set("total", size)

	local names = {}
	for i=0,size-1 do
		local element = col:get(i)
		local t = element:get("long_name")
		table.insert(names, t)
	end

	responseData:set("names", names)
	responseData:del("items")
`,
			},
		},
	})

	if err != nil {
		t.Error(err)
	}

	URL, _ := url.Parse("https://some.host.tld/path/to/resource?and=querystring")

	_, err = prxy(context.Background(), &proxy.Request{
		Method:  "GET",
		Path:    "/some-path",
		Params:  map[string]string{"Id": "42"},
		Headers: map[string][]string{},
		URL:     URL,
		Body:    io.NopCloser(strings.NewReader("initial req content")),
	})

	if err != nil {
		t.Error(err)
	}

	fmt.Println(buff.String())
}

func Test_jsonNumber(t *testing.T) {
	buff := bytes.NewBuffer(make([]byte, 1024))
	logger, err := logging.NewLogger("ERROR", buff, "pref")
	if err != nil {
		t.Error("building the logger:", err.Error())
		return
	}

	response := `{"id": 1, "name": "foo", "funny_property": true, "long_name": "foo bar"}`
	r := map[string]interface{}{}
	encoding.JSONDecoder(strings.NewReader(response), &r)

	dummyProxyFactory := proxy.FactoryFunc(func(_ *config.EndpointConfig) (proxy.Proxy, error) {
		return func(_ context.Context, _ *proxy.Request) (*proxy.Response, error) {
			return &proxy.Response{
				Data: r,
				Metadata: proxy.Metadata{
					Headers: map[string][]string{},
				},
			}, nil
		}, nil
	})

	prxy, err := ProxyFactory(logger, dummyProxyFactory).New(&config.EndpointConfig{
		Endpoint: "/",
		ExtraConfig: config.ExtraConfig{
			ProxyNamespace: map[string]interface{}{
				"post": `
local resp = response.load()
local responseData = resp:data()
print(responseData:get("id"))
responseData:set("id", responseData:get("id")+1)
`,
			},
		},
	})

	if err != nil {
		t.Error(err)
	}

	URL, _ := url.Parse("https://some.host.tld/path/to/resource?and=querystring")

	resp, err := prxy(context.Background(), &proxy.Request{
		Method:  "GET",
		Path:    "/some-path",
		Params:  map[string]string{"Id": "42"},
		Headers: map[string][]string{},
		URL:     URL,
		Body:    io.NopCloser(strings.NewReader("initial req content")),
	})

	if err != nil {
		t.Error(err)
	}

	if id, ok := resp.Data["id"].(float64); !ok || id != 2 {
		t.Errorf("unexpected id %f", id)
	}
}

func Test_keyValConverter(t *testing.T) {
	buff := bytes.NewBuffer(make([]byte, 1024))
	logger, err := logging.NewLogger("ERROR", buff, "pref")
	if err != nil {
		t.Error("building the logger:", err.Error())
		return
	}

	response := `
    { "data": [
        {"key": "name",
         "value": "Impala"
        },
        {"key": "IBU",
         "value": null
        },
        {"key": "type",
         "value": "IPA"
        }
    ]}
`
	r := map[string]interface{}{}
	if err := encoding.JSONDecoder(strings.NewReader(response), &r); err != nil {
		t.Errorf("cannot deserialize response string: %s", err)
		return
	}

	dummyProxyFactory := proxy.FactoryFunc(func(_ *config.EndpointConfig) (proxy.Proxy, error) {
		return func(_ context.Context, _ *proxy.Request) (*proxy.Response, error) {
			return &proxy.Response{
				Data: r,
				Metadata: proxy.Metadata{
					Headers: map[string][]string{},
				},
			}, nil
		}, nil
	})

	prxy, err := ProxyFactory(logger, dummyProxyFactory).New(&config.EndpointConfig{
		Endpoint: "/",
		ExtraConfig: config.ExtraConfig{
			ProxyNamespace: map[string]interface{}{
				"post": `
local resp = response.load()
local responseData = resp:data()
local formated = luaTable.new()
local items = responseData:get("data")

local size = items:len()

if size > 0 then
    for i=0,size-1 do
        local element = items:get(i)
        local key = element:get("key")
        local value = element:get("value")
        responseData:set(key, value)
    end
end

responseData:del("data")
`,
			},
		},
	})

	if err != nil {
		t.Error(err)
	}

	URL, _ := url.Parse("https://some.host.tld/path/to/resource?and=querystring")

	resp, err := prxy(context.Background(), &proxy.Request{
		Method:  "GET",
		Path:    "/some-path",
		Params:  map[string]string{"Id": "42"},
		Headers: map[string][]string{},
		URL:     URL,
		Body:    io.NopCloser(strings.NewReader("initial req content")),
	})

	if err != nil {
		t.Error(err)
	}

	if strType, ok := resp.Data["type"].(string); !ok || strType != "IPA" {
		t.Errorf("unexpected type %#v", resp.Data["type"])
	}

	v, ok := resp.Data["IBU"]
	if !ok {
		t.Errorf("the IBU key must exist and be nil: %#v", resp.Data)
		return
	}
	if v != nil {
		t.Errorf("IBU value should be nil")
	}
}

func Test_listGrowsWhenUpperIndexOutOfBound(t *testing.T) {
	buff := bytes.NewBuffer(make([]byte, 1024))
	logger, err := logging.NewLogger("ERROR", buff, "pref")
	if err != nil {
		t.Error("building the logger:", err.Error())
		return
	}
	r := map[string]interface{}{}

	dummyProxyFactory := proxy.FactoryFunc(func(_ *config.EndpointConfig) (proxy.Proxy, error) {
		return func(_ context.Context, _ *proxy.Request) (*proxy.Response, error) {
			return &proxy.Response{
				Data: r,
				Metadata: proxy.Metadata{
					Headers: map[string][]string{},
				},
			}, nil
		}, nil
	})

	prxy, err := ProxyFactory(logger, dummyProxyFactory).New(&config.EndpointConfig{
		Endpoint: "/",
		ExtraConfig: config.ExtraConfig{
			ProxyNamespace: map[string]interface{}{
				"post": `
local resp = response.load()
local responseData = resp:data()
local growingList = luaList.new()

growingList:set(0, "foo")
growingList:set(2, "bar")

responseData:set("grow_list", growingList)
`,
			},
		},
	})

	if err != nil {
		t.Error(err)
	}

	URL, _ := url.Parse("https://some.host.tld/path/to/resource?and=querystring")

	resp, err := prxy(context.Background(), &proxy.Request{
		Method:  "GET",
		Path:    "/some-path",
		Params:  map[string]string{"Id": "42"},
		Headers: map[string][]string{},
		URL:     URL,
		Body:    io.NopCloser(strings.NewReader("initial req content")),
	})
	if err != nil {
		t.Error(err)
	}

	lst, ok := resp.Data["grow_list"].([]interface{})
	if !ok {
		t.Errorf("cannot get list 'grow_list' %#v", resp.Data)
		return
	}

	if len(lst) != 3 {
		t.Errorf("expected len 4 != %d -> %#v", len(lst), resp.Data)
		return
	}

	if lst[0].(string) != "foo" {
		t.Errorf("expected 'foo' in position 0")
		return
	}
	if lst[1] != nil {
		t.Errorf("expected nil in position 1")
		return
	}
	if lst[2].(string) != "bar" {
		t.Errorf("expected 'bar' in position 2")
		return
	}
}

func Test_tableGetSupportsClientErrors(t *testing.T) {
	errA := client.HTTPResponseError{
		Code: 418,
		Msg:  "I'm a teapot",
		Enc:  "text/plain",
	}
	errB := client.NamedHTTPResponseError{
		HTTPResponseError: client.HTTPResponseError{
			Code: 481,
			Msg:  "I'm not a teapot",
			Enc:  "text/plain",
		},
	}

	dummyProxyFactory := proxy.FactoryFunc(func(_ *config.EndpointConfig) (proxy.Proxy, error) {
		return func(_ context.Context, _ *proxy.Request) (*proxy.Response, error) {
			return &proxy.Response{
				Data: map[string]interface{}{
					"error_backend_alias_a": errA,
					"error_backend_alias_b": errB,
				},
				Metadata: proxy.Metadata{
					Headers: map[string][]string{},
				},
			}, nil
		}, nil
	})

	prxy, err := ProxyFactory(logging.NoOp, dummyProxyFactory).New(&config.EndpointConfig{
		Endpoint: "/",
		ExtraConfig: config.ExtraConfig{
			ProxyNamespace: map[string]interface{}{
				"post": `
local resp = response.load()
local responseData = resp:data()
local errorA = responseData:get('error_backend_alias_a')
responseData:set("code_a", errorA:get('http_status_code'))
responseData:set("body_a", errorA:get('http_body'))
responseData:set("encoding_a", errorA:get('http_body_encoding'))
local errorB = responseData:get('error_backend_alias_b')
responseData:set("code_b", errorB:get('http_status_code'))
responseData:set("body_b", errorB:get('http_body'))
responseData:set("encoding_b", errorB:get('http_body_encoding'))
responseData:set("name_b", errorB:get('name'))
`,
			},
		},
	})

	if err != nil {
		t.Error(err)
	}

	URL, _ := url.Parse("https://some.host.tld/path/to/resource?and=querystring")

	resp, err := prxy(context.Background(), &proxy.Request{
		Method:  "GET",
		Path:    "/some-path",
		Params:  map[string]string{"Id": "42"},
		Headers: map[string][]string{},
		URL:     URL,
		Body:    io.NopCloser(strings.NewReader("initial req content")),
	})
	if err != nil {
		t.Error(err)
	}

	bodyA, ok := resp.Data["body_a"].(string)
	if !ok {
		t.Errorf("cannot get 'body_a' %#v", resp.Data)
		return
	}
	if bodyA != errA.Msg {
		t.Errorf("unexpected body a. have %s, want %s", bodyA, errA.Msg)
	}

	bodyB, ok := resp.Data["body_b"].(string)
	if !ok {
		t.Errorf("cannot get 'body_b' %#v", resp.Data)
		return
	}
	if bodyB != errB.Msg {
		t.Errorf("unexpected body b. have %s, want %s", bodyB, errB.Msg)
	}

	codeA, ok := resp.Data["code_a"].(float64)
	if !ok {
		t.Errorf("cannot get 'code_a' %#v", resp.Data)
		return
	}
	if int(codeA) != errA.Code {
		t.Errorf("unexpected code a. have %d, want %d", int(codeA), errA.Code)
	}

	codeB, ok := resp.Data["code_b"].(float64)
	if !ok {
		t.Errorf("cannot get 'code_b' %#v", resp.Data)
		return
	}
	if int(codeB) != errB.Code {
		t.Errorf("unexpected code b. have %d, want %d", int(codeB), errB.Code)
	}

	encA, ok := resp.Data["encoding_a"].(string)
	if !ok {
		t.Errorf("cannot get 'encoding_a' %#v", resp.Data)
		return
	}
	if encA != errA.Enc {
		t.Errorf("unexpected encoding a. have %s, want %s", encA, errA.Enc)
	}

	encB, ok := resp.Data["encoding_b"].(string)
	if !ok {
		t.Errorf("cannot get 'encoding_b' %#v", resp.Data)
		return
	}
	if encB != errB.Enc {
		t.Errorf("unexpected encoding b. have %s, want %s", encB, errB.Enc)
	}
}
