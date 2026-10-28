package dialect

import (
	"fmt"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/util"
)

// unescape resolves backslash escapes and entity references in text.
func unescape(b []byte) string {
	return string(util.ResolveNumericReferences(util.ResolveEntityNames(util.UnescapePunctuations(b))))
}

func (c *conv) codeText(n *ast.CodeSpan) string {
	var sb strings.Builder
	for ch := n.FirstChild(); ch != nil; ch = ch.NextSibling() {
		switch t := ch.(type) {
		case *ast.Text:
			sb.Write(t.Segment.Value(c.src))
			if t.SoftLineBreak() {
				sb.WriteByte(' ')
			}
		case *ast.String:
			sb.Write(t.Value)
		}
	}
	return sb.String()
}

// runs converts the inline children of a block into merged runs.
func (c *conv) runs(n ast.Node) []Run {
	var out []Run
	c.inline(n, Run{}, &out)
	return mergeRuns(out)
}

func (c *conv) inline(n ast.Node, style Run, out *[]Run) {
	for ch := n.FirstChild(); ch != nil; ch = ch.NextSibling() {
		switch ch := ch.(type) {
		case *ast.Text:
			r := style
			r.T = unescape(ch.Segment.Value(c.src))
			r.Line = c.line(ch.Segment.Start)
			*out = append(*out, r)
			if ch.SoftLineBreak() || ch.HardLineBreak() {
				sp := style
				sp.T, sp.Line = " ", r.Line
				*out = append(*out, sp)
			}
		case *ast.String:
			r := style
			r.T = string(ch.Value)
			r.Line = c.last
			*out = append(*out, r)
		case *ast.CodeSpan:
			r := style
			r.Code = true
			r.T = c.codeText(ch)
			r.Line = c.nodeLine(ch)
			*out = append(*out, r)
		case *ast.Emphasis:
			s := style
			if ch.Level >= 2 {
				s.Bold = true
			} else {
				s.Italic = true
			}
			c.inline(ch, s, out)
		case *ast.Link:
			s := style
			s.Link = c.checkLink(string(ch.Destination), c.nodeLine(ch))
			c.inline(ch, s, out)
		case *ast.AutoLink:
			c.add("inline-forbidden", c.nodeLine(ch), fmt.Sprintf("Autolink <%s>", ch.URL(c.src)),
				"Write it as a link with words: [what the page is](https://…)")
		case *ast.Image:
			c.add("inline-forbidden", c.nodeLine(ch), "Image inside a line of text",
				"Put the image on its own line, as its own paragraph")
		case *ast.RawHTML:
			c.add("html-forbidden", c.nodeLine(ch), "Raw HTML in text",
				"Remove the HTML; use **bold**, *italic*, `code` or a link instead")
		default:
			c.add("inline-forbidden", c.nodeLine(ch), fmt.Sprintf("Unsupported inline markup (%s)", ch.Kind()),
				"Use only **bold**, *italic*, `code` and links")
		}
	}
}

// checkLink validates a link destination and returns it, or "" when it is
// not allowed (the text is kept without the link).
func (c *conv) checkLink(dest string, line int) string {
	switch {
	case strings.HasPrefix(dest, "https://") && len(dest) > len("https://"):
		return dest
	case strings.HasPrefix(dest, "doc:"):
		if _, _, ok := ParseDocLink(dest); ok {
			return dest
		}
		c.add("link-scheme", line, fmt.Sprintf("Malformed document link %q", dest),
			"Write doc:<slug> or doc:<slug>#<section-id>, using lowercase words joined by -")
	case strings.HasPrefix(dest, "http://"):
		c.add("link-scheme", line, fmt.Sprintf("Insecure link %q", dest),
			"Use the https:// address of the page")
	case strings.Contains(dest, ".html") && !strings.Contains(dest, "://"):
		c.add("link-html", line, fmt.Sprintf("Link to a page file %q", dest),
			"Link to the document instead: [text](doc:<slug>)")
	case strings.HasPrefix(dest, "mailto:"):
		c.add("link-scheme", line, fmt.Sprintf("Mail link %q", dest),
			"Write the address as text; use {support_email} for the support address")
	case strings.Contains(dest, "://"):
		c.add("link-scheme", line, fmt.Sprintf("Link %q is not https://", dest),
			"Use an https:// address, or doc:<slug> for a document")
	default:
		c.add("link-html", line, fmt.Sprintf("Relative link %q", dest),
			"Link to a document with [text](doc:<slug>) or to a page with https://")
	}
	return ""
}

// mergeRuns joins adjacent runs with the same style, trims the ends and drops
// empty runs.
func mergeRuns(in []Run) []Run {
	var out []Run
	for _, r := range in {
		if r.T == "" {
			continue
		}
		if n := len(out); n > 0 && out[n-1].sameStyle(r) {
			out[n-1].T += r.T
			continue
		}
		out = append(out, r)
	}
	if len(out) > 0 {
		out[0].T = strings.TrimLeft(out[0].T, " \t")
		last := len(out) - 1
		out[last].T = strings.TrimRight(out[last].T, " \t")
	}
	res := out[:0]
	for _, r := range out {
		if r.T != "" {
			res = append(res, r)
		}
	}
	if len(res) == 0 {
		return nil
	}
	return res
}
