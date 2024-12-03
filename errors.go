package lua

import (
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

func ToError(e error, source *SourceMap) error {
	if e == nil {
		return nil
	}

	binderError, ok := e.(*binder.Error)
	if !ok {
		return e
	}

	originalMsg := binderError.Error()
	msgSplitIndex := strings.Index(originalMsg, ":")
	errMsgParts := strings.Split(originalMsg[msgSplitIndex+2:], " || ")

	if len(errMsgParts) == 0 {
		return binderError
	}

	// Internal errors not coming from a LUA custom_error()
	if len(errMsgParts) == 1 {
		if source == nil {
			return ErrInternal(errMsgParts[0])
		}
		luaLine, convErr := strconv.Atoi(originalMsg[5:msgSplitIndex])
		if convErr != nil {
			luaLine = 0
		}
		if affectedScript, relativeLine, err := source.AffectedSource(luaLine); err == nil {
			return ErrInternal(fmt.Sprintf("%s (%s:L%d)", errMsgParts[0], affectedScript, relativeLine))
		}
		return ErrInternal(errMsgParts[0])
	}

	// If the error was correctly splitted by separator means we're dealing with a LUA custom_error()
	code, err := strconv.Atoi(errMsgParts[1])
	if err != nil {
		code = 500
	}
	if code == -1 {
		return ErrInternal(errMsgParts[0])
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
