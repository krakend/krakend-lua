package mux

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"

	luarouter "github.com/krakend/krakend-lua/v3/router"
	"github.com/luraproject/lura/v3/config"
	"github.com/luraproject/lura/v3/logging"
)

func ExampleRegisterMiddleware_routerDebug() {
	logger := logging.BasicLogger{Level: logging.LEVEL_DEBUG, Prefix: "->", Logger: log.New(os.Stdout, "", 0)}

	cfg := config.ExtraConfig{
		luarouter.Namespace: map[string]interface{}{
			"pre": "debug.dump(ctx.load(), 'ctx object');debug.dump(2, 'number')",
		},
	}

	mws := RegisterMiddleware(logger, cfg, func(_ *http.Request) map[string]string {
		return map[string]string{
			"param1": "10",
			"param2": "testparam",
		}
	}, nil)

	req, _ := http.NewRequest("GET", "/ping/10/testparam?f1=v1&f2=v2", http.NoBody)
	req.Header.Add("X-Test", "test")
	req.Header.Add("X-Multi-Test", "test2")
	req.Header.Add("X-Multi-Test", "test3")

	var h http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	for _, mw := range mws {
		h = mw.Handler(h)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		fmt.Printf("unexpected status code: %d", w.Result().StatusCode)
		return
	}

	// output:
	// -> DEBUG: [Service: Mux][Lua] Middleware is now ready
	// -> DEBUG: [Lua] Dumping "ctx object"
	// (map)
	//   headers: (map)
	//     X-Multi-Test: [test2 test3] ([]string)
	//     X-Test: [test] ([]string)
	//   method: GET (string)
	//   params: (map)
	//     param1: 10 (string)
	//     param2: testparam (string)
	//   path: /ping/10/testparam (string)
	//   query: f1=v1&f2=v2 (string)
	//   url: /ping/10/testparam?f1=v1&f2=v2 (string)
	//
	// -> DEBUG: [Lua] Dumping "number"
	// 2 (int)
}
