package rules

import (
	"fmt"
	"strings"

	"github.com/42kohd/docpack/internal/dialect"
)

type tsec struct {
	title     string
	required  bool
	generated bool
}

type layout int

const (
	fixed     layout = iota // only the listed sections, in order
	bookended               // Overview first, free middle, Related last
	related                 // free sections, Related last
	free                    // anything
)

type template struct {
	layout   layout
	sections []tsec
}

func req(t string) tsec { return tsec{title: t, required: true} }
func opt(t string) tsec { return tsec{title: t} }
func gen(t string) tsec { return tsec{title: t, generated: true} }

// Templates are the section templates of SPEC A2.2.
var templates = map[string]template{
	"integration": {fixed, []tsec{req("Overview"), opt("Prerequisites"), req("Configuration"), opt("Target configuration"),
		opt("Data reference"), opt("Queries"), opt("Dashboards"), opt("Capacity"), req("Troubleshooting"), req("Related")}},
	"howto":     {fixed, []tsec{req("Goal"), opt("Prerequisites"), req("Steps"), req("Result"), opt("Options"), req("Related")}},
	"format":    {fixed, []tsec{req("Overview"), req("Best for"), req("Usage"), req("Schema"), opt("Examples"), opt("Notes"), req("Related")}},
	"release":   {fixed, []tsec{req("What's new"), opt("Fixes"), opt("Advisories"), opt("Upgrade notes"), opt("Known issues")}},
	"dashboard": {fixed, []tsec{req("Purpose"), gen("Panels"), gen("Fields used"), opt("Queries"), req("Related")}},
	"reference": {bookended, []tsec{req("Overview"), req("Related")}},
	"concept":   {related, []tsec{req("Related")}},
	"landing":   {free, nil},
}

// TemplateTitles returns the top-level section titles of a type in order,
// with required ones marked by a trailing "*". Used by docpack new and docs.
func TemplateTitles(docType string) []string {
	var out []string
	for _, s := range templates[docType].sections {
		t := s.title
		if s.required {
			t += "*"
		}
		out = append(out, t)
	}
	return out
}

func (t template) index(title string) int {
	for i, s := range t.sections {
		if s.title == title {
			return i
		}
	}
	return -1
}

func (t template) caseMatch(title string) string {
	norm := func(s string) string { return strings.ReplaceAll(strings.ToLower(s), "’", "'") }
	for _, s := range t.sections {
		if norm(s.title) == norm(title) {
			return s.title
		}
	}
	return ""
}

func (t template) list() string {
	var names []string
	for _, s := range t.sections {
		if s.generated {
			continue
		}
		n := s.title
		if s.required {
			n += " (required)"
		}
		names = append(names, n)
	}
	return strings.Join(names, ", ")
}

func (c *checker) sections(d *dialect.Document) {
	t, ok := templates[d.Front.Type]
	if !ok || t.layout == free {
		return
	}
	top := d.Body
	missingLine := d.BodyLine
	if len(top) > 0 {
		missingLine = top[len(top)-1].Line
	}
	if t.layout == fixed {
		c.fixedSections(d, t, top, missingLine)
		return
	}
	// bookended and related layouts
	if t.layout == bookended && (len(top) == 0 || top[0].Title != "Overview") {
		line := d.BodyLine
		if len(top) > 0 {
			line = top[0].Line
		}
		c.docAdd(d, "section-missing", line, fmt.Sprintf("A %s document must start with ## Overview", d.Front.Type),
			"Add ## Overview as the first section")
	}
	relIdx := -1
	for i, s := range top {
		if s.Title == "Related" {
			if relIdx >= 0 {
				c.docAdd(d, "section-duplicate", s.Line, "## Related appears twice", "Merge the two Related sections")
			}
			relIdx = i
		} else if s.Title == "Overview" && t.layout == bookended && i > 0 {
			c.docAdd(d, "section-duplicate", s.Line, "## Overview appears again", "Keep one Overview section, first")
		}
	}
	switch {
	case relIdx < 0:
		c.docAdd(d, "section-missing", missingLine, "No ## Related section",
			"Add ## Related as the last section, with ::: cards linking to related documents")
	case relIdx != len(top)-1:
		c.docAdd(d, "section-order", top[relIdx+1].Line, fmt.Sprintf("Section %q comes after ## Related", top[relIdx+1].Title),
			"Move ## Related to the end of the document")
	}
}

