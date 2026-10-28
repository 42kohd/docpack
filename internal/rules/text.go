package rules

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/42kohd/docpack/internal/dialect"
)

// Placeholders are the substitutions of SPEC A4.
var Placeholders = []string{"product", "short", "binary", "version", "support_email"}

var (
	placeholderRe = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]*)\}`)
	flagSpanRe    = regexp.MustCompile(`^--?[A-Za-z]`)
	versionRe     = regexp.MustCompile(`(^|[^0-9.])v?[0-9]+\.[0-9]+\.[0-9]+([^0-9.]|$)`)
	linkDestRe    = regexp.MustCompile(`\]\([^)]*\)`)
)

// inline checks links, flag and field spans and placeholders, and marks
// flag and field runs.
func (c *checker) inline(d *dialect.Document) {
	release := d.Front.Type == "release"
	checkTitle := func(s string, line int) { c.placeholders(d, s, line) }
	dialect.Walk(d.Body, func(b *dialect.Block) {
		switch b.Type {
		case dialect.TSection, dialect.TCallout:
			checkTitle(b.Title, b.Line)
		case dialect.TCommand:
			checkTitle(b.Caption, b.Line)
		case dialect.TSteps:
			for _, s := range b.Steps {
				checkTitle(s.Title, s.Line)
			}
		case dialect.TCards:
			for _, card := range b.Cards {
				c.docLink(d, card.Doc, card.Section, card.Line)
				checkTitle(card.Title, card.Line)
				checkTitle(card.Text, card.Line)
			}
		case dialect.TImage:
			checkTitle(b.Alt, b.Line)
			checkTitle(b.Caption, b.Line)
		}
		for _, runs := range b.RunSlices() {
			for i := range runs {
				r := &runs[i]
				line := r.Line
				if line == 0 {
					line = b.Line
				}
				if slug, sec, ok := dialect.ParseDocLink(r.Link); ok {
					c.docLink(d, slug, sec, line)
				}
				if !r.Code {
					c.placeholders(d, r.T, line)
					continue
				}
				c.codeSpan(d, r, line, release)
			}
		}
	})
}

func (c *checker) codeSpan(d *dialect.Document, r *dialect.Run, line int, release bool) {
	text := strings.TrimSpace(r.T)
	if flagSpanRe.MatchString(text) {
		name := text
		if i := strings.IndexAny(name, "= "); i >= 0 {
			name = name[:i]
		}
		f, ok := c.flag(name)
		if !ok {
			c.docAdd(d, "flag-unknown", line, fmt.Sprintf("Flag %s is not in data/flags.yaml", name),
				"Check the spelling, or add the flag to data/flags.yaml")
			return
		}
		r.Flag = true
		if f.Status == "deprecated" && !release {
			c.docAdd(d, "flag-deprecated", line, fmt.Sprintf("Flag %s is deprecated", name),
				fmt.Sprintf("Use %s instead", f.ReplacedBy))
		}
		return
	}
	if _, _, ok := c.field(text); ok {
		r.Field = true
	}
}

func (c *checker) placeholders(d *dialect.Document, s string, line int) {
	for _, m := range placeholderRe.FindAllStringSubmatch(s, -1) {
		if !contains(Placeholders, m[1]) {
			c.docAdd(d, "placeholder-unknown", line, fmt.Sprintf("Unknown placeholder {%s}", m[1]),
				"Use one of {product}, {short}, {binary}, {version}, {support_email}, or rephrase without braces")
		}
	}
}

func (c *checker) docLink(d *dialect.Document, slug, section string, line int) {
	target, ok := c.p.DocBySlug[slug]
	if !ok {
		fix := "Use the slug of an existing document (its filename without .md)"
		for _, o := range c.p.Docs {
			if contains(o.Front.Aliases, slug) {
				fix = fmt.Sprintf("%q is a former slug; link to doc:%s", slug, o.Name)
			}
		}
		c.docAdd(d, "link-doc-missing", line, fmt.Sprintf("Link to doc:%s, which has no document", slug), fix)
		return
	}
	if section != "" && !target.HasSection(section) {
		c.docAdd(d, "link-section-missing", line, fmt.Sprintf("doc:%s has no section #%s", slug, section),
			fmt.Sprintf("Use one of its section ids: %s", strings.Join(target.SectionIDs(), ", ")))
	}
}

func (c *checker) images(d *dialect.Document) {
	dialect.Walk(d.Body, func(b *dialect.Block) {
		if b.Type != dialect.TImage || b.Line == 0 {
			return // generated screenshots are checked with the data
		}
		c.referenced[b.Src] = true
		if !c.p.HasAsset(b.Src) {
			c.docAdd(d, "image-missing", b.Line, fmt.Sprintf("Image %s does not exist", b.Src),
				"Add the file under assets/, or correct the path")
		}
	})
}

// text scans the source lines for skeleton markers and literal product facts.
func (c *checker) text(d *dialect.Document) {
	lines := strings.Split(string(d.Source), "\n")
	for i, l := range lines {
		if strings.Contains(l, "_fill:") {
			c.docAdd(d, "fill-left", i+1, "A _fill: marker is still in the document",
				"Replace the _fill: line with the content it describes")
		}
	}
	if d.Front.Type == "release" {
		return
	}
	pr := c.p.Product
	var nameRe, binRe *regexp.Regexp
	if pr.Name != "" {
		nameRe = regexp.MustCompile(`(^|[^A-Za-z0-9_])` + regexp.QuoteMeta(pr.Name) + `($|[^A-Za-z0-9_])`)
	}
	if pr.Binary != "" && pr.Binary != pr.Name {
		binRe = regexp.MustCompile(`(^|[^A-Za-z0-9_])` + regexp.QuoteMeta(pr.Binary) + `($|[^A-Za-z0-9_])`)
	}
	for i := d.BodyLine - 1; i < len(lines); i++ {
		l := linkDestRe.ReplaceAllString(lines[i], "]()")
		if nameRe != nil && nameRe.MatchString(l) {
			c.docAdd(d, "literal-product", i+1, fmt.Sprintf("The product name %q is written out", pr.Name),
				"Write {product} instead")
		}
		if binRe != nil && binRe.MatchString(l) {
			c.docAdd(d, "literal-binary", i+1, fmt.Sprintf("The binary name %q is written out", pr.Binary),
				"Write {binary} instead")
		}
		if versionRe.MatchString(l) {
			c.docAdd(d, "literal-version", i+1, "A version number is written out",
				"Write {version} for the current version; describe older behavior in a release document")
		}
	}
}

func (c *checker) assets() {
	for _, a := range c.p.Assets {
		if !c.referenced[a] {
			c.p.Diags.Add("asset-unused", a, 0, "Asset is not referenced by any document or data file",
				"Reference it from a document or data file, or delete it")
		}
	}
}

func (c *checker) nav() {
	p := c.p
	if p.Product.Name == "" {
		return // product.yaml missing or invalid; already reported
	}
	_, listed, diags := p.ResolveNav()
	p.Diags = append(p.Diags, diags...)
	count := map[string]int{}
	for _, s := range listed {
		count[s]++
	}
	reported := map[string]bool{}
	for _, s := range listed {
		if count[s] > 1 && !reported[s] {
			reported[s] = true
			p.Diags.Add("nav-duplicate", "product.yaml", 0, fmt.Sprintf("%s appears %d times in navigation", s, count[s]),
				"List each document once; use related for cross-links")
		}
	}
	reachable := map[string]bool{}
	for s := range count {
		reachable[s] = true
	}
	for _, d := range p.Docs {
		for _, r := range d.Front.Related {
			if r != d.Name {
				reachable[r] = true
			}
		}
	}
	for _, d := range p.Docs {
		if !reachable[d.Name] {
			d.Diags.Add("nav-unreachable", d.Path, 1, "The document is not in navigation and no document lists it in related",
				fmt.Sprintf("Add %s to nav in product.yaml, or to the related key of a document about the same topic", d.Name))
		}
	}
}
