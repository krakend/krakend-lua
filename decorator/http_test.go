package decorator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/krakend/binder"
)

func ExampleRegisterHTTPRequest() {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers, _ := json.Marshal(r.Header)
		fmt.Println(string(headers))
		fmt.Println(r.Method)

		w.Header().Add("X-Multi", "A")
		w.Header().Add("X-Multi", "B")

		if r.Body != nil {
			body, _ := io.ReadAll(r.Body)
			fmt.Println(string(body))
			r.Body.Close()
		}
		fmt.Fprintln(w, "Hello, client")
	}))
	defer ts.Close()

	bindr := binder.New(binder.Options{
		SkipOpenLibs:        true,
		IncludeGoStackTrace: true,
	})

	RegisterLuaList(bindr)
	RegisterHTTPRequest(context.Background(), bindr)

	code := fmt.Sprintf("local url = '%s'\n%s", ts.URL, sampleLuaCode)

	if err := bindr.DoString(code); err != nil {
		fmt.Println(err.Error())
	}

	// output:
	// lua http test
	//
	// {"123":["456"],"Accept-Encoding":["gzip"],"Content-Length":["13"],"Foo":["bar"],"Multi":["a","b"],"User-Agent":["KrakenD Version undefined"]}
	// POST
	// {"foo":"bar"}
	// 200
	// text/plain; charset=utf-8
	// A
	// B
	// Hello, client
	//
	// {"Accept-Encoding":["gzip"],"Content-Length":["13"],"User-Agent":["KrakenD Version undefined"]}
	// POST
	// {"foo":"bar"}
	// 200
	// text/plain; charset=utf-8
	// A
	// B
	// Hello, client
	//
	// {"Accept-Encoding":["gzip"],"User-Agent":["KrakenD Version undefined"]}
	// GET
	//
	// 200
	// text/plain; charset=utf-8
	// A
	// B
	// Hello, client
}

const sampleLuaCode = `print("lua http test\n")
local r = http_response.new(url, "POST", '{"foo":"bar"}', {["foo"] = "bar", ["123"] = "456", ["multi"] = {"a", "b"}})
print(r:statusCode())
print(r:headers('Content-Type'))
local hr = r:headerList('X-Multi')
print(hr:get(0))
print(hr:get(1))
print(r:body())
r:close()

local r = http_response.new(url, "POST", '{"foo":"bar"}')
print(r:statusCode())
print(r:headers('Content-Type'))
local hr = r:headerList('X-Multi')
print(hr:get(0))
print(hr:get(1))
print(r:body())
r:close()

local r = http_response.new(url)
print(r:statusCode())
print(r:headers('Content-Type'))
local hr = r:headerList('X-Multi')
print(hr:get(0))
print(hr:get(1))
print(r:body())
r:close()
`
