// Package dialect parses documents written in the docpack Markdown dialect
// into a tree of typed blocks, reporting every construct the dialect does not
// allow (SPEC part A).
package dialect

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"

	"github.com/42kohd/docpack/internal/report"
)

// Document is one parsed .md file.
type Document struct {
	Path     string // project-relative, for example "docs/install.md"
	Name     string // filename without .md
	Front    Frontmatter
	Body     []*Block // top-level section blocks
	Source   []byte   // the file, CRLF normalised to LF
	BodyLine int      // first line after the frontmatter
	Diags    report.List

	sections map[string]bool
}

// HasSection reports whether the document has a section with this id.
func (d *Document) HasSection(id string) bool { return d.sections[id] }

// SectionIDs returns every section id in document order.
func (d *Document) SectionIDs() []string {
	var ids []string
	Walk(d.Body, func(b *Block) {
		if b.Type == TSection {
			ids = append(ids, b.ID)
		}
	})
	return ids
}

// Reindex recomputes the section index after the body has been changed.
func (d *Document) Reindex() {
	d.sections = map[string]bool{}
	for _, id := range d.SectionIDs() {
		d.sections[id] = true
	}
}

var md = goldmark.New(goldmark.WithExtensions(extension.Table, extension.DefinitionList, Containers))

// Parse parses one document. path is project-relative; diagnostics are
// collected on the returned document rather than returned as an error.
func Parse(path string, src []byte) *Document {
	src = bytes.ReplaceAll(src, []byte("\r\n"), []byte("\n"))
	src = bytes.TrimPrefix(src, []byte("\xef\xbb\xbf"))
	name := strings.TrimSuffix(filepath.Base(path), ".md")
	d := &Document{Path: path, Name: name, Source: src, BodyLine: 1}

	body := append([]byte(nil), src...)
	if fm, end, ok := splitFrontmatter(src); ok {
		d.Front = parseFrontmatter(path, name, fm, 1, &d.Diags)
		for i := 0; i < end; i++ {
			if body[i] != '\n' {
				body[i] = ' '
			}
		}
		d.BodyLine = bytes.Count(src[:end], []byte("\n")) + 1
	} else {
		d.Front = Frontmatter{lines: map[string]int{}}
		d.Diags.Add("fm-parse", path, 1, "The file does not start with --- frontmatter",
			"Start the file with a --- line, then type, slug, title and summary, then another --- line")
	}

	c := &conv{doc: d, src: body, ids: map[string]int{}}
	c.index()
	c.scanCharacters()
	root := md.Parser().Parse(text.NewReader(body))
	c.top(root)
	d.Reindex()
	return d
}

// splitFrontmatter returns the YAML between a leading --- line and the next
// --- line, and the byte offset just past the closing line.
func splitFrontmatter(src []byte) (fm []byte, end int, ok bool) {
	if !bytes.HasPrefix(src, []byte("---\n")) {
		return nil, 0, false
	}
	pos := 4
	for pos <= len(src) {
		nl := bytes.IndexByte(src[pos:], '\n')
		var line []byte
		next := len(src)
		if nl >= 0 {
			line = src[pos : pos+nl]
			next = pos + nl + 1
		} else {
			line = src[pos:]
		}
		if t := strings.TrimRight(string(line), " \t"); t == "---" || t == "..." {
			return src[4:pos], next, true
		}
		if nl < 0 {
			break
		}
		pos = next
	}
	return nil, 0, false
}

type ctx int

const (
	inSection ctx = iota
	inStep
	inCallout
)

type conv struct {
	doc   *Document
	src   []byte
	lines []int // byte offset of each line start
	ids   map[string]int
	last  int // last line resolved, a fallback for nodes without segments
}

func (c *conv) index() {
	c.lines = []int{0}
	for i, b := range c.src {
		if b == '\n' {
			c.lines = append(c.lines, i+1)
		}
	}
}

// line converts a byte offset to a 1-based line number.
func (c *conv) line(off int) int {
	l := sort.Search(len(c.lines), func(i int) bool { return c.lines[i] > off })
	c.last = l
	return l
}

// lineText returns the source text of a 1-based line, without the newline.
func (c *conv) lineText(l int) string {
	if l < 1 || l > len(c.lines) {
		return ""
	}
	start := c.lines[l-1]
	end := len(c.src)
	if l < len(c.lines) {
		end = c.lines[l] - 1
	}
	return string(c.src[start:end])
}

