package proxy

import (
	"context"
	"errors"

	"github.com/alexeyco/binder"
	lua "github.com/devopsfaith/krakend-lua/v2"
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

func New(cfg lua.Config, next proxy.Proxy) proxy.Proxy {
	return func(ctx context.Context, req *proxy.Request) (resp *proxy.Response, err error) {
		b := binder.New(binder.Options{
			SkipOpenLibs:        !cfg.AllowOpenLibs,
			IncludeGoStackTrace: true,
		})

		lua.RegisterErrors(b)
		registerHTTPRequest(b)
		registerRequestTable(req, b)

		for _, source := range cfg.Sources {
			src, ok := cfg.Get(source)
			if !ok {
				return nil, lua.ErrUnknownSource(source)
			}
			if err := b.DoString(src); err != nil {
				return nil, lua.ToError(err)
			}
		}

		if err := b.DoString(cfg.PreCode); err != nil {
			return nil, lua.ToError(err)
		}

		if !cfg.SkipNext {
			resp, err = next(ctx, req)
			if err != nil {
				return resp, lua.ToError(err)
			}
		} else {
			resp = &proxy.Response{}
		}

		registerResponseTable(resp, b)

		err = lua.ToError(b.DoString(cfg.PostCode))

		return resp, err
	}
}

var errNeedsArguments = errors.New("need arguments")
