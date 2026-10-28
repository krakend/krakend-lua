package proxy

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"

	"github.com/luraproject/lura/v3/config"
	"github.com/luraproject/lura/v3/logging"
	"github.com/luraproject/lura/v3/proxy"
)

var preCode = func(server *httptest.Server) string {
	return `
		local noLabel = 1
		debug.dump(noLabel)

		local n = 1
		debug.dump(n, "n")

		local text = "sample text"
		debug.dump(text, "text")

		local innerCustomTable = luaTable.new()
		innerCustomTable:set("k1", "v1")

		local customList = luaList.new()
		customList:set(0, "t0")
		customList:set(1, "t1")
		customList:set(2, "t2")
		customList:set(3, innerCustomTable)

		local customTable = luaTable.new()
		customTable:set("t0", "v0")
		customTable:set("t1", "v1")
		customTable:set("t2", customList)
		debug.dump(customList, "customList")
		debug.dump(customTable, "customTable")

		debug.dump(request.load(), "request object")

		local hr = http_response.new('` + server.URL + `', 'GET', '{}', {["X-Request"]="abcd"})
		debug.dump(hr, "http response object")
	`
}

var postCode = func() string { return "debug.dump(response.load(), \"response object\")" }

func ExampleRegisterDecorator_debug() {
	logger := logging.BasicLogger{Level: logging.LEVEL_DEBUG, Prefix: "->", Logger: log.New(os.Stdout, "", 0)}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Add("X-Response", "A")
		w.Header().Set("Date", "static")
		fmt.Fprintln(w, "Hello, client")
	}))
	defer ts.Close()

	pf := proxy.FactoryFunc(func(_ *config.EndpointConfig) (proxy.Proxy, error) {
		return func(_ context.Context, _ *proxy.Request) (*proxy.Response, error) {
			return &proxy.Response{
				Data: map[string]interface{}{
					"f1": "v1",
					"f2": "v2",
				},
				IsComplete: true,
				Metadata: proxy.Metadata{
					StatusCode: 200,
					Headers: map[string][]string{
						"X-Response":    {"a"},
						"X-Multi-Value": {"v1", "v2"},
					},
				},
			}, nil
		}, nil
	})

	pr, err := ProxyFactory(logger, pf).New(&config.EndpointConfig{
		Endpoint: "/",
		ExtraConfig: config.ExtraConfig{
			ProxyNamespace: map[string]interface{}{
				"pre":  preCode(ts),
				"post": postCode(),
			},
		},
	})
	if err != nil {
		fmt.Printf("error creating proxy: %s", err.Error())
		return
	}

	URL, _ := url.Parse("https://somewhere.tld/")

	resp, err := pr(context.Background(), &proxy.Request{
		Method:  "GET",
		Path:    "/",
		Params:  map[string]string{"user": "pepe"},
		Headers: map[string][]string{"X-Correlation-Id": {"something"}, "X-Some-List": {"i1", "i2"}},
		URL:     URL,
	})
	if err != nil {
		fmt.Printf("unexpected response error: %s", err.Error())
		return
	}

	if resp == nil {
		fmt.Printf("expected response")
		return
	}

	// output:
	// -> DEBUG: [ENDPOINT:  /][Lua] Middleware is now ready
	// -> DEBUG: [Lua]
	// 1 (int)
	//
	// -> DEBUG: [Lua] Dumping "n"
	// 1 (int)
	//
	// -> DEBUG: [Lua] Dumping "text"
	// sample text (string)
	//
	// -> DEBUG: [Lua] Dumping "customList"
	// (array)
	//   - t0 (string)
	//   - t1 (string)
	//   - t2 (string)
	//   - (map)
	//     k1: v1 (string)
	//
	// -> DEBUG: [Lua] Dumping "customTable"
	// (map)
	//   t0: v0 (string)
	//   t1: v1 (string)
	//   t2: (array)
	//     - t0 (string)
	//     - t1 (string)
	//     - t2 (string)
	//     - (map)
	//       k1: v1 (string)
	//
	// -> DEBUG: [Lua] Dumping "request object"
	// (map)
	//   headers: (map)
	//     X-Correlation-Id: [something] ([]string)
	//     X-Some-List: [i1 i2] ([]string)
	//   method: GET (string)
	//   params: (map)
	//     user: pepe (string)
	//   path: / (string)
	//   query:  (string)
	//   url: https://somewhere.tld/ (string)
	//
	// -> DEBUG: [Lua] Dumping "http response object"
	// (map)
	//   request: (map)
	//     headers: (map)
	//       User-Agent: [KrakenD Version undefined] ([]string)
	//       X-Request: [abcd] ([]string)
	//     method: GET (string)
	//   response: (map)
	//     body: Hello, client
	//  (string)
	//     headers: (map)
	//       Content-Length: [14] ([]string)
	//       Content-Type: [text/plain; charset=utf-8] ([]string)
	//       Date: [static] ([]string)
	//       X-Response: [A] ([]string)
	//     statusCode: 200 (float64)
	//
	// -> DEBUG: [Lua] Dumping "response object"
	// (map)
	//   data: (map)
	//     f1: v1 (string)
	//     f2: v2 (string)
	//   headers: (map)
	//     X-Multi-Value: [v1 v2] ([]string)
	//     X-Response: [a] ([]string)
	//   isComplete: true (bool)
	//   statusCode: 200 (float64)
}
