package mux

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
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
		req:url(req:url() .. "&more=true")
		req:query("extra", "foo")
		req:body(req:body().."fooooooo")`,
			},
		},
	}

	hf := func(_ *config.EndpointConfig, _ proxy.Proxy) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if URL := r.URL.String(); URL != "/some-path/42?extra=foo&id=1&more=true" {
				t.Errorf("unexpected URL: %s", URL)
			}
			if accept := r.Header.Get("Accept"); accept != "application/xml" {
				t.Errorf("unexpected accept header: %s", accept)
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
			b, err := ioutil.ReadAll(r.Body)
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

	req, _ := http.NewRequest("GET", "/some-path/42?id=1", nil)
	req.Header.Set("Accept", "application/json")
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
		return func(w http.ResponseWriter, r *http.Request) {
			t.Error("the handler shouldn't be executed")
		}
	}
	handler := HandlerFactory(logging.NoOp, hf, func(_ *http.Request) map[string]string {
		return map[string]string{}
	})(cfg, proxy.NoopProxy)

	req, _ := http.NewRequest("GET", "/some-path/42?id=1", nil)
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
		return func(w http.ResponseWriter, r *http.Request) {
			t.Error("the handler shouldn't be executed")
		}
	}
	handler := HandlerFactory(logging.NoOp, hf, func(_ *http.Request) map[string]string {
		return map[string]string{}
	})(cfg, proxy.NoopProxy)

	req, _ := http.NewRequest("GET", "/some-path/42?id=1", nil)
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
		return func(w http.ResponseWriter, r *http.Request) {
			t.Error("the handler shouldn't be executed")
		}
	}
	handler := HandlerFactory(logging.NoOp, hf, func(_ *http.Request) map[string]string {
		return map[string]string{}
	})(cfg, proxy.NoopProxy)

	req, _ := http.NewRequest("GET", "/some-path/42?id=1", nil)
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