func (c *checker) fixedSections(d *dialect.Document, t template, top []*dialect.Block, missingLine int) {
	seen := map[int]bool{}
	maxIdx, maxTitle := -1, ""
	for _, s := range top {
		idx := t.index(s.Title)
		if idx < 0 {
			if m := t.caseMatch(s.Title); m != "" {
				c.docAdd(d, "section-unknown", s.Line, fmt.Sprintf("Section %q is not in the %s template", s.Title, d.Front.Type),
					fmt.Sprintf("Rename it to %q; section titles are exact", m))
			} else {
				c.docAdd(d, "section-unknown", s.Line, fmt.Sprintf("Section %q is not in the %s template", s.Title, d.Front.Type),
					fmt.Sprintf("Move the content into one of: %s, or make it a ### subsection", t.list()))
			}
			continue
		}
		if t.sections[idx].generated {
			c.docAdd(d, "section-generated", s.Line, fmt.Sprintf("## %s is generated", s.Title),
				"Delete this section; it is built from data/dashboards/<target>.yaml")
			continue
		}
		if seen[idx] {
			c.docAdd(d, "section-duplicate", s.Line, fmt.Sprintf("## %s appears twice", s.Title), "Merge the two sections")
			continue
		}
		seen[idx] = true
		if idx < maxIdx {
			c.docAdd(d, "section-order", s.Line, fmt.Sprintf("## %s comes after ## %s", s.Title, maxTitle),
				fmt.Sprintf("Move ## %s before ## %s; the order is: %s", s.Title, maxTitle, t.list()))
			continue
		}
		maxIdx, maxTitle = idx, s.Title
	}
	for i, s := range t.sections {
		if s.required && !seen[i] {
			where := "as the first section"
			for j := i - 1; j >= 0; j-- {
				if seen[j] {
					where = "after ## " + t.sections[j].title
					break
				}
			}
			c.docAdd(d, "section-missing", missingLine, fmt.Sprintf("No ## %s section", s.title),
				fmt.Sprintf("Add ## %s %s", s.title, where))
		}
	}
}

// generate inserts the generated Panels and Fields used sections of a
// dashboard document after Purpose.
func (c *checker) generate(d *dialect.Document) {
	if d.Front.Type != "dashboard" {
		return
	}
	board, ok := c.dashboard(d.Front.Target, d.Front.DashboardID)
	if !ok {
		return
	}
	at := 0
	for i, s := range d.Body {
		if s.Title == "Purpose" {
			at = i + 1
		}
	}
	filter := map[string]string{"target": d.Front.Target, "dashboard": board.ID}
	panels := &dialect.Block{Type: dialect.TSection, ID: uniqueID(d, "panels"), Title: "Panels", Level: 2, Line: 0}
	if board.Screenshot != "" {
		c.referenced[board.Screenshot] = true
		panels.Body = append(panels.Body, &dialect.Block{Type: dialect.TImage, Src: board.Screenshot,
			Alt: board.Title + " dashboard", Caption: board.Purpose})
	}
	pt := &dialect.Block{Type: dialect.TReferenceTable, Source: "panels", Filter: filter, Generated: true,
		Columns: []string{"Panel", "Query", "Fields"}}
	var order []string
	seen := map[string]bool{}
	for _, pn := range board.Panels {
		pt.RefRows = append(pt.RefRows, []string{pn.Title, pn.Query, strings.Join(pn.Fields, ", ")})
		for _, f := range pn.Fields {
			if !seen[f] {
				seen[f] = true
				order = append(order, f)
			}
		}
	}
	panels.Body = append(panels.Body, pt)
	ft := &dialect.Block{Type: dialect.TReferenceTable, Source: "dashboard_fields", Filter: filter, Generated: true,
		Columns: []string{"Field", "Dataset", "Type", "Description"}}
	for _, name := range order {
		ds, f, ok := c.field(name)
		if !ok {
			continue // reported against the data file
		}
		ft.RefRows = append(ft.RefRows, []string{f.Name, ds, f.Type, f.Description})
	}
	fields := &dialect.Block{Type: dialect.TSection, ID: uniqueID(d, "fields-used"), Title: "Fields used", Level: 2,
		Body: []*dialect.Block{ft}}
	if len(ft.RefRows) == 0 {
		fields = nil
	}
	gen := []*dialect.Block{panels}
	if fields != nil {
		gen = append(gen, fields)
	}
	body := append([]*dialect.Block{}, d.Body[:at]...)
	body = append(body, gen...)
	d.Body = append(body, d.Body[at:]...)
}

func uniqueID(d *dialect.Document, id string) string {
	if !d.HasSection(id) {
		return id
	}
	for n := 2; ; n++ {
		if cand := fmt.Sprintf("%s-%d", id, n); !d.HasSection(cand) {
			return cand
		}
	}
}
