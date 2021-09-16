package indents

// NOTE
// A text editor might report this file as having a mixed-indent error.
// Please ignore, because text fixtures embedded in this file have tests
// for both tab and space indentation. Do not fix it.

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"testing/iotest"

	"gotest.tools/v3/assert"
)

func TestStyleLevel(t *testing.T) {
	tests := map[string]struct {
		level int
		text  string
	}{
		"0 level":    {0, "text"},
		"0 5 level":  {0, "-text"},
		"1 level":    {1, "--text"},
		"2 levels":   {2, "----text"},
		"3 levels":   {3, "------text"},
		"3 5 levels": {3, "-------text"},
		"4 levels":   {4, "--------text"},
		//                 1-2-3-4-
	}

	style := &Style{'-', 2}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, style.Level(tt.text), tt.level)
		})
	}
}

func TestStyleLevelOdd(t *testing.T) {
	tests := map[string]struct {
		level int
		text  string
	}{
		"0 level":    {0, "text"},
		"0.5 level":  {0, "--text"},
		"1 level":    {1, "---text"},
		"2 levels":   {2, "------text"},
		"3 levels":   {3, "---------text"},
		"3.5 levels": {3, "----------text"},
		"3.7 levels": {3, "-----------text"},
		"4 levels":   {4, "------------text"},
		//                 1--2--3--4--
	}

	style := &Style{'-', 3}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, style.Level(tt.text), tt.level)
		})
	}
}

func TestAutoDetect(t *testing.T) {
	tests := map[string]struct {
		line  string
		style *Style
	}{
		"no text":               {"", nil},
		"no indentation":        {"text       ", nil},
		"1 space only":          {" text      ", Spaces(1)},
		"1 space ignore other":  {" x text    ", Spaces(1)},
		"1 space ignore tab p":  {" \txtext   ", Spaces(1)},
		"1 space ignore tab s":  {" x\ttext   ", Spaces(1)},
		"2 spaces only":         {"  text     ", Spaces(2)},
		"2 spaces ignore other": {"  x text   ", Spaces(2)},
		"2 spaces ignore tab p": {"  \ttext   ", Spaces(2)},
		"2 spaces ignore tab s": {"  x\ttext  ", Spaces(2)},
		"1 tab only":            {"\ttext     ", Tabs(1)},
		"1 tab ignore other":    {"\tx\ttext  ", Tabs(1)},
		"1 tab ignore space p":  {"\t text    ", Tabs(1)},
		"1 tab ignore space s":  {"\tx text   ", Tabs(1)},
		"2 tabs only":           {"\t\ttext   ", Tabs(2)},
		"2 tabs ignore other":   {"\t\tx\ttext", Tabs(2)},
		"2 tabs ignore space":   {"\t\t text  ", Tabs(2)},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			style := AutoDetect(tt.line)

			if tt.style == nil {
				assert.Assert(t, style == nil)
			} else {
				assert.DeepEqual(t, *style, *tt.style)
			}
		})
	}
}

func TestStyleSpaces(t *testing.T) {
	for n := 0; n < 10; n++ {
		assert.DeepEqual(t, *Spaces(n), Style{' ', n})
		assert.DeepEqual(t, *Spaces(n), Style{Space, n})
	}
}

func TestStyleTabs(t *testing.T) {
	for n := 0; n < 10; n++ {
		assert.DeepEqual(t, *Tabs(n), Style{'\t', n})
		assert.DeepEqual(t, *Tabs(n), Style{Tab, n})
	}
}

/////////////////////////
// IndentScanner tests //
/////////////////////////

func parseLineLevel(number int, line string) *Line {
	var (
		level int64
		err   error
	)

	fields := strings.Fields(line)

	if len(fields) > 0 {
		level, err = strconv.ParseInt(fields[0], 10, 32)

		if err != nil {
			return nil
		}
	}

	return &Line{line, number, int(level)}
}

func makeIndentScanner(text string, style *Style) *IndentScanner {
	reader := strings.NewReader(strings.TrimSpace(text))
	return NewIndentScanner(reader, style)
}