// nodeLine finds the first source line of a node.
func (c *conv) nodeLine(n ast.Node) int {
	switch n := n.(type) {
	case *Container:
		return c.line(n.Offset)
	case *ast.Text:
		return c.line(n.Segment.Start)
	case *ast.FencedCodeBlock:
		if n.Info != nil {
			return c.line(n.Info.Segment.Start)
		}
		if n.Lines().Len() > 0 {
			return c.line(n.Lines().At(0).Start) - 1
		}
	}
	if n.Type() == ast.TypeBlock && n.Lines().Len() > 0 {
		return c.line(n.Lines().At(0).Start)
	}
	for ch := n.FirstChild(); ch != nil; ch = ch.NextSibling() {
		if l := c.nodeLine(ch); l > 0 {
			return l
		}
	}
	return c.last
}

func (c *conv) add(rule string, line int, msg, fix string) {
	c.doc.Diags.Add(rule, c.doc.Path, line, msg, fix)
}

// scanCharacters reports emoji and typed bullet glyphs anywhere in the body.
func (c *conv) scanCharacters() {
	for l := c.doc.BodyLine; l <= len(c.lines); l++ {
		s := c.lineText(l)
		for _, r := range s {
			switch {
			case r == '•' || r == '◦' || r == '▪' || r == '‣':
				c.add("bullet-glyph", l, fmt.Sprintf("Typed bullet character %q", r),
					`Start the line with "- " to make a list item`)
			case isEmoji(r):
				c.add("emoji", l, fmt.Sprintf("Emoji %q", r), "Remove it; say the same thing in words")
			default:
				continue
			}
			break
		}
	}
}

func isEmoji(r rune) bool {
	switch {
	case r >= 0x1F000 && r <= 0x1FAFF, // pictographs, emoticons, transport, symbols
		r >= 0x2600 && r <= 0x27BF, // miscellaneous symbols, dingbats
		r >= 0x2B00 && r <= 0x2BFF, // arrows and stars used as emoji
		r == 0xFE0F, r == 0x200D,   // variation selector, zero-width joiner
		r >= 0x1F1E6 && r <= 0x1F1FF: // regional indicators
		return true
	}
	return false
}

// top walks the document root and builds the section tree.
func (c *conv) top(root ast.Node) {
	var stack []*Block
	reportedPreamble := false
	for n := root.FirstChild(); n != nil; n = n.NextSibling() {
		if h, ok := n.(*ast.Heading); ok {
			c.heading(h, &stack)
			continue
		}
		blocks := c.block(n, inSection)
		if len(blocks) == 0 {
			continue
		}
		if len(stack) == 0 {
			if !reportedPreamble {
				c.add("content-before-section", blocks[0].Line, "Content appears before the first ## section",
					"Move it into a section, or put a one-sentence lead in the summary frontmatter key")
				reportedPreamble = true
			}
			continue
		}
		top := stack[len(stack)-1]
		top.Body = append(top.Body, blocks...)
	}
}

func (c *conv) heading(h *ast.Heading, stack *[]*Block) {
	line := c.nodeLine(h)
	if !strings.HasPrefix(strings.TrimSpace(c.lineText(line)), "#") {
		c.add("block-forbidden", line, "Underlined (setext) heading or a --- line under text",
			"Write headings with ## at the start of the line, and remove --- lines from the body")
		return
	}
	if h.Level == 1 {
		c.add("h1-forbidden", line, "A # heading in the body",
			"Use ## for top-level sections; the page title comes from the title frontmatter key")
		return
	}
	if h.Level > 5 {
		c.add("heading-depth", line, fmt.Sprintf("Heading level %d is deeper than #####", h.Level),
			"Use ##### or shallower, or turn the content into a list")
		return
	}
	title := c.headingTitle(h, line)
	level := h.Level
	for len(*stack) > 0 && (*stack)[len(*stack)-1].Level >= level {
		*stack = (*stack)[:len(*stack)-1]
	}
	want := 2
	if len(*stack) > 0 {
		want = (*stack)[len(*stack)-1].Level + 1
	}
	if level != want {
		c.add("heading-skip", line, fmt.Sprintf("Heading %q skips a level", title),
			fmt.Sprintf("Make it %s (one level below its parent)", strings.Repeat("#", want)))
		level = want
	}
	sec := &Block{Type: TSection, ID: c.uniqueID(title), Title: title, Level: level, Line: line}
	if len(*stack) == 0 {
		c.doc.Body = append(c.doc.Body, sec)
	} else {
		parent := (*stack)[len(*stack)-1]
		parent.Body = append(parent.Body, sec)
	}
	*stack = append(*stack, sec)
}

