package decorator

import (
	"fmt"

	"github.com/krakendio/binder"
)

const separator = " || "

func RegisterErrors(b *binder.Binder) {
	b.Func("custom_error", func(c *binder.Context) error {
		switch c.Top() {
		case 0:
			return ErrNeedsArguments
		case 1:
			return fmt.Errorf("%s%s%d", c.Arg(1).String(), separator, -1)
		case 2:
			return fmt.Errorf("%s%s%d", c.Arg(1).String(), separator, int(c.Arg(2).Number()))
		default:
			return fmt.Errorf("%s%s%d%s%s", c.Arg(1).String(), separator, int(c.Arg(2).Number()), separator, c.Arg(3).String())
		}
	})
}
