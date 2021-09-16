// Package indents provides functions for parsing text with indentation.
package indents

import (
	"bufio"
	"fmt"
	"io"
)

const (
	Space = ' '
	Tab   = '\t'
)

///////////
// Style //
///////////

// Encapsulates an indentation style.
type Style struct {
	Char rune // The indent character.
	Size int  // Number of subsequent Char's considered as 1 indent.
}

// Create a new Style by auto-detecting indentation of given line.
// It is assumed that the given line has (potentially) a first-level
// indentation. If there are no indentation in the line, this function
// will return nil.
func AutoDetect(line string) *Style {
	var char rune = 0
	var size int = 0

	for n, c := range line {
		if char > 0 && c != char {
			break
		}

		if c != Space && c != Tab {
			break
		}

		if char == 0 {
			char = c
		}

		size = n + 1
	}

	if char == 0 || size == 0 {
		return nil
	} else {
		return &Style{char, size}
	}
}

// Shortcut. Spaces(2) -> &Style{Space, 2}
func Spaces(size int) *Style {
	return &Style{Space, size}
}

// Shortcut. Tabs(1) -> &Style{Tab, 1}
func Tabs(size int) *Style {
	return &Style{Tab, size}
}

// Calculate the given line's indentation level based on this indent style.
func (s *Style) Level(line string) int {
	level := 0

	for n, c := range line {
		if c != s.Char {
			break
		} else {
			level = (n + 1) / s.Size
		}
	}

	return level
}

///////////////////
// IndentScanner //
///////////////////

// Encapsulates an indented line.
type Line struct {
	Text   string // The line text without indentation.
	Number int    // The line number.
	Level  int    // The indentation level.
}

// Provides a bufio.Scanner-like interface for reading indented-text.
// Successive calls to the Scan method will step through the lines of
// a file. Call the Line method to get the current *Line struct, with
// the indentation level calculated and set.
//
// The IndentScanner is indentation-"dumb", it merely detects the
// indentation size of each line and sets it on the *Line struct.
// It's up to the caller to assert any indentation-aware logic.
//
// See the ParseNodeTree function, where Lines produced by this
// scanner are parsed in an indentation-aware manner and converted
// into a tree structure.
type IndentScanner struct {
	scanner *bufio.Scanner
	style   *Style
	lines   int
}

// Crete a new IndentScanner that reads data from Reader r.
// The style argument sets the assumed indentation style of the data.
// If it's nil, the scanner will try to auto-detect the style
// using the AutoDetect function.
func NewIndentScanner(r io.Reader, style *Style) *IndentScanner {
	return &IndentScanner{bufio.NewScanner(r), style, 0}
}

// Returns the first non-EOF error that was encountered by the Scanner.
func (s *IndentScanner) Err() error {
	return s.scanner.Err()
}

// Advance the scanner to the next line, which will then be available
// through the Line method.  It returns false when the scan stops,
// either by reaching the end of the input or an error. After Scan
// returns false, the Err method will return any error that occurred
// during scanning, except that if it was io.EOF, Err will return nil.
func (s *IndentScanner) Scan() bool {
	ok := s.scanner.Scan()
	if ok {
		s.lines++
	}
	return ok
}

// Returns the most recent line generated by a call to Scan as a
// newly allocated Line struct.
func (s *IndentScanner) Line() *Line {
	text := s.scanner.Text()
	level := 0
	index := 0

	if s.style == nil {
		s.style = AutoDetect(text)
	}

	if s.style != nil {
		level = s.style.Level(text)
		index = level * s.style.Size
	}

	return &Line{
		Text:   text[index:],
		Number: s.lines,
		Level:  level,
	}
}

// Returns the number of lines read.
// Will return 0 if the Scan method was never called before.
func (s *IndentScanner) Lines() int {
	return s.lines
}

// Returns the autodetected indent style.
// Will return nil if no indetation detected, or the Line method was never
// called before.
func (s *IndentScanner) Style() *Style {
	return s.style
}

///////////////////
// ParseNodeTree //
///////////////////

type ExtraIndentationError struct {
	Line int // Line number where the error is found.
}

func (e *ExtraIndentationError) Error() string {
	return fmt.Sprintf("Extra indentation at line %d", e.Line)
}

// Encapsulates a node in a tree.
type Node struct {
	Line     *Line   // The corresponding Line
	Parent   *Node   // This node's parent node
	Children []*Node // This node's child nodes
}

func (n *Node) Level() int {
	if n.Line == nil {
		return -1
	} else {
		return n.Line.Level
	}
}

func (n *Node) Number() int {
	if n.Line == nil {
		return -1
	} else {
		return n.Line.Number
	}
}

func (n *Node) Text() string {
	if n.Line == nil {
		return ""
	} else {
		return n.Line.Text
	}
}

type NodeProcessor func(node *Node, options *ParseNodeTreeOptions) error

// Options to pass to ParseNodeTree function to customize its behaviour.
type ParseNodeTreeOptions struct {
	// If true, extra indentations will be ignored.
	IgnoreExtraIndentation bool

	// If set, this function will be called on each Node generated.
	Processor NodeProcessor
}

// Read lines from an IndentScanner and produce a node tree sructure.
//
// Behaviour of the parser can be customized with the options argument.
// See ParseNodeTreeOptions.
func ParseNodeTree(
	scanner *IndentScanner,
	root *Node,
	options *ParseNodeTreeOptions,
) (*Node, error) {
	if root == nil {
		root = &Node{}
	}

	if options == nil {
		options = &ParseNodeTreeOptions{false, nil}
	}

	root.Line = nil
	root.Parent = nil

	bloc := root
	prev := root

	for scanner.Scan() {
		line := scanner.Line()

		// Ignore empty lines
		if len(line.Text) == 0 {
			continue
		}

		switch {
		case line.Level == prev.Level():
			// Same level
			// noop
		case line.Level == prev.Level()+1:
			// +1 indent
			bloc = prev
		case line.Level < prev.Level():
			// -N indent
			for {
				bloc = bloc.Parent
				if bloc.Level() == line.Level-1 {
					break
				}
			}
		default:
			// +N indent
			if !options.IgnoreExtraIndentation {
				// error: extra indentation
				return root, &ExtraIndentationError{line.Number}
			} else {
				// ignore error: treat as +1 indent
				bloc = prev
			}
		}

		node := &Node{line, bloc, make([]*Node, 0)}
		bloc.Children = append(bloc.Children, node)
		prev = node

		if options.Processor != nil {
			if err := options.Processor(node, options); err != nil {
				return root, err
			}
		}
	}

	return root, nil
}
