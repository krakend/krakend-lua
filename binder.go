package lua

import (
	"strings"

	"github.com/krakend/binder"
)

type Binder = binder.Binder
type Context = binder.Context
type Handler = binder.Handler

type BinderWrapper struct {
	binder    *binder.Binder
	sourceMap *SourceMap
}

func NewBinderWrapper(binderOptions binder.Options) BinderWrapper {
	b := binder.New(binderOptions)
	m := NewSourceMap()
	return BinderWrapper{b, &m}
}

func (b BinderWrapper) GetBinder() *binder.Binder {
	return b.binder
}

func (b BinderWrapper) WithConfig(cfg *Config) error {
	var srcBlock []string
	for _, source := range cfg.Sources {
		src, ok := cfg.Get(source)
		if !ok {
			return ErrUnknownSource(source)
		}
		srcBlock = append(srcBlock, src)
		b.sourceMap.Append(source, src)
	}
	if len(srcBlock) > 0 {
		if err := b.binder.DoString(strings.Join(srcBlock, "\n")); err != nil {
			return ToError(err, b.sourceMap)
		}
	}

	return nil
}

func (b BinderWrapper) WithCode(key, src string) error {
	if err := b.binder.DoString(src); err != nil {
		v := *b.sourceMap
		v.Append(key, src)

		return ToError(err, &v)
	}
	return nil
}
