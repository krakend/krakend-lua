package decorator

import (
	"errors"
)

var (
	errNeedArguments = errors.New("need arguments")
	errInvalidType   = errors.New("invalid argument type")
)
