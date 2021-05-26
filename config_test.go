package lua

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/luraproject/lura/config"
	"github.com/luraproject/lura/logging"
)

func TestParse(t *testing.T) {
	source := "lua/factorial.lua"
	key := "1234"
	in := config.ExtraConfig{
		key: map[string]interface{}{
			"sources": []interface{}{
				source,
			},
			"md5": map[string]interface{}{
				source: "49ae50f58e35f4821ad4550e1a4d1de0",
			},
			"pre":       "pre",
			"post":      "post",
			"skip_next": true,
		},
	}
	cfg, err := Parse(logging.NoOp, in, key)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}
	b, _ := ioutil.ReadFile(source)
	if src, ok := cfg.Get(source); !ok || src != string(b) {
		t.Errorf("wrong content %s", string(b))
	}
	if !cfg.SkipNext {
		t.Errorf("the skip next flag is not enabled")
	}
	if cfg.PreCode != "pre" {
		t.Errorf("wrong pre code %s", cfg.PreCode)
	}
	if cfg.PostCode != "post" {
		t.Errorf("wrong post code %s", cfg.PostCode)
	}
}

func TestParse_live(t *testing.T) {
	tmpfile, err := ioutil.TempFile("", "test_parse_live")
	if err != nil {
		t.Error(err)
		return
	}

	source := tmpfile.Name()

	defer os.Remove(source) // clean up

	initialContent := `print("hello, lua")`
	finalContent := `print("bye, lua")`

	if _, err := tmpfile.Write([]byte(initialContent)); err != nil {
		t.Error(err)
		return
	}

	tmpfile.Close()

	key := "1234"
	in := config.ExtraConfig{
		key: map[string]interface{}{
			"sources": []interface{}{
				source,
			},
			"live": true,
		},
	}
	cfg, err := Parse(logging.NoOp, in, key)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	if src, ok := cfg.Get(source); !ok || src != initialContent {
		t.Errorf("wrong content %s", src)
	}

	if err := ioutil.WriteFile(source, []byte(finalContent), 0644); err != nil {
		t.Error(err)
		return
	}

	if src, ok := cfg.Get(source); !ok || src != finalContent {
		t.Errorf("wrong content %s", src)
	}
}

func TestParse_noExtra(t *testing.T) {
	_, err := Parse(logging.NoOp, config.ExtraConfig{}, "1234")
	if err != ErrNoExtraConfig {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestParse_wrongExtra(t *testing.T) {
	key := "1234"
	in := config.ExtraConfig{
		key: 42,
	}
	_, err := Parse(logging.NoOp, in, key)
	if err != ErrWrongExtraConfig {
		t.Errorf("unexpected error: %v", err)
	}
}
