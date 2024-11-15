package lua

import (
	"testing"
)

var codeExamples = map[string]string{
	"single-line":     "line1",
	"inline":          "line1\nline2\nline3",
	"inline-trailing": "line1\nline2\nline3\n",
	"multi-line": `line1
		line2
		line3
		line4
		
		line6`,
}

type TestConfig struct {
	path string
	src  string
}
type TestCase struct {
	line                 int
	expectedPath         string
	expectedRelativeLine int
	expectError          bool
}

func TestAffectedSource(t *testing.T) {
	tests := map[string]struct {
		config []TestConfig
		cases  []TestCase
	}{
		"Empty case": {
			config: []TestConfig{},
			cases: []TestCase{
				{1, "", 0, true},
			},
		},
		"Simple case": {
			config: []TestConfig{
				{"test.src", codeExamples["single-line"]},
			},
			cases: []TestCase{
				{1, "test.src", 1, false},
			},
		},
		"Single inline": {
			config: []TestConfig{
				{"test1.src", codeExamples["inline"]},
			},
			cases: []TestCase{
				{1, "test1.src", 1, false},
				{3, "test1.src", 3, false},
			},
		},
		"Single inline trailing delimiter": {
			config: []TestConfig{
				{"test1.src", codeExamples["inline-trailing"]},
			},
			cases: []TestCase{
				{1, "test1.src", 1, false},
				{3, "test1.src", 3, false},
				{4, "", 0, true},
			},
		},
		"Multiple sources of single lines": {
			config: []TestConfig{
				{"test1.src", codeExamples["single-line"]},
				{"test2.src", codeExamples["single-line"]},
			},
			cases: []TestCase{
				{1, "test1.src", 1, false},
				{2, "test2.src", 1, false},
			},
		},
		"Multiple sources": {
			config: []TestConfig{
				{"test1.src", codeExamples["single-line"]},
				{"test2.src", codeExamples["single-line"]},
				{"test3.src", codeExamples["multi-line"]},
				{"test4.src", codeExamples["multi-line"]},
				{"test5.src", codeExamples["single-line"]},
			},
			cases: []TestCase{
				{1, "test1.src", 1, false},
				{2, "test2.src", 1, false},
				{3, "test3.src", 1, false},
				{5, "test3.src", 3, false},
				{10, "test4.src", 2, false},
				{14, "test4.src", 6, false},
				{15, "test5.src", 1, false},
				{16, "", 0, true},
			},
		},
		"Returns only the affected file name": {
			config: []TestConfig{
				{"test1.src", codeExamples["single-line"]},
				{"./test2.src", codeExamples["single-line"]},
				{"/path/test3.src", codeExamples["single-line"]},
				{"/path/path2/test4.src", codeExamples["single-line"]},
				{"../path2/test5.src", codeExamples["single-line"]},
			},
			cases: []TestCase{
				{1, "test1.src", 1, false},
				{2, "test2.src", 1, false},
				{3, "test3.src", 1, false},
				{4, "test4.src", 1, false},
				{5, "test5.src", 1, false},
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			sourceMap := NewSourceMap()
			for _, cfg := range test.config {
				sourceMap.Append(cfg.path, cfg.src)
			}

			for _, c := range test.cases {
				affectedPath, relativeLine, err := sourceMap.AffectedSource(c.line)
				if err != nil && !c.expectError {
					t.Errorf("case '%s' return an unexpected error", name)
				}
				if relativeLine != c.expectedRelativeLine {
					t.Errorf("unexpected relative line in '%s', expected=%d, got=%d", name, c.expectedRelativeLine, relativeLine)
				}
				if affectedPath != c.expectedPath {
					t.Errorf("unexpected path in '%s', expected=%s, got=%s", name, c.expectedPath, affectedPath)
				}
			}
		})
	}
}
