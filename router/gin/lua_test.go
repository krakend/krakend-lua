package gin

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devopsfaith/krakend-lua/router"
	"github.com/devopsfaith/krakend/config"
	"github.com/devopsfaith/krakend/logging"
	"github.com/devopsfaith/krakend/proxy"
	"github.com/gin-gonic/gin"
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
		req:body("fooooooo")`,
			},
		},
	}

	hf := func(_ *config.EndpointConfig, _ proxy.Proxy) gin.HandlerFunc {
		return func(c *gin.Context) {
			if URL := c.Request.URL.String(); URL != "/some-path/42?extra=foo&id=1&more=true" {
				t.Errorf("unexpected URL: %s", URL)
			}
			if accept := c.Request.Header.Get("Accept"); accept != "application/xml" {
				t.Errorf("unexpected accept header: %s", accept)
			}
			if "POST" != c.Request.Method {
				t.Errorf("unexpected method: %s", c.Request.Method)
			}
			if foo := c.Param("foo"); foo != "some_new_value" {
				t.Errorf("unexpected param foo: %s", foo)
			}
			if id := c.Param("id"); id != "42" {
				t.Errorf("unexpected param id: %s", id)
			}
			if e := c.Query("extra"); e != "foo" {
				t.Errorf("unexpected querystring extra: '%s'", e)
			}
			b, err := ioutil.ReadAll(c.Request.Body)
			if err != nil {
				t.Error(err)
				return
			}
			if "fooooooo" != string(b) {
				t.Errorf("unexpected body: %s", string(b))
			}
		}
	}
	handler := HandlerFactory(logging.NoOp, hf)(cfg, proxy.NoopProxy)

	engine := gin.New()
	engine.GET("/some-path/:id", handler)

	req, _ := http.NewRequest("GET", "/some-path/42?id=1", nil)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	engine.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("unexpected status code %d", w.Code)
		return
	}
}
