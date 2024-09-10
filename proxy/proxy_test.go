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
)

func TestProxyFactory_error(t *testing.T) {
	testProxyFactoryError(t, `custom_error('expect me')`, "expect me", false, 0)
	testProxyFactoryPostError(t, `custom_error('expect me')`, "expect me", false, 0)
}

func TestProxyFactory_errorHTTP(t *testing.T) {
	testProxyFactoryError(t, `custom_error('expect me', 404)`, "expect me", true, 404)
	testProxyFactoryPostError(t, `custom_error('expect me', 404)`, "expect me", true, 404)
}

func TestProxyFactory_errorHTTPJson(t *testing.T) {
	testProxyFactoryError(t, `custom_error('{"msg":"expect me"}', 404)`, `{"msg":"expect me"}`, true, 404)
	testProxyFactoryPostError(t, `custom_error('{"msg":"expect me"}', 404)`, `{"msg":"expect me"}`, true, 404)
}

func testProxyFactoryError(t *testing.T, code, errMsg string, isHTTP bool, statusCode int) {
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
	default:
		t.Errorf("unexpected error: %v (%T)", err, err)
		return
	}

	if e := err.Error(); e != errMsg {
		t.Errorf("unexpected error. have: '%s', want: '%s' (%T)", e, errMsg, err)
		return
	}
}

func testProxyFactoryPostError(t *testing.T, code, errMsg string, isHTTP bool, statusCode int) {
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
		Metadata: proxy.Metadata{},
		Io:       strings.NewReader("initial resp content"),
	}

	dummyProxyFactory := proxy.FactoryFunc(func(_ *config.EndpointConfig) (proxy.Proxy, error) {
		return func(ctx context.Context, req *proxy.Request) (*proxy.Response, error) {
			if req.Method != "POST" {
				t.Errorf("unexpected method %s", req.Method)
			}
			if req.Params["foo"] != "some_new_value" {
				t.Errorf("unexpected param foo %s", req.Params["foo"])
			}
			if req.Headers["Accept"][0] != "application/xml" {
				t.Errorf("unexpected header 'Accept' %v", req.Headers["Accept"])
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
		resp:statusCode(200)
		resp:body(resp:body() .. " bar" .. resp:headers("unknown"))`,
			},
		},
	})

	if err != nil {
		t.Error(err)
	}

	resp, err := prxy(context.Background(), &proxy.Request{
		Method:  "GET",
		Path:    "/some-path",
		Params:  map[string]string{"Id": "42"},
		Headers: map[string][]string{},
		URL:     URL,
		Body:    io.NopCloser(strings.NewReader("initial req content")),
	})
	if err != nil {
		t.Errorf("unexpected error %s", err.Error())
		return
	}
	if resp.Metadata.StatusCode != 200 {
		t.Errorf("unexpected status code %d", resp.Metadata.StatusCode)
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
		"ids": {
			"1": 1,
			"2": 42,
			"3": 0
		},
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
		return func(ctx context.Context, req *proxy.Request) (*proxy.Response, error) {
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
		return func(ctx context.Context, req *proxy.Request) (*proxy.Response, error) {
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
		return func(ctx context.Context, req *proxy.Request) (*proxy.Response, error) {
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
		return func(ctx context.Context, req *proxy.Request) (*proxy.Response, error) {
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