// headingTitle returns the plain text of a heading. Code spans are kept as
// text; links, images and emphasis are reported.
func (c *conv) headingTitle(h ast.Node, line int) string {
	var sb strings.Builder
	reported := false
	var walk func(ast.Node)
	walk = func(n ast.Node) {
		for ch := n.FirstChild(); ch != nil; ch = ch.NextSibling() {
			switch ch := ch.(type) {
			case *ast.Text:
				sb.WriteString(unescape(ch.Segment.Value(c.src)))
				if ch.SoftLineBreak() {
					sb.WriteByte(' ')
				}
			case *ast.String:
				sb.Write(ch.Value)
			case *ast.CodeSpan:
				sb.WriteString(c.codeText(ch))
			default:
				if !reported {
					c.add("heading-inline", line, "A heading contains a link, image or emphasis",
						"Make the heading plain text; put links in the paragraph below it")
					reported = true
				}
				walk(ch)
			}
		}
	}
	walk(h)
	return strings.TrimSpace(sb.String())
}

var apostrophes = strings.NewReplacer("'", "", "’", "")

// SectionID derives a section id from a title (SPEC A3).
func SectionID(title string) string {
	s := strings.ToLower(apostrophes.Replace(title))
	var sb strings.Builder
	dash := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if dash && sb.Len() > 0 {
				sb.WriteByte('-')
			}
			dash = false
			sb.WriteRune(r)
		} else {
			dash = true
		}
	}
	if sb.Len() == 0 {
		return "section"
	}
	return sb.String()
}

func (c *conv) uniqueID(title string) string {
	id := SectionID(title)
	c.ids[id]++
	if n := c.ids[id]; n > 1 {
		return fmt.Sprintf("%s-%d", id, n)
	}
	return id
}

// block converts one goldmark block node. It returns nil when the node is
// reported as an error.
func (c *conv) block(n ast.Node, where ctx) []*Block {
	line := c.nodeLine(n)
	one := func(b *Block) []*Block {
		if b == nil {
			return nil
		}
		return []*Block{b}
	}
	switch n := n.(type) {
	case *ast.Paragraph:
		return one(c.paragraph(n, line))
	case *ast.TextBlock:
		return one(c.paragraph(n, line))
	case *ast.List:
		return one(c.list(n, line))
	case *ast.FencedCodeBlock:
		return one(c.code(n, line))
	case *east.Table:
		return one(c.table(n, line))
	case *east.DefinitionList:
		return one(c.deflist(n, line))
	case *Container:
		return one(c.container(n, line, where))
	case *ast.CodeBlock:
		c.add("block-forbidden", line, "Indented code block",
			"Use a fenced code block with a language, for example ```bash")
	case *ast.Blockquote:
		c.add("block-forbidden", line, "Blockquote",
			"Use a callout instead: ::: note Title, the text, then :::")
	case *ast.ThematicBreak:
		c.add("block-forbidden", line, "Horizontal rule",
			"Remove it; sections already separate content")
	case *ast.HTMLBlock:
		c.add("html-forbidden", line, "Raw HTML block",
			"Remove the HTML and use the Markdown constructs listed in the kit")
	case *ast.Heading:
		c.add("block-forbidden", line, "A heading inside a container",
			"Move the heading out of the container, or use ::: steps where each ### starts a step")
	default:
		c.add("block-forbidden", line, fmt.Sprintf("Unsupported construct (%s)", kindName(n)),
			"Rewrite it with the constructs listed in the kit")
	}
	return nil
}

var (
	strayCloser = regexp.MustCompile(`(?m)^[ \t]*:::`)
	refExpr     = regexp.MustCompile(`^\{\{\s*([a-z_]+)((?:\s+[a-z_]+=(?:"[^"]*"|[0-9]+))*)\s*\}\}$`)
	refLoose    = regexp.MustCompile(`^\{\{\s*([A-Za-z_]+)(.*)\}\}$`)
	refArg      = regexp.MustCompile(`([a-z_]+)=(?:"([^"]*)"|([0-9]+))`)
)

// rawText returns the source text of a block's lines, trimmed.
func (c *conv) rawText(n ast.Node) string {
	var sb strings.Builder
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		sb.Write(seg.Value(c.src))
	}
	return strings.TrimSpace(sb.String())
}

func (c *conv) paragraph(n ast.Node, line int) *Block {
	raw := c.rawText(n)
	if strayCloser.MatchString(raw) {
		c.add("container-stray", line, "A ::: line with no open container",
			"Remove the line, or open a container above it, for example ::: note Title")
		return nil
	}
	if strings.Contains(raw, "{{") || strings.Contains(raw, "}}") {
		return c.reference(raw, line)
	}
	if img := c.soleImage(n); img != nil {
		return c.image(img, line)
	}
	runs := c.runs(n)
	if len(runs) == 0 {
		return nil
	}
	return &Block{Type: TParagraph, Text: runs, Line: line}
}