// key   - the `tk` test key (see makeTestTexts)
// text  - indendeted text formatted properly (see makeTestTexts)
// count - expected line count
func runIndentScannerTest(
	t *testing.T,
	key tk,
	text string,
	count int,
) *IndentScanner {
	scanner := makeIndentScanner(text, key.style)

	var n int

	for n = 0; scanner.Scan(); n++ {
		line := scanner.Line()
		lnum := n + 1
		assert.Equal(t, scanner.lines, lnum)
		assert.DeepEqual(
			t,
			line,
			parseLineLevel(lnum, line.Text),
		)
	}

	assert.Equal(t, n, count)
	assert.Equal(t, scanner.Lines(), count)
	assert.DeepEqual(t, scanner.Style(), key.detect)

	return scanner
}

func TestIndentScanner(t *testing.T) {
	for k, text := range makeTestTexts() {
		t.Run(k.name, func(t *testing.T) {
			runIndentScannerTest(t, k, text, k.lines)
		})
	}
}

func TestIndentScannerErr(t *testing.T) {
	err := errors.New("test error")
	rdr := iotest.ErrReader(err)
	scr := NewIndentScanner(rdr, nil)

	for scr.Scan() {
		t.Error("scr.Scan() should not succeed")
	}

	assert.Equal(t, scr.Err(), err)
}

/////////////////////////
// ParseNodeTree tests //
/////////////////////////

func countNodes(root *Node) int {
	count := len(root.Children)
	for _, n := range root.Children {
		count = count + countNodes(n)
	}
	return count
}

func testNode(t *testing.T, node *Node, parent *Node) {
	// Test line only if not root
	if node.Level() >= 0 {
		num := node.Number()
		txt := node.Text()
		assert.DeepEqual(t, parseLineLevel(num, txt), node.Line)
	}

	assert.Equal(t, node.Parent, parent)

	for _, n := range node.Children {
		testNode(t, n, node)
	}
}

func testRootNode(t *testing.T, root *Node) {
	assert.Assert(t, root.Parent == nil)
	assert.Assert(t, root.Line == nil)
	assert.Equal(t, root.Text(), "")
	assert.Equal(t, root.Number(), -1)
	assert.Equal(t, root.Level(), -1)
}

func runTestParseNodeTree(
	text string,
	style *Style,
	options *ParseNodeTreeOptions,
) (*Node, error) {
	scan := makeIndentScanner(text, style)
	return ParseNodeTree(scan, nil, options)
}

func TestParseNodeTree(t *testing.T) {
	for k, text := range makeTestTexts() {
		if strings.HasSuffix(k.name, "(extra)") {
			t.Run(k.name+"(error)", func(t *testing.T) {
				_, err := runTestParseNodeTree(text, k.style, nil)
				assert.ErrorContains(t, err, "Extra indentation at line ")
			})

			t.Run(k.name+"(ignore)", func(t *testing.T) {
				options := &ParseNodeTreeOptions{
					IgnoreExtraIndentation: true,
				}
				root, err := runTestParseNodeTree(text, k.style, options)
				assert.NilError(t, err)
				testRootNode(t, root)
				testNode(t, root, nil)
			})
		} else {
			t.Run(k.name, func(t *testing.T) {
				root, err := runTestParseNodeTree(text, k.style, nil)
				assert.NilError(t, err)
				testRootNode(t, root)
				testNode(t, root, nil)
			})
		}
	}
}

func TestNodeProcessor(t *testing.T) {
	for k, text := range makeTestTexts() {
		if !strings.HasSuffix(k.name, "(extra)") {
			t.Run(k.name+"(called)", func(t *testing.T) {
				called := 0
				options := &ParseNodeTreeOptions{
					Processor: func(n *Node, opt *ParseNodeTreeOptions) error {
						called++
						return nil
					},
				}
				root, err := runTestParseNodeTree(text, k.style, options)
				assert.NilError(t, err)
				assert.Equal(t, called, countNodes(root))
			})

			t.Run(k.name+"(error)", func(t *testing.T) {
				testError := fmt.Errorf("test")
				options := &ParseNodeTreeOptions{
					Processor: func(n *Node, opt *ParseNodeTreeOptions) error {
						return testError
					},
				}
				_, err := runTestParseNodeTree(text, k.style, options)
				assert.Equal(t, err, testError)
			})
		}
	}
}

