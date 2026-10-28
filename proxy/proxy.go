package proxy

import (
	"context"
	"errors"
	"fmt"

	"github.com/krakend/binder"
	lua "github.com/krakend/krakend-lua/v3"
	"github.com/krakend/krakend-lua/v3/decorator"
	"github.com/luraproject/lura/v3/config"
	"github.com/luraproject/lura/v3/logging"
	"github.com/luraproject/lura/v3/proxy"
)

const (
	ProxyNamespace   = "modifier/lua-proxy"
	BackendNamespace = "modifier/lua-backend"
)

func ProxyFactory(l logging.Logger, pf proxy.Factory) proxy.Factory {
	return proxy.FactoryFunc(func(remote *config.EndpointConfig) (proxy.Proxy, error) {
		logPrefix := fmt.Sprintf("[ENDPOINT: %s %s][Lua]", remote.Method, remote.Endpoint)
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

		return NewWithLogger(l, cfg, next), nil
	})
}

func BackendFactory(l logging.Logger, bf proxy.BackendFactory) proxy.BackendFactory {
	return func(remote *config.Backend) proxy.Proxy {
		logPrefix := fmt.Sprintf("[BACKEND: %s %s -> %s %s][Lua]",
			remote.ParentEndpointMethod, remote.ParentEndpoint,
			remote.Method, remote.URLPattern)
		next := bf(remote)

		cfg, err := lua.Parse(l, remote.ExtraConfig, BackendNamespace)
		if err != nil {
			if err != lua.ErrNoExtraConfig {
				l.Debug(logPrefix, err)
			}
			return next
		}

		return NewWithLogger(l, cfg, next)
	}
}

type registerer struct {
	decorators []decorator.Decorator
}

var localRegisterer = registerer{decorators: []decorator.Decorator{}}

func RegisterDecorator(f decorator.Decorator) {
	localRegisterer.decorators = append(localRegisterer.decorators, f)
}

func NewWithLogger(l logging.Logger, cfg lua.Config, next proxy.Proxy) proxy.Proxy {
	if l == nil {
		return new(logging.NoOp, cfg, next)
	}
	return new(l, cfg, next)
}

func new(l logging.Logger, cfg lua.Config, next proxy.Proxy) proxy.Proxy {
	return func(ctx context.Context, req *proxy.Request) (resp *proxy.Response, err error) {
		b := lua.NewBinderWrapper(binder.Options{
			SkipOpenLibs:        !cfg.AllowOpenLibs,
			IncludeGoStackTrace: true,
		})
		defer b.GetBinder().Close()

		decorator.RegisterErrors(b.GetBinder())
		decorator.RegisterNil(b.GetBinder())
		decorator.RegisterLuaTable(b.GetBinder())
		decorator.RegisterLuaList(b.GetBinder())
		decorator.RegisterHTTPRequest(ctx, b.GetBinder())

		decorator.RegisterBase64(b.GetBinder())
		decorator.RegisterHashing(b.GetBinder())
		decorator.RegisterJson(b.GetBinder())
		decorator.RegisterYaml(b.GetBinder())
		decorator.RegisterXml(b.GetBinder())
		decorator.RegisterCsv(b.GetBinder())
		decorator.RegisterTime(b.GetBinder())
		decorator.RegisterDebug(l, parseNativeUserData, b.GetBinder())
		for _, f := range localRegisterer.decorators {
			f(b.GetBinder())
		}

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

		if err := b.WithCode("post-script", cfg.PostCode); err != nil {
			return nil, err
		}

		return resp, nil
	}
}

// New does not have a logger, so debug calls with this proxy won't log anything, use NewWithLogger instead
func New(cfg lua.Config, next proxy.Proxy) proxy.Proxy {
	return new(logging.NoOp, cfg, next)
}

var (
	errNeedsArguments = errors.New("need arguments")
	errInvalidLuaList = errors.New("invalid header value, must be a luaList")
)