func (c *conv) soleImage(n ast.Node) *ast.Image {
	var img *ast.Image
	for ch := n.FirstChild(); ch != nil; ch = ch.NextSibling() {
		switch ch := ch.(type) {
		case *ast.Image:
			if img != nil {
				return nil
			}
			img = ch
		case *ast.Text:
			if len(bytes.TrimSpace(ch.Segment.Value(c.src))) > 0 {
				return nil
			}
		default:
			return nil
		}
	}
	return img
}

func (c *conv) image(img *ast.Image, line int) *Block {
	var alt strings.Builder
	for ch := img.FirstChild(); ch != nil; ch = ch.NextSibling() {
		if t, ok := ch.(*ast.Text); ok {
			alt.WriteString(unescape(t.Segment.Value(c.src)))
		}
	}
	b := &Block{Type: TImage, Src: string(img.Destination), Alt: strings.TrimSpace(alt.String()),
		Caption: strings.TrimSpace(string(img.Title)), Line: line}
	if b.Alt == "" {
		c.add("image-alt", line, fmt.Sprintf("Image %s has no alt text", b.Src),
			"Describe the image between the brackets: ![What the image shows](path)")
	}
	if !strings.HasPrefix(b.Src, "assets/") || strings.Contains(b.Src, "..") {
		c.add("image-path", line, fmt.Sprintf("Image path %q is not under assets/", b.Src),
			"Put the file in the project's assets/ directory and write the path as assets/<file>")
		return nil
	}
	return b
}

// refSources are the expressions an author may write, with their arguments:
// true means required.
var refSources = map[string]map[string]bool{
	"flags":     {"group": false},
	"fields":    {"dataset": true},
	"platforms": {},
	"datasets":  {},
	"releases":  {"limit": false},
}

func (c *conv) reference(raw string, line int) *Block {
	m := refExpr.FindStringSubmatch(raw)
	if m == nil {
		if lm := refLoose.FindStringSubmatch(raw); lm != nil && !strings.Contains(raw, "\n") {
			if _, ok := refSources[lm[1]]; !ok {
				c.unknownRef(lm[1], line)
				return nil
			}
			c.add("template-arg", line, fmt.Sprintf("Malformed arguments in %s", raw),
				`Write arguments as name="value" (quoted text) or limit=N (a number), for example {{ flags group="core" }}`)
			return nil
		}
		c.add("template-inline", line, "{{ … }} inside a paragraph",
			`Put the reference expression alone in its own paragraph, for example {{ flags group="core" }}`)
		return nil
	}
	name := m[1]
	allowed, ok := refSources[name]
	if !ok {
		c.unknownRef(name, line)
		return nil
	}
	b := &Block{Type: TReferenceTable, Source: name, Line: line}
	seen := map[string]bool{}
	for _, a := range refArg.FindAllStringSubmatch(m[2], -1) {
		key, str, num := a[1], a[2], a[3]
		if _, ok := allowed[key]; !ok {
			c.add("template-arg", line, fmt.Sprintf("{{ %s }} takes no argument %q", name, key), argHelp(name))
			return nil
		}
		seen[key] = true
		if key == "limit" {
			if num == "" {
				c.add("template-arg", line, "limit must be an unquoted number", "Write limit=3")
				return nil
			}
			fmt.Sscan(num, &b.Limit)
			if b.Limit < 1 {
				c.add("template-arg", line, "limit must be at least 1", "Write limit=3")
				return nil
			}
			continue
		}
		if num != "" || str == "" {
			c.add("template-arg", line, fmt.Sprintf("%s must be a quoted, non-empty value", key), argHelp(name))
			return nil
		}
		if b.Filter == nil {
			b.Filter = map[string]string{}
		}
		b.Filter[key] = str
	}
	for key, required := range allowed {
		if required && !seen[key] {
			c.add("template-arg", line, fmt.Sprintf("{{ %s }} needs %s", name, key), argHelp(name))
			return nil
		}
	}
	return b
}

func (c *conv) unknownRef(name string, line int) {
	fix := `Use one of {{ flags group="…" }}, {{ fields dataset="…" }}, {{ platforms }}, {{ datasets }}, {{ releases limit=N }}`
	if name == "panels" || name == "dashboard_fields" {
		fix = "Remove it; dashboard panels and fields are generated from data/dashboards/"
	}
	c.add("template-unknown", line, fmt.Sprintf("Unknown reference expression {{ %s }}", name), fix)
}

func argHelp(name string) string {
	switch name {
	case "flags":
		return `Write {{ flags }} or {{ flags group="<group id from data/flags.yaml>" }}`
	case "fields":
		return `Write {{ fields dataset="<dataset id from data/datasets.yaml>" }}`
	case "releases":
		return "Write {{ releases }} or {{ releases limit=3 }}"
	}
	return fmt.Sprintf("Write {{ %s }} with no arguments", name)
}

