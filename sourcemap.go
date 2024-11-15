package lua

import (
	"errors"
	"path/filepath"
	"strings"
)

type SourceMap []struct {
	Lines int
	Path  string
}

func NewSourceMap() SourceMap {
	return SourceMap{}
}

func (s *SourceMap) Append(path string, src string) *SourceMap {
	src = strings.Trim(src, "\n")
	lines := strings.Count(src, "\n") + 1

	*s = append(*s, struct {
		Lines int
		Path  string
	}{lines, path})

	return s
}

func (s *SourceMap) AffectedSource(line int) (string, int, error) {
	count := 0
	for _, source := range *s {
		count += source.Lines
		if count >= line {
			relativeLine := source.Lines - (count - line)
			return filepath.Base(source.Path), relativeLine, nil
		}
	}
	return "", 0, errors.New("line number out of bounds")
}
