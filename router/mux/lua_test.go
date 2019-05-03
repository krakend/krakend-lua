package mux

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devopsfaith/krakend-lua/router"
	"github.com/devopsfaith/krakend/config"
	"github.com/devopsfaith/krakend/logging"
	"github.com/devopsfaith/krakend/proxy"
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
		req:query("extra", "foo")`,
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
			if "POST" != r.Method {
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
