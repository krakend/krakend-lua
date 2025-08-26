package decorator

import (
	"errors"

	"github.com/krakend/binder"
)

type Decorator func(*binder.Binder)

var (
	ErrNeedsArguments   = errors.New("need arguments")
	ErrResponseExpected = errors.New("response expected")
)
