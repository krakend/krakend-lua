package proxy

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"sync"

	"github.com/alexeyco/binder"
)

func registerHTTPRequest(b *binder.Binder) {
	t := b.Table("http_response")

	t.Static("new", newHttpResponse)

	t.Dynamic("statusCode", httpStatus)
	t.Dynamic("headers", httpHeaders)
	t.Dynamic("body", httpBody)
}

func newHttpResponse(c *binder.Context) error {
	if c.Top() == 0 || c.Top() == 2 {
		return errors.New("need 1 or 3 arguments")
	}

	URL := c.Arg(1).String()
	if c.Top() == 1 {
		resp, err := http.Get(URL)
		if err != nil {
			return err
		}
		pushHTTPResponse(c, resp)
		return nil
	}

	method := c.Arg(2).String()
	body := c.Arg(3).String()

	req, err := http.NewRequest(method, URL, bytes.NewBufferString(body))
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	if resp == nil {
		return errResponseExpected
	}
	pushHTTPResponse(c, resp)
	return nil
}

type httpResponse struct {
	once *sync.Once
	r    *http.Response
	body string
}

func (h *httpResponse) Body() string {
	h.once.Do(func() {
		b, _ := ioutil.ReadAll(h.r.Body)
		h.r.Body.Close()
		h.body = string(b)
	})
	return h.body
}

func (h *httpResponse) Header(k string) string {
	return h.r.Header.Get(k)
}

func pushHTTPResponse(c *binder.Context, r *http.Response) {
	c.Push().Data(
		&httpResponse{
			once: new(sync.Once),
			r:    r,
		},
		"http_response",
	)
}

func httpStatus(c *binder.Context) error {
	resp, ok := c.Arg(1).Data().(*httpResponse)
	if !ok {
		return errResponseExpected
	}
	c.Push().Number(float64(resp.r.StatusCode))

	return nil
}

func httpHeaders(c *binder.Context) error {
	resp, ok := c.Arg(1).Data().(*httpResponse)
	if !ok {
		return errResponseExpected
	}
	if c.Top() != 2 {
		return errNeedsArguments
	}
	c.Push().String(resp.Header(c.Arg(2).String()))

	return nil
}

func httpBody(c *binder.Context) error {
	resp, ok := c.Arg(1).Data().(*httpResponse)
	if !ok {
		return errResponseExpected
	}
	c.Push().String(resp.Body())

	return nil
}
