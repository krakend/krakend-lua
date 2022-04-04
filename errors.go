package lua

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/alexeyco/binder"
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

func ToError(e error) error {
	if e == nil {
		return nil
	}

	if _, ok := e.(*binder.Error); !ok {
		return e
	}

	originalMsg := e.Error()
	start := strings.Index(originalMsg, ":")

	if l := len(originalMsg); originalMsg[l-1] == ')' && originalMsg[l-5] == '(' {
		code, err := strconv.Atoi(originalMsg[l-4 : l-1])
		if err != nil {
			code = 500
		}
		return ErrInternalHTTP{msg: originalMsg[start+2 : l-6], code: code}
	}

	return ErrInternal(originalMsg[start+2:])
}

func RegisterErrors(b *binder.Binder) {
	b.Func("custom_error", func(c *binder.Context) error {
		switch c.Top() {
		case 0:
			return errNeedsArguments
		case 1:
			return errors.New(c.Arg(1).String())
		default:
			return fmt.Errorf("%s (%d)", c.Arg(1).String(), int(c.Arg(2).Number()))
		}
	})
}
