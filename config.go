package lua

import (
	"errors"
	"io/ioutil"

	"github.com/devopsfaith/krakend/config"
	"github.com/devopsfaith/krakend/logging"
)

type Config struct {
	Sources  []string
	PreCode  string
	PostCode string
	SkipNext bool
}

func Parse(l logging.Logger, e config.ExtraConfig, namespace string) (Config, error) {
	res := Config{}
	v, ok := e[namespace]
	if !ok {
		return res, ErrNoExtraConfig
	}
	c, ok := v.(map[string]interface{})
	if !ok {
		return res, ErrWrongExtraConfig
	}
	if pre, ok := c["pre"].(string); ok {
		res.PreCode = pre
	}
	if post, ok := c["post"].(string); ok {
		res.PostCode = post
	}
	if b, ok := c["skip_next"].(bool); ok && b {
		res.SkipNext = b
	}
	sources, ok := c["sources"].([]interface{})
	if ok {
		s := []string{}
		for _, source := range sources {
			if t, ok := source.(string); ok {
				b, err := ioutil.ReadFile(t)
				if err != nil {
					l.Error("lua:", err)
					continue
				}
				s = append(s, string(b))
			}
		}
		res.Sources = s
	}
	return res, nil
}

var (
	ErrNoExtraConfig    = errors.New("no extra config")
	ErrWrongExtraConfig = errors.New("wrong extra config")
)
