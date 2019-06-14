package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"testing"

	"github.com/devopsfaith/krakend/config"
	"github.com/devopsfaith/krakend/logging"
	"github.com/devopsfaith/krakend/proxy"
)

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
			},
		},
		Metadata: proxy.Metadata{
			Headers: map[string][]string{},
		},
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
		req:url(req:url() .. "&more=true")`,

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

		resp:headers("Content-Type", "application/xml")
		resp:statusCode(200)`,
			},
		},
	})

	resp, err := prxy(context.Background(), &proxy.Request{
		Method:  "GET",
		Path:    "/some-path",
		Params:  map[string]string{"Id": "42"},
		Headers: map[string][]string{},
		URL:     URL,
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
		}
	],
	"foo": "some_new_value",
	"more": {
		"bar": 120,
		"collection_size": 2,
		"foobar": true,
		"ids": {
			"1": 1,
			"2": 42
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
}