///////////////////
// Test fixtures //
///////////////////

type tk struct {
	name   string // name of the test
	lines  int    // expected lines (we hardcode it for simplicity sake)
	style  *Style // the indentation style to *set* (use nil to autodetect)
	detect *Style // the expected autodetected indentation style
}

// Make test fixtures for testing the scanner in normal mode
func makeTestTexts() map[tk]string {
	ts := Tabs(1)
	texts := make(map[tk]string)

	texts[tk{"no indentation", 3, ts, ts}] = `
0
0
0
`
	texts[tk{"1 level step", 3, ts, ts}] = `
0
	1
		2
		`
	texts[tk{"1 level steps (extra)", 3, ts, ts}] = `
0
	1
			3
		`
	texts[tk{"1 level step with empty lines", 5, ts, ts}] = `
0

	1

		2
		`
	texts[tk{"1 level step with empty lines (extra)", 5, ts, ts}] = `
0

		2

			3
		`
	texts[tk{"1 level multiline", 10, ts, ts}] = `
0
	1
	1
		2
		2
		2
			3
			3
			3
			3
			`
	texts[tk{"1 level multiline with empty lines", 14, ts, ts}] = `
0
	1

	1
		2
		2

		2
			3

			3
			3

			3
			`
	texts[tk{"1 level unindent", 5, ts, ts}] = `
0
	1
		2
	1
0
`
	texts[tk{"1 level unindent (extra)", 5, ts, ts}] = `
0
	1
			3
	1
0
`
	texts[tk{"1 level unindent with empty lines", 7, ts, ts}] = `
0
	1

		2
	1

0
`
	texts[tk{"1 level unindent with empty lines (extra)", 7, ts, ts}] = `
0
	1

			3
	1

0
`
	texts[tk{"2 level unindent", 6, ts, ts}] = `
0
	1
		2
			3
	1
0
`
	texts[tk{"2 level unindent with empty lines", 8, ts, ts}] = `
0
	1

		2
			3

	1
0
`
	texts[tk{"3 level unindent", 7, ts, ts}] = `
0
	1
		2
			3
				4
	1
0
`
	texts[tk{"3 level unindent with empty lines", 10, ts, ts}] = `
0
	1

		2
			3

				4

	1
0
`
	texts[tk{"1 level unindent multiline", 12, ts, ts}] = `
0
	1
	1
		2
		2
		2
	1
	1
	1
	1
0
0
`
	texts[tk{"1 level unindent multiline with empty lines", 17, ts, ts}] = `
0
	1

	1

		2
		2

		2
	1
	1

	1

	1
0
0
`
	texts[tk{"2 level unindent multiline", 13, ts, ts}] = `
0
	1
	1
		2
		2
		2
			3
			3
			3
			3
	1
	1
0
`
	texts[tk{"2 level unindent multiline with empty lines", 19, ts, ts}] = `
0
	1

	1

		2
		2

		2
			3
			3

			3

			3
	1

	1
0
`
	texts[tk{"3 level unindent multiline", 17, ts, ts}] = `
0
	1
	1
		2
		2
		2
			3
			3
			3
			3
				4
				4
				4
	1
	1
	1
0
`
	texts[tk{"3 level unindent multiline with empty lines", 26, ts, ts}] = `
0
	1

	1
		2

		2
		2

			3
			3

			3

			3
				4

				4

				4

	1

	1
	1
0
`
	texts[tk{"autodetect 2 spaces", 3, nil, Spaces(2)}] = `
0
  1
    2
`
	texts[tk{"autodetect 2 spaces mix", 3, nil, Spaces(2)}] = `
0
  1
    	2
`
	texts[tk{"autodetect 4 spaces", 3, nil, Spaces(4)}] = `
0
    1
        2
`
	texts[tk{"autodetect 1 tab", 3, nil, Tabs(1)}] = `
0
	1
		2
`
	texts[tk{"autodetect none", 3, nil, nil}] = `
0
0
0
`
	// Text below have mixed tab/space indentation
	texts[tk{"autodetect 4 spaces mix", 3, nil, Spaces(4)}] = `
0
    	1 // this line is prefixed with 4 spaces and 1 tab
        2
`

	return texts
}
