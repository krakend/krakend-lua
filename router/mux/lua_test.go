package mux

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/krakendio/krakend-lua/v2/router"
	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/logging"
	"github.com/luraproject/lura/v2/proxy"
)

func TestHandlerFactory(t *testing.T) {
	cfg := &config.EndpointConfig{
		Endpoint: "/",
		ExtraConfig: config.ExtraConfig{
			router.Namespace: map[string]interface{}{
				"sources": []interface{}{
					"../../lua/factorial.lua",
				},

				"pre": `local req = ctx.load()
		req:method("POST")
		req:params("foo", "some_new_value")
		req:headers("Accept", "application/xml")
		req:headers("X-To-Delete", nil)
		req:headers("X-TO-DELETE-LOWER", nil)
		local multi = luaList.new()
		multi:set(0, "A")
		multi:set(1, "B")
		req:headerList("X-Multi", multi)
		req:url(req:url() .. "&more=true")
		req:query("extra", "foo")
		req:body(req:body().."fooooooo")`,
			},
		},
	}

	hf := func(_ *config.EndpointConfig, _ proxy.Proxy) http.HandlerFunc {
		return func(_ http.ResponseWriter, r *http.Request) {
			if URL := r.URL.String(); URL != "/some-path/42?extra=foo&id=1&more=true" {
				t.Errorf("unexpected URL: %s", URL)
			}
			if accept := r.Header.Get("Accept"); accept != "application/xml" {
				t.Errorf("unexpected accept header: %s", accept)
			}
			if multi := r.Header.Values("X-Multi"); multi[0] != "A" || multi[1] != "B" {
				t.Errorf("unexpected X-Multi header: %v", multi)
			}
			if toDelete := r.Header.Get("X-To-Delete"); len(toDelete) > 0 {
				t.Error("unexpected header 'X-To-Delete', should have been deleted")
			}
			if toDeleteLower := r.Header.Get("X-To-Delete-Lower"); len(toDeleteLower) > 0 {
				t.Error("unexpected header 'X-To-Delete-Lower', should have been deleted")
			}
			if r.Method != "POST" {
				t.Errorf("unexpected method: %s", r.Method)
			}
			if e := r.URL.Query().Get("extra"); e != "foo" {
				t.Errorf("unexpected querystring extra: '%s' %v", e, r.URL.Query())
			}
			// if foo := c.Param("foo"); foo != "some_new_value" {
			// 	t.Errorf("unexpected param foo: %s", foo)
			// }
			// if id := c.Param("id"); id != "42" {
			// 	t.Errorf("unexpected param id: %s", id)
			// }
			b, err := io.ReadAll(r.Body)
			if err != nil {
				t.Error(err)
				return
			}
			if string(b) != "fooooooo" {
				t.Errorf("unexpected body: %s", string(b))
			}
		}
	}
	handler := HandlerFactory(logging.NoOp, hf, func(_ *http.Request) map[string]string {
		return map[string]string{}
	})(cfg, proxy.NoopProxy)

	req, _ := http.NewRequest("GET", "/some-path/42?id=1", http.NoBody)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-To-Delete", "deleteme")
	req.Header.Set("x-to-delete-lower", "deleteme")
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != 200 {
		t.Errorf("unexpected status code %d", w.Code)
		return
	}
}

func TestHandlerFactory_error(t *testing.T) {
	cfg := &config.EndpointConfig{
		Endpoint: "/",
		ExtraConfig: config.ExtraConfig{
			router.Namespace: map[string]interface{}{
				"pre": `custom_error('expect me')`,
			},
		},
	}

	hf := func(_ *config.EndpointConfig, _ proxy.Proxy) http.HandlerFunc {
		return func(_ http.ResponseWriter, _ *http.Request) {
			t.Error("the handler shouldn't be executed")
		}
	}
	handler := HandlerFactory(logging.NoOp, hf, func(_ *http.Request) map[string]string {
		return map[string]string{}
	})(cfg, proxy.NoopProxy)

	req, _ := http.NewRequest("GET", "/some-path/42?id=1", http.NoBody)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != 500 {
		t.Errorf("unexpected status code %d", w.Code)
		return
	}
}

func TestHandlerFactory_errorHTTP(t *testing.T) {
	cfg := &config.EndpointConfig{
		Endpoint: "/",
		ExtraConfig: config.ExtraConfig{
			router.Namespace: map[string]interface{}{
				"pre": `custom_error('expect me', 999)`,
			},
		},
	}

	hf := func(_ *config.EndpointConfig, _ proxy.Proxy) http.HandlerFunc {
		return func(_ http.ResponseWriter, _ *http.Request) {
			t.Error("the handler shouldn't be executed")
		}
	}
	handler := HandlerFactory(logging.NoOp, hf, func(_ *http.Request) map[string]string {
		return map[string]string{}
	})(cfg, proxy.NoopProxy)

	req, _ := http.NewRequest("GET", "/some-path/42?id=1", http.NoBody)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != 999 {
		t.Errorf("unexpected status code %d", w.Code)
		return
	}
}