var codeLangRe = regexp.MustCompile(`^[a-z0-9_+-]+$`)
var codeTitleRe = regexp.MustCompile(`^title="([^"]+)"$`)

func (c *conv) code(n *ast.FencedCodeBlock, line int) *Block {
	b := &Block{Type: TCode, Line: line}
	var sb strings.Builder
	for i := 0; i < n.Lines().Len(); i++ {
		seg := n.Lines().At(i)
		sb.Write(seg.Value(c.src))
	}
	b.Code = strings.TrimRight(sb.String(), "\n")
	info := ""
	if n.Info != nil {
		info = strings.TrimSpace(string(n.Info.Segment.Value(c.src)))
	}
	if info == "" {
		c.add("code-language", line, "Code block has no language",
			"Add a language after the opening fence, for example ```bash, ```yaml or ```text")
		return nil
	}
	lang, rest, _ := strings.Cut(info, " ")
	if !codeLangRe.MatchString(lang) {
		c.add("code-language", line, fmt.Sprintf("Code block language %q is not lowercase letters, digits, _, + or -", lang),
			"Use a plain language name, for example ```bash")
		return nil
	}
	b.Language = lang
	rest = strings.TrimSpace(rest)
	if rest != "" {
		m := codeTitleRe.FindStringSubmatch(rest)
		if m == nil {
			c.add("code-info", line, fmt.Sprintf("Unsupported code fence attributes %q", rest),
				"Keep only the language and an optional title=\"…\", for example ```yaml title=\"agent.yaml\"")
			return nil
		}
		b.Title = m[1]
	}
	return b
}

func (c *conv) container(n *Container, line int, where ctx) *Block {
	if !n.Closed {
		c.add("container-unclosed", line, fmt.Sprintf("::: %s is never closed, so it runs to the end of the file", n.Name),
			"Add a line containing only ::: where the container ends")
		return nil
	}
	nesting := func(what string) *Block {
		c.add("container-nesting", line, fmt.Sprintf("::: %s cannot appear inside %s", n.Name, what),
			"Move it out to the section level")
		return nil
	}
	switch n.Name {
	case "steps":
		if where == inStep {
			return nesting("a step")
		}
		if where == inCallout {
			return nesting("a callout")
		}
		return c.steps(n, line)
	case "command":
		return c.command(n, line)
	case "note", "tip", "warning", "important":
		if where == inCallout {
			return nesting("a callout")
		}
		if strings.TrimSpace(n.Arg) == "" {
			c.add("callout-title", line, fmt.Sprintf("::: %s has no title", n.Name),
				fmt.Sprintf("Add a short title on the same line: ::: %s Before you start", n.Name))
		}
		b := &Block{Type: TCallout, Kind: n.Name, Title: strings.TrimSpace(n.Arg), Line: line}
		for ch := n.FirstChild(); ch != nil; ch = ch.NextSibling() {
			b.Body = append(b.Body, c.block(ch, inCallout)...)
		}
		return b
	case "cards":
		if where != inSection {
			return nesting("a step or callout")
		}
		return c.cards(n, line)
	}
	if strings.EqualFold(n.Name, "callout") {
		c.add("container-unknown", line, "::: callout is not a container name",
			"Use the callout kind as the name: ::: note, ::: tip, ::: warning or ::: important, followed by a title")
		return nil
	}
	c.add("container-unknown", line, fmt.Sprintf("Unknown container ::: %s", n.Name),
		"Use one of steps, command, note, tip, warning, important or cards")
	return nil
}

var numberedTitle = regexp.MustCompile(`^(step\s+)?[0-9]+[.):]?\s`)

func (c *conv) steps(n *Container, line int) *Block {
	b := &Block{Type: TSteps, Line: line}
	if strings.TrimSpace(n.Arg) != "" {
		c.add("steps-structure", line, "::: steps takes no title",
			"Write ::: steps alone on the line; put any introduction in the paragraph above it")
	}
	var cur *Step
	reported := false
	for ch := n.FirstChild(); ch != nil; ch = ch.NextSibling() {
		if h, ok := ch.(*ast.Heading); ok {
			hl := c.nodeLine(h)
			title := c.headingTitle(h, hl)
			if h.Level < 3 {
				c.add("steps-structure", hl, fmt.Sprintf("Step %q uses a ## heading", title),
					"Start each step with ###")
			}
			if numberedTitle.MatchString(strings.ToLower(title)) {
				c.add("steps-structure", hl, fmt.Sprintf("Step title %q starts with a number", title),
					"Remove the number; steps are numbered when rendered")
			}
			cur = &Step{Title: title, Line: hl}
			b.Steps = append(b.Steps, cur)
			continue
		}
		blocks := c.block(ch, inStep)
		if cur == nil {
			if !reported && len(blocks) > 0 {
				c.add("steps-structure", blocks[0].Line, "Content inside ::: steps before the first step heading",
					"Start the steps block with ### <first step title>, or move the content above ::: steps")
				reported = true
			}
			continue
		}
		cur.Body = append(cur.Body, blocks...)
	}
	if len(b.Steps) == 0 {
		c.add("steps-structure", line, "::: steps holds no steps",
			"Add one ### heading per step inside the container")
		return nil
	}
	return b
}

