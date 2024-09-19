package gin

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/krakendio/krakend-lua/v2/router"
	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/logging"
	"github.com/luraproject/lura/v2/proxy"
)

func TestHandlerFactory(t *testing.T) {
	gin.SetMode(gin.TestMode)
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
		req:host(req:host() .. ".newtld")
		req:query("extra", "foo")
		req:body(req:body().."fooooooo")`,
			},
		},
	}

	hf := func(_ *config.EndpointConfig, _ proxy.Proxy) gin.HandlerFunc {
		return func(c *gin.Context) {
			if URL := c.Request.URL.String(); URL != "/some-path/42?extra=foo&id=1&more=true" {
				t.Errorf("unexpected URL: %s", URL)
			}
			if host := c.Request.Host; host != "domain.tld.newtld" {
				t.Errorf("unexpected Host: %s", host)
			}
			if accept := c.Request.Header.Get("Accept"); accept != "application/xml" {
				t.Errorf("unexpected accept header: %s", accept)
			}
			if c.Request.Method != "POST" {
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
			b, err := io.ReadAll(c.Request.Body)
			if err != nil {
				t.Error(err)
				return
			}
			if string(b) != "fooooooo" {
				t.Errorf("unexpected body: %s", string(b))
			}
		}
	}
	handler := HandlerFactory(logging.NoOp, hf)(cfg, proxy.NoopProxy)

	engine := gin.New()
	engine.GET("/some-path/:id", handler)

	req, _ := http.NewRequest("GET", "/some-path/42?id=1", nil)
	req.Host = "domain.tld"
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	engine.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("unexpected status code %d", w.Code)
		return
	}
}

func TestHandlerFactory_error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.EndpointConfig{
		Endpoint: "/",
		ExtraConfig: config.ExtraConfig{
			router.Namespace: map[string]interface{}{
				"pre": `custom_error('expect me')`,
			},
		},
	}

	hf := func(_ *config.EndpointConfig, _ proxy.Proxy) gin.HandlerFunc {
		return func(c *gin.Context) {
			t.Error("the handler shouldn't be executed")
		}
	}
	handler := HandlerFactory(logging.NoOp, hf)(cfg, proxy.NoopProxy)

	engine := gin.New()
	engine.GET("/some-path/:id", handler)

	req, _ := http.NewRequest("GET", "/some-path/42?id=1", nil)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	engine.ServeHTTP(w, req)

	if w.Code != 500 {
		t.Errorf("unexpected status code %d", w.Code)
		return
	}
}

func TestHandlerFactory_errorHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.EndpointConfig{
		Endpoint: "/",
		ExtraConfig: config.ExtraConfig{
			router.Namespace: map[string]interface{}{
				"pre": `custom_error('expect me', 999)`,
			},
		},
	}

	hf := func(_ *config.EndpointConfig, _ proxy.Proxy) gin.HandlerFunc {
		return func(c *gin.Context) {
			t.Error("the handler shouldn't be executed")
		}
	}
	handler := HandlerFactory(logging.NoOp, hf)(cfg, proxy.NoopProxy)

	engine := gin.New()
	engine.GET("/some-path/:id", handler)

	req, _ := http.NewRequest("GET", "/some-path/42?id=1", nil)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	engine.ServeHTTP(w, req)

	if w.Code != 999 {
		t.Errorf("unexpected status code %d", w.Code)
		return
	}
}
