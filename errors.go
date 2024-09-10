package lua

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/krakendio/binder"
)

type ErrWrongChecksumType string

func (e ErrWrongChecksumType) Error() string {
	return "lua: wrong cheksum type for source " + string(e)
}

type ErrWrongChecksum struct {
	Source, Actual, Expected string
}

func (e ErrWrongChecksum) Error() string {
	return fmt.Sprintf("lua: wrong cheksum for source %s. have: %v, want: %v", e.Source, e.Actual, e.Expected)
}

type ErrUnknownSource string

func (e ErrUnknownSource) Error() string {
	return "lua: unable to load required source " + string(e)
}

var errNeedsArguments = errors.New("need arguments")

type ErrInternal string

func (e ErrInternal) Error() string {
	return string(e)
}

type ErrInternalHTTP struct {
	msg  string
	code int
}

func (e ErrInternalHTTP) StatusCode() int {
	return e.code
}

func (e ErrInternalHTTP) Error() string {
	return e.msg
}

type ErrInternalHTTPWithContentType struct {
	ErrInternalHTTP
	contentType string
}

func (e ErrInternalHTTPWithContentType) Encoding() string {
	return e.contentType
}

const separator = " || "

func ToError(e error) error {
	if e == nil {
		return nil
	}

	if _, ok := e.(*binder.Error); !ok {
		return e
	}

	originalMsg := e.Error()
	start := strings.Index(originalMsg, ":")
	errMsg := originalMsg[start+2:]
	errMsgParts := strings.Split(errMsg, separator)

	if len(errMsgParts) == 0 {
		return e
	}
	if len(errMsgParts) == 1 {
		return ErrInternal(errMsgParts[0])
	}

	code, err := strconv.Atoi(errMsgParts[1])
	if err != nil {
		code = 500
	}
	errHTTP := ErrInternalHTTP{msg: errMsgParts[0], code: code}

	if len(errMsgParts) == 2 {
		return errHTTP
	}

	return ErrInternalHTTPWithContentType{
		ErrInternalHTTP: errHTTP,
		contentType:     errMsgParts[2],
	}
}

func RegisterErrors(b *binder.Binder) {
	b.Func("custom_error", func(c *binder.Context) error {
		switch c.Top() {
		case 0:
			return errNeedsArguments
		case 1:
			return errors.New(c.Arg(1).String())
		case 2:
			return fmt.Errorf("%s%s%d", c.Arg(1).String(), separator, int(c.Arg(2).Number()))
		default:
			return fmt.Errorf("%s%s%d%s%s", c.Arg(1).String(), separator, int(c.Arg(2).Number()), separator, c.Arg(3).String())
		}
	})
}