func TestHandlerFactory_errorHTTPWithContentType(t *testing.T) {
	cfg := &config.EndpointConfig{
		Endpoint: "/",
		ExtraConfig: config.ExtraConfig{
			router.Namespace: map[string]interface{}{
				"pre": `custom_error('expect me', 999, 'foo/bar')`,
			},
		},
	}

	hf := func(_ *config.EndpointConfig, _ proxy.Proxy) http.HandlerFunc {
		return func(_ http.ResponseWriter, _ *http.Request) {
			t.Error("the handler shouldn't be executed")
		}
	}
	handler := HandlerFactory(logging.NoOp, hf, func(_ *http.Request) map[string]string {
		return map[string]string{}
	})(cfg, proxy.NoopProxy)

	req, _ := http.NewRequest("GET", "/some-path/42?id=1", http.NoBody)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != 999 {
		t.Errorf("unexpected status code %d", w.Code)
		return
	}

	if h := w.Header().Get("content-type"); h != "foo/bar" {
		t.Errorf("unexpected content-type %s", h)
		return
	}
}

func TestHandlerFactory_luaError(t *testing.T) {
	var luaPreErrorTestTable = []struct {
		Name          string
		Cfg           map[string]interface{}
		ExpectedError string
	}{
		{
			Name: "Pre: Syntax error",
			Cfg: map[string]interface{}{
				"pre": "local c = ctx.load()\nlokal a = 1()\nlocal b = 2",
			},
			ExpectedError: "'a': parse error (pre-script:L2)",
		},
		{
			Name: "Pre: Inline syntax error",
			Cfg: map[string]interface{}{
				"pre": "local c = ctx.load();lokal a = 1();local b = 2",
			},
			ExpectedError: "'a': parse error (pre-script:L1)",
		},
		{
			Name: "Pre: Inline semicolon separated",
			Cfg: map[string]interface{}{
				"pre": "local c = ctx.load();method_does_not_exist();local test = 1",
			},
			ExpectedError: "attempt to call a non-function object (pre-script:L1)",
		},
		{
			Name: "Pre: Inline",
			Cfg: map[string]interface{}{
				"pre": "local c = ctx.load()\nmethod_does_not_exist()\nlocal test = 1",
			},
			ExpectedError: "attempt to call a non-function object (pre-script:L2)",
		},
		{
			Name: "Pre: Multiline",
			Cfg: map[string]interface{}{
				"pre": `local req = ctx.load()
						req:method("POST")
						req:params("foo", "some_new_value")
						req:headers("Accept", "application/xml")
						req:url(req:url() .. "&more=true")
						req:query("extra", "foo")
						reqw:body(req:body().."fooooooo")`,
			},
			ExpectedError: "attempt to index a non-table object(nil) with key 'body' (pre-script:L7)",
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
					"../../lua/bad-code.lua",
				},
				"pre": "custom_error(\"wont reach here\")",
			},
			ExpectedError: "attempt to index a non-table object(function) with key 'really_bad' (bad-code.lua:L5)",
		},
		{
			Name: "Pre: Single source with bad method implementation",
			Cfg: map[string]interface{}{
				"sources": []interface{}{
					"../../lua/bad-func.lua",
				},
				"pre": "badfunc(1)",
			},
			ExpectedError: "attempt to call a non-function object (bad-func.lua:L3)",
		},
		{
			Name: "Pre: Multiple sources",
			Cfg: map[string]interface{}{
				"sources": []interface{}{
					"../../lua/factorial.lua",
					"../../lua/bad-code.lua",
					"../../lua/add.lua",
				},
				"pre": "custom_error(\"wont reach here\")",
			},
			ExpectedError: "attempt to index a non-table object(function) with key 'really_bad' (bad-code.lua:L5)",
		},
		{
			Name: "Pre: Multiple sources, bad function call",
			Cfg: map[string]interface{}{
				"sources": []interface{}{
					"../../lua/env.lua",
					"../../lua/factorial.lua",
					"../../lua/bad-func.lua",
					"../../lua/add.lua",
				},
				"pre": "badfunc(1)",
			},
			ExpectedError: "attempt to call a non-function object (bad-func.lua:L3)",
		},
	}

	for _, test := range luaPreErrorTestTable {
		t.Run(test.Name, func(t *testing.T) {
			cfg := &config.EndpointConfig{
				Endpoint: "/",
				ExtraConfig: config.ExtraConfig{
					router.Namespace: test.Cfg,
				},
			}

			hf := func(_ *config.EndpointConfig, _ proxy.Proxy) http.HandlerFunc {
				return func(_ http.ResponseWriter, _ *http.Request) {
					t.Error("the handler shouldn't be executed")
				}
			}
			handler := HandlerFactory(logging.NoOp, hf, func(_ *http.Request) map[string]string {
				return map[string]string{}
			})(cfg, proxy.NoopProxy)

			req, _ := http.NewRequest("GET", "/some-path/42?id=1", http.NoBody)
			req.Header.Set("Accept", "application/json")
			w := httptest.NewRecorder()

			handler(w, req)

			reqErr := strings.Trim(w.Body.String(), "\n")
			if reqErr != test.ExpectedError {
				t.Errorf("unexpected error, have: '%s', want: '%s'", reqErr, test.ExpectedError)
			}
		})
	}

}