func (c *conv) command(n *Container, line int) *Block {
	b := &Block{Type: TCommand, Caption: strings.TrimSpace(n.Arg), Variants: map[string]Variant{}, Line: line}
	if b.Caption == "" {
		c.add("cmd-caption", line, "::: command has no caption",
			"Add a caption on the same line saying what the command does: ::: command Start the agent")
	}
	for i := 0; i < n.Lines().Len(); i++ {
		seg := n.Lines().At(i)
		l := c.line(seg.Start)
		s := strings.TrimSpace(string(seg.Value(c.src)))
		if s == "" {
			continue
		}
		key, val, ok := strings.Cut(s, ":")
		key = strings.TrimSpace(key)
		if !ok || !contains(OSes, key) {
			c.add("cmd-unknown-key", l, fmt.Sprintf("Command line %q does not start with windows:, linux: or macos:", truncate(s, 40)),
				"Write exactly three lines: windows: <command>, linux: <command>, macos: <command>")
			continue
		}
		if _, dup := b.Variants[key]; dup {
			c.add("cmd-duplicate-key", l, fmt.Sprintf("%s: appears twice", key),
				"Keep one line per OS; join multi-step commands with && or use ::: steps")
			continue
		}
		val = strings.TrimSpace(val)
		v := Variant{Command: val}
		if rest, isNone := strings.CutPrefix(val, "none:"); isNone {
			v = Variant{None: strings.TrimSpace(rest)}
			if v.None == "" {
				c.add("cmd-empty", l, fmt.Sprintf("%s: none: has no reason", key),
					fmt.Sprintf("Say why, for example %s: none: Not available on this OS", key))
				continue
			}
		} else if val == "" {
			c.add("cmd-empty", l, fmt.Sprintf("%s: has no command", key),
				fmt.Sprintf("Write the command after %s:, or %s: none: <reason> if it does not apply", key, key))
			continue
		}
		b.Variants[key] = v
	}
	missing := false
	for _, os := range OSes {
		if _, ok := b.Variants[os]; !ok {
			missing = true
			c.add("cmd-missing-variant", line, fmt.Sprintf("Command %q has no %s: line", b.Caption, os),
				fmt.Sprintf("Add %s: <command>, or %s: none: <reason> if the command does not apply there", os, os))
		}
	}
	if missing || b.Caption == "" {
		return nil
	}
	return b
}

func (c *conv) cards(n *Container, line int) *Block {
	b := &Block{Type: TCards, Line: line}
	fix := "Write each card as - [Title](doc:slug) — one line of text"
	for ch := n.FirstChild(); ch != nil; ch = ch.NextSibling() {
		list, ok := ch.(*ast.List)
		if !ok || list.IsOrdered() {
			c.add("cards-item", c.nodeLine(ch), "::: cards holds something other than a - list", fix)
			continue
		}
		for li := list.FirstChild(); li != nil; li = li.NextSibling() {
			il := c.nodeLine(li)
			card := c.card(li)
			if card == nil {
				c.add("cards-item", il, "Card is not [Title](doc:slug) — text", fix)
				continue
			}
			card.Line = il
			b.Cards = append(b.Cards, card)
		}
	}
	if len(b.Cards) == 0 {
		c.add("cards-item", line, "::: cards holds no cards", fix)
		return nil
	}
	return b
}

func (c *conv) card(li ast.Node) *Card {
	if li.ChildCount() != 1 {
		return nil
	}
	para := li.FirstChild()
	link, ok := para.FirstChild().(*ast.Link)
	if !ok {
		return nil
	}
	dest := string(link.Destination)
	slug, section, ok := ParseDocLink(dest)
	if !ok {
		return nil
	}
	var title, rest strings.Builder
	for ch := link.FirstChild(); ch != nil; ch = ch.NextSibling() {
		title.WriteString(c.plain(ch))
	}
	for ch := link.NextSibling(); ch != nil; ch = ch.NextSibling() {
		rest.WriteString(c.plain(ch))
		if t, ok := ch.(*ast.Text); ok && t.SoftLineBreak() {
			rest.WriteByte(' ')
		}
	}
	text, ok := strings.CutPrefix(strings.TrimSpace(rest.String()), "—")
	text = strings.TrimSpace(text)
	if !ok || text == "" || strings.TrimSpace(title.String()) == "" {
		return nil
	}
	return &Card{Title: strings.TrimSpace(title.String()), Doc: slug, Section: section, Text: text}
}

