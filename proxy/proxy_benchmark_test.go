package proxy

import (
	"bytes"
	"context"
	"net/url"
	"testing"

	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/logging"
	"github.com/luraproject/lura/v2/proxy"
)

var localResponse *proxy.Response

func BenchmarkProxyFactory(b *testing.B) {
	buff := bytes.NewBuffer(make([]byte, 1024))
	logger, err := logging.NewLogger("ERROR", buff, "pref")
	if err != nil {
		b.Error("building the logger:", err.Error())
		return
	}

	dummyProxyFactory := proxy.FactoryFunc(func(_ *config.EndpointConfig) (proxy.Proxy, error) {
		return func(_ context.Context, _ *proxy.Request) (*proxy.Response, error) {
			return &proxy.Response{
				Data: map[string]interface{}{"ok": true},
				Metadata: proxy.Metadata{
					Headers: map[string][]string{},
				},
			}, nil
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

				"pre": `local req = request.load()
		req:method("POST")
		req:params("foo", "some_new_value")
		req:headers("Accept", "application/xml")
		req:url(req:url() .. "&more=true")`,

				"post": `local resp = response.load()
		resp:isComplete(true)
		resp:data("foo", "some_new_value")

		data = {}
		data["bar"] = fact(5)
		data["foobar"] = true
		data["supu"] = {}
		data["supu"]["tupu"] = "some"
		data["supu"]["original"] = resp:data("ok")
		resp:data("more", data)

		resp:headers("Content-Type", "application/xml")
		resp:statusCode(200)`,
			},
		},
	})

	if err != nil {
		b.Error(err)
	}

	var resp *proxy.Response

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		resp, _ = prxy(context.Background(), &proxy.Request{
			Method:  "GET",
			Path:    "/some-path",
			Params:  map[string]string{"Id": "42"},
			Headers: map[string][]string{},
			URL:     URL,
		})
	}
	localResponse = resp
}
