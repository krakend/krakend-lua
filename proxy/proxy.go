package proxy

import (
	"context"
	"errors"

	"github.com/krakendio/binder"
	lua "github.com/krakendio/krakend-lua/v2"
	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/logging"
	"github.com/luraproject/lura/v2/proxy"
)

const (
	ProxyNamespace   = "github.com/devopsfaith/krakend-lua/proxy"
	BackendNamespace = "github.com/devopsfaith/krakend-lua/proxy/backend"
)

func ProxyFactory(l logging.Logger, pf proxy.Factory) proxy.Factory {
	return proxy.FactoryFunc(func(remote *config.EndpointConfig) (proxy.Proxy, error) {
		logPrefix := "[ENDPOINT: " + remote.Endpoint + "][Lua]"
		next, err := pf.New(remote)
		if err != nil {
			return next, err
		}

		cfg, err := lua.Parse(l, remote.ExtraConfig, ProxyNamespace)
		if err != nil {
			if err != lua.ErrNoExtraConfig {
				l.Debug(logPrefix, err)
			}
			return next, nil
		}

		l.Debug(logPrefix, "Middleware is now ready")

		return New(cfg, next), nil
	})
}

func BackendFactory(l logging.Logger, bf proxy.BackendFactory) proxy.BackendFactory {
	return func(remote *config.Backend) proxy.Proxy {
		logPrefix := "[BACKEND: " + remote.URLPattern + "][Lua]"
		next := bf(remote)

		cfg, err := lua.Parse(l, remote.ExtraConfig, BackendNamespace)
		if err != nil {
			if err != lua.ErrNoExtraConfig {
				l.Debug(logPrefix, err)
			}
			return next
		}

		return New(cfg, next)
	}
}

type registerer struct {
	decorators []func(*binder.Binder)
}

var localRegisterer = registerer{decorators: []func(*binder.Binder){}}

func RegisterDecorator(f func(*binder.Binder)) {
	localRegisterer.decorators = append(localRegisterer.decorators, f)
}

func New(cfg lua.Config, next proxy.Proxy) proxy.Proxy {
	return func(ctx context.Context, req *proxy.Request) (resp *proxy.Response, err error) {
		b := lua.NewBinderWrapper(binder.Options{
			SkipOpenLibs:        !cfg.AllowOpenLibs,
			IncludeGoStackTrace: true,
		})
		defer b.GetBinder().Close()

		lua.RegisterErrors(b.GetBinder())
		lua.RegisterNil(b.GetBinder())
		registerHTTPRequest(ctx, b.GetBinder())
		registerRequestTable(req, b.GetBinder())

		if err := b.WithConfig(&cfg); err != nil {
			return nil, err
		}

		if err := b.WithCode("pre-script", cfg.PreCode); err != nil {
			return nil, err
		}

		if !cfg.SkipNext {
			resp, err = next(ctx, req)
			if err != nil {
				return resp, lua.ToError(err, nil)
			}
		} else {
			resp = &proxy.Response{}
		}

		registerResponseTable(resp, b.GetBinder())
		registerJson(b.GetBinder())

		for _, f := range localRegisterer.decorators {
			f(b.GetBinder())
		}

		if err := b.WithCode("post-script", cfg.PostCode); err != nil {
			return nil, err
		}

		return resp, nil
	}
}

var errNeedsArguments = errors.New("need arguments")
