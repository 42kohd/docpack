package dialect

import (
	"bytes"
	"regexp"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// KindContainer is the goldmark node kind of a ::: container.
var KindContainer = ast.NewNodeKind("Container")

// Container is a "::: name argument" … ":::" block. A raw container (command)
// keeps its lines verbatim in Lines(); the others hold parsed children.
type Container struct {
	ast.BaseBlock
	Name   string
	Arg    string
	Offset int // byte offset of the opening line
	Closed bool

	raw     bool
	depth   int  // nested containers currently open inside this one
	inFence bool // inside a fenced code block in this container
}

// Kind implements ast.Node.
func (n *Container) Kind() ast.NodeKind { return KindContainer }

// Dump implements ast.Node.
func (n *Container) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{"Name": n.Name, "Arg": n.Arg}, nil)
}

// IsRaw implements ast.Node; raw containers are not parsed as Markdown.
func (n *Container) IsRaw() bool { return n.raw }

var containerOpen = regexp.MustCompile(`^:::[ \t]*([A-Za-z][A-Za-z0-9_-]*)[ \t]*(.*?)[ \t]*\r?\n?$`)

func isCloser(line []byte) bool {
	return string(bytes.TrimSpace(line)) == ":::"
}

func isFence(line []byte) bool {
	t := bytes.TrimLeft(line, " \t")
	return bytes.HasPrefix(t, []byte("```")) || bytes.HasPrefix(t, []byte("~~~"))
}

type containerParser struct{}

func (containerParser) Trigger() []byte { return []byte{':'} }

func (containerParser) Open(parent ast.Node, reader text.Reader, pc parser.Context) (ast.Node, parser.State) {
	line, seg := reader.PeekLine()
	pos := pc.BlockOffset()
	if pos < 0 {
		return nil, parser.NoChildren
	}
	m := containerOpen.FindSubmatch(line[pos:])
	if m == nil {
		return nil, parser.NoChildren
	}
	n := &Container{Name: string(m[1]), Arg: string(m[2]), Offset: seg.Start, raw: string(m[1]) == "command"}
	reader.AdvanceToEOL()
	if n.raw {
		return n, parser.NoChildren
	}
	return n, parser.HasChildren
}

// Continue decides whether the line belongs to this container. Because
// goldmark offers each line to the outermost open block first, a container
// counts the containers opened inside it so that their ":::" closes them,
// not this one, and it ignores ":::" inside fenced code.
func (containerParser) Continue(node ast.Node, reader text.Reader, pc parser.Context) parser.State {
	n := node.(*Container)
	line, seg := reader.PeekLine()
	if n.raw {
		if isCloser(line) {
			n.Closed = true
			reader.AdvanceToEOL()
			return parser.Close
		}
		node.Lines().Append(seg)
		reader.AdvanceToEOL()
		return parser.Continue | parser.NoChildren
	}
	if isFence(line) {
		n.inFence = !n.inFence
		return parser.Continue | parser.HasChildren
	}
	if n.inFence {
		return parser.Continue | parser.HasChildren
	}
	if isCloser(line) {
		if n.depth > 0 {
			n.depth--
			return parser.Continue | parser.HasChildren
		}
		n.Closed = true
		reader.AdvanceToEOL()
		return parser.Close
	}
	if containerOpen.Match(bytes.TrimLeft(line, " \t")) {
		n.depth++
	}
	return parser.Continue | parser.HasChildren
}

func (containerParser) Close(node ast.Node, reader text.Reader, pc parser.Context) {}

func (containerParser) CanInterruptParagraph() bool { return true }

func (containerParser) CanAcceptIndentedLine() bool { return false }

type containerExt struct{}

// Containers is the goldmark extension that parses ::: containers.
var Containers goldmark.Extender = containerExt{}

func (containerExt) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithBlockParsers(util.Prioritized(containerParser{}, 90)))
}