// plain returns the text of an inline node and its children.
func (c *conv) plain(n ast.Node) string {
	switch n := n.(type) {
	case *ast.Text:
		return unescape(n.Segment.Value(c.src))
	case *ast.String:
		return string(n.Value)
	case *ast.CodeSpan:
		return c.codeText(n)
	}
	var sb strings.Builder
	for ch := n.FirstChild(); ch != nil; ch = ch.NextSibling() {
		sb.WriteString(c.plain(ch))
	}
	return sb.String()
}

var docLinkRe = regexp.MustCompile(`^doc:([a-z0-9]+(?:-[a-z0-9]+)*)(?:#([a-z0-9]+(?:-[a-z0-9]+)*))?$`)

// ParseDocLink splits "doc:slug#id" into slug and section id.
func ParseDocLink(dest string) (slug, section string, ok bool) {
	m := docLinkRe.FindStringSubmatch(dest)
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

func (c *conv) list(n *ast.List, line int) *Block {
	b := &Block{Type: TList, Ordered: n.IsOrdered(), Line: line}
	c.checkMarker(n, line)
	b.Items = c.listItems(n, 1)
	if len(b.Items) == 0 {
		return nil
	}
	return b
}

func (c *conv) checkMarker(n *ast.List, line int) {
	if (n.IsOrdered() && n.Marker != '.') || (!n.IsOrdered() && n.Marker != '-') {
		c.add("list-marker", line, fmt.Sprintf("List marker %q", string(n.Marker)),
			`Start bullet items with "- " and numbered items with "1. "`)
	}
}

func (c *conv) listItems(n *ast.List, depth int) []*ListItem {
	var items []*ListItem
	for li := n.FirstChild(); li != nil; li = li.NextSibling() {
		il := c.nodeLine(li)
		item := &ListItem{}
		for ch := li.FirstChild(); ch != nil; ch = ch.NextSibling() {
			switch ch := ch.(type) {
			case *ast.TextBlock, *ast.Paragraph:
				if item.Text != nil {
					c.add("list-content", c.nodeLine(ch), "A list item holds more than one paragraph",
						"Keep one paragraph per item, or use ::: steps for a procedure")
					continue
				}
				item.Text = c.runs(ch)
			case *ast.List:
				nl := c.nodeLine(ch)
				if depth >= 3 {
					c.add("list-depth", nl, "Lists are nested more than 3 levels",
						"Flatten the list, or split it into sections")
					continue
				}
				c.checkMarker(ch, nl)
				if indent := leadingIndent(c.lineText(nl)); indent != 4*depth {
					c.add("list-indent", nl, fmt.Sprintf("Nested list is indented %d spaces", indent),
						fmt.Sprintf("Indent it by exactly %d spaces", 4*depth))
				}
				if item.Items != nil {
					c.add("list-content", nl, "A list item holds two nested lists", "Merge them into one nested list")
					continue
				}
				item.Items = c.listItems(ch, depth+1)
			default:
				c.add("list-content", c.nodeLine(ch), fmt.Sprintf("A list item holds %s", kindName(ch)),
					"Move it out of the list; use ::: steps when a procedure needs code or commands")
			}
		}
		if len(item.Text) == 0 {
			c.add("list-content", il, "Empty list item", "Write text after the marker, or remove the item")
			continue
		}
		items = append(items, item)
	}
	return items
}

// kindName describes a goldmark node for a message.
func kindName(n ast.Node) string {
	switch n.(type) {
	case *ast.FencedCodeBlock, *ast.CodeBlock:
		return "a code block"
	case *ast.Blockquote:
		return "a blockquote"
	case *ast.Heading:
		return "a heading"
	case *ast.HTMLBlock:
		return "HTML"
	case *ast.ThematicBreak:
		return "a horizontal rule"
	case *east.Table:
		return "a table"
	case *east.DefinitionList:
		return "a definition list"
	case *Container:
		return "a ::: " + n.(*Container).Name + " container"
	}
	return "a " + strings.ToLower(n.Kind().String())
}

func leadingIndent(s string) int {
	n := 0
	for _, r := range s {
		switch r {
		case ' ':
			n++
		case '\t':
			n += 4
		default:
			return n
		}
	}
	return n
}

func (c *conv) deflist(n *east.DefinitionList, line int) *Block {
	b := &Block{Type: TDefinitionList, Line: line}
	var cur *Def
	described := map[*Def]bool{}
	for ch := n.FirstChild(); ch != nil; ch = ch.NextSibling() {
		cl := c.nodeLine(ch)
		switch ch := ch.(type) {
		case *east.DefinitionTerm:
			cur = &Def{Term: c.runs(ch)}
			b.Defs = append(b.Defs, cur)
		case *east.DefinitionDescription:
			if cur == nil {
				continue
			}
			first := !described[cur]
			described[cur] = true
			if !first {
				c.add("deflist-content", cl, "A term has more than one definition",
					"Merge the definitions into one sentence or paragraph")
				continue
			}
			if ch.ChildCount() != 1 {
				c.add("deflist-content", cl, "A definition holds more than one paragraph or block",
					"Keep each definition to one paragraph of text")
				continue
			}
			switch p := ch.FirstChild().(type) {
			case *ast.Paragraph, *ast.TextBlock:
				cur.Definition = c.runs(p)
			default:
				c.add("deflist-content", cl, "A definition holds something other than text",
					"Keep each definition to one paragraph of text")
			}
		}
	}
	for _, d := range b.Defs {
		if !described[d] {
			c.add("deflist-content", line, fmt.Sprintf("Term %q has no definition", runsText(d.Term)),
				"Follow each term with a line starting with ':   ' and the definition")
		}
		if len(d.Definition) == 0 || len(d.Term) == 0 {
			return nil
		}
	}
	return b
}

var referenceShapes = map[string]string{
	"flag": "{{ flags group=\"…\" }}", "flags": "{{ flags group=\"…\" }}",
	"field": "{{ fields dataset=\"…\" }}", "fields": "{{ fields dataset=\"…\" }}",
	"platform": "{{ platforms }}", "platforms": "{{ platforms }}",
}

func (c *conv) table(n *east.Table, line int) *Block {
	b := &Block{Type: TTable, Line: line}
	ok := true
	for ch := n.FirstChild(); ch != nil; ch = ch.NextSibling() {
		var cells [][]Run
		for cell := ch.FirstChild(); cell != nil; cell = cell.NextSibling() {
			cells = append(cells, c.runs(cell))
		}
		if _, isHeader := ch.(*east.TableHeader); isHeader {
			b.Header = cells
		} else {
			b.Rows = append(b.Rows, cells)
		}
	}
	for i, h := range b.Header {
		if len(h) == 0 {
			c.add("table-header", line, fmt.Sprintf("Header cell %d is empty", i+1), "Give every column a name")
			ok = false
		}
	}
	if len(b.Header) > 0 && len(b.Header[0]) > 0 {
		first := strings.ToLower(strings.TrimSpace(runsText(b.Header[0])))
		if ref, isRef := referenceShapes[first]; isRef {
			c.add("table-reference-shape", line, fmt.Sprintf("Hand-typed %s table", first),
				fmt.Sprintf("Delete the table and write %s in its place; the rows come from data/", ref))
			ok = false
		}
	}
	if len(b.Rows) > 12 {
		c.add("table-rows", line, fmt.Sprintf("Table has %d rows; the maximum is 12", len(b.Rows)),
			"Split the table, or move the facts into a data file and use a {{ … }} reference")
		ok = false
	}
	want := countCells(c.lineText(line))
	for i := range b.Rows {
		l := line + 2 + i
		if got := countCells(c.lineText(l)); got != want {
			c.add("table-ragged", l, fmt.Sprintf("Row has %d cells; the header has %d", got, want),
				fmt.Sprintf("Give the row exactly %d cells; leave a cell empty with | |", want))
			ok = false
		}
	}
	if !ok {
		return nil
	}
	return b
}

// countCells counts the cells of one pipe-table source line, ignoring escaped
// pipes and pipes inside code spans.
func countCells(line string) int {
	s := strings.TrimSpace(line)
	s = strings.TrimPrefix(s, "|")
	if strings.HasSuffix(s, "|") && !strings.HasSuffix(s, `\|`) {
		s = s[:len(s)-1]
	}
	n := 1
	inCode := false
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case '`':
			inCode = !inCode
		case '|':
			if !inCode {
				n++
			}
		}
	}
	return n
}

func runsText(runs []Run) string {
	var sb strings.Builder
	for _, r := range runs {
		sb.WriteString(r.T)
	}
	return sb.String()
}

// RunsText returns the plain text of a run slice.
func RunsText(runs []Run) string { return runsText(runs) }

func truncate(s string, n int) string {
	if len([]rune(s)) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "…"
}
