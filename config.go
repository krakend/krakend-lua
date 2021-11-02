package lua

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"io"
	"io/ioutil"

	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/logging"
)

type Config struct {
	Sources       []string
	PreCode       string
	PostCode      string
	SkipNext      bool
	AllowOpenLibs bool
	SourceLoader  SourceLoader
}

func (c *Config) Get(k string) (string, bool) {
	return c.SourceLoader.Get(k)
}

type SourceLoader interface {
	Get(string) (string, bool)
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
	if b, ok := c["allow_open_libs"].(bool); ok && b {
		res.AllowOpenLibs = b
	}

	sources, ok := c["sources"].([]interface{})
	if ok {
		s := []string{}
		for _, source := range sources {
			if t, ok := source.(string); ok {
				s = append(s, t)
			}
		}
		res.Sources = s
	}

	if b, ok := c["live"].(bool); ok && b {
		res.SourceLoader = new(liveLoader)
		return res, nil
	}

	loader := map[string]string{}

	for _, source := range res.Sources {
		b, err := ioutil.ReadFile(source)
		if err != nil {
			l.Error("[Lua] Opening the source file", err.Error())
			continue
		}
		loader[source] = string(b)
	}
	res.SourceLoader = onceLoader(loader)

	checksums, ok := c["md5"].(map[string]interface{})
	if !ok {
		return res, nil
	}

	for source, c := range checksums {
		checksum, ok := c.(string)
		if !ok {
			return res, ErrWrongChecksumType(source)
		}
		content, _ := res.SourceLoader.Get(source)
		hash := md5.New()
		if _, err := io.Copy(hash, bytes.NewBuffer([]byte(content))); err != nil {
			return res, err
		}
		hashInBytes := hash.Sum(nil)[:16]
		if actual := hex.EncodeToString(hashInBytes); checksum != actual {
			return res, ErrWrongChecksum{
				Source:   source,
				Actual:   actual,
				Expected: checksum,
			}
		}
	}

	return res, nil
}

type onceLoader map[string]string

func (o onceLoader) Get(k string) (string, bool) {
	v, ok := o[k]
	return v, ok
}

type liveLoader struct{}

func (l *liveLoader) Get(k string) (string, bool) {
	b, err := ioutil.ReadFile(k)
	if err != nil {
		return "", false
	}
	return string(b), true
}

var (
	ErrNoExtraConfig    = errors.New("no extra config")
	ErrWrongExtraConfig = errors.New("wrong extra config")
)
