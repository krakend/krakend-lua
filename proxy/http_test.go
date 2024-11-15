package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/krakendio/binder"
	lua "github.com/krakendio/krakend-lua/v2"
)

func Example_RegisterBackendModule() {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers, _ := json.Marshal(r.Header)
		fmt.Println(string(headers))
		fmt.Println(r.Method)

		if r.Body != nil {
			body, _ := io.ReadAll(r.Body)
			fmt.Println(string(body))
			r.Body.Close()
		}
		fmt.Fprintln(w, "Hello, client")
	}))
	defer ts.Close()

	bindr := lua.NewBinderWrapper(binder.Options{
		SkipOpenLibs:        true,
		IncludeGoStackTrace: true,
	})

	registerHTTPRequest(context.Background(), bindr.GetBinder())

	code := fmt.Sprintf("local url = '%s'\n%s", ts.URL, sampleLuaCode)

	if err := bindr.WithCode("test-code", code); err != nil {
		fmt.Println(err.Error())
	}

	// output:
	// lua http test
	//
	// {"123":["456"],"Accept-Encoding":["gzip"],"Content-Length":["13"],"Foo":["bar"],"User-Agent":["KrakenD Version undefined"]}
	// POST
	// {"foo":"bar"}
	// 200
	// text/plain; charset=utf-8
	// Hello, client
	//
	// {"Accept-Encoding":["gzip"],"Content-Length":["13"],"User-Agent":["KrakenD Version undefined"]}
	// POST
	// {"foo":"bar"}
	// 200
	// text/plain; charset=utf-8
	// Hello, client
	//
	// {"Accept-Encoding":["gzip"],"User-Agent":["KrakenD Version undefined"]}
	// GET
	//
	// 200
	// text/plain; charset=utf-8
	// Hello, client
}

const sampleLuaCode = `print("lua http test\n")
local r = http_response.new(url, "POST", '{"foo":"bar"}', {["foo"] = "bar", ["123"] = "456"})
print(r:statusCode())
print(r:headers('Content-Type'))
print(r:body())
r:close()

local r = http_response.new(url, "POST", '{"foo":"bar"}')
print(r:statusCode())
print(r:headers('Content-Type'))
print(r:body())
r:close()

local r = http_response.new(url)
print(r:statusCode())
print(r:headers('Content-Type'))
print(r:body())
r:close()
`
