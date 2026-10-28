package decorator

import (
	"errors"
	"fmt"
	"strconv"
	stdtime "time"

	lua "github.com/krakend/krakend-lua/v3"
)

var (
	errInvalidTime     = errors.New("invalid data, use \"time.parse\" or \"time.now\" to generate a valid time")
	errInvalidDuration = errors.New("invalid data, use \"time.parseduration\" to generate a valid duration")
)

func RegisterTime(b *lua.Binder) {
	t := b.Table("time")
	t.Static("parse", newTime())
	t.Static("parseduration", newDuration())
	t.Static("now", newTimeNow())

	t.Dynamic("unix", getUnixTime())
	t.Dynamic("format", formatTime())
	t.Dynamic("nanoseconds", getNanoseconds())
	t.Dynamic("string", formatDuration())
}

func newTime() lua.Handler {
	return func(c *lua.Context) error {
		if c.Top() != 2 {
			return errNeedArguments
		}

		layout := c.Arg(1).String()
		timeStr := c.Arg(2).String()

		time, err := stdtime.Parse(layout, timeStr)
		if err != nil {
			return err
		}

		c.Push().Data(time, "time")
		return nil
	}
}

func newTimeNow() lua.Handler {
	return func(c *lua.Context) error {
		c.Push().Data(stdtime.Now(), "time")
		return nil
	}
}

func getUnixTime() lua.Handler {
	return func(c *lua.Context) error {
		time, ok := c.Arg(1).Data().(stdtime.Time)
		if !ok {
			return errInvalidTime
		}
		c.Push().Number(float64(time.Unix()))
		return nil
	}
}

func formatTime() lua.Handler {
	return func(c *lua.Context) error {
		if c.Top() != 2 {
			return errNeedArguments
		}
		time, ok := c.Arg(1).Data().(stdtime.Time)
		if !ok {
			return errInvalidTime
		}

		c.Push().String(time.Format(c.Arg(2).String()))
		return nil
	}
}

func newDuration() lua.Handler {
	return func(c *lua.Context) error {
		if c.Top() != 1 {
			return errNeedArguments
		}

		input := c.Arg(1).String()
		if t, isInt := strconv.Atoi(input); isInt == nil {
			input = fmt.Sprintf("%dns", t)
		}

		duration, err := stdtime.ParseDuration(input)
		if err != nil {
			return err
		}

		c.Push().Data(duration, "time")
		return nil
	}
}

func getNanoseconds() lua.Handler {
	return func(c *lua.Context) error {
		duration, ok := c.Arg(1).Data().(stdtime.Duration)
		if !ok {
			return errInvalidDuration
		}
		c.Push().Number(float64(duration.Nanoseconds()))
		return nil
	}
}

func formatDuration() lua.Handler {
	return func(c *lua.Context) error {
		duration, ok := c.Arg(1).Data().(stdtime.Duration)
		if !ok {
			return errInvalidDuration
		}
		c.Push().String(duration.String())
		return nil
	}
}
