package project

import (
	"fmt"
	"sort"
	"strings"

	"github.com/42kohd/docpack/internal/dialect"
	"github.com/42kohd/docpack/internal/report"
)

// NavOut is one resolved navigation node (SPEC B2).
type NavOut struct {
	Title    string    `json:"title"`
	Doc      string    `json:"doc,omitempty"`
	Children []*NavOut `json:"children,omitempty"`
}

// ResolveNav expands product.yaml nav into the manifest form. It returns the
// tree, the slugs it lists (with repeats), and the problems found.
func (p *Project) ResolveNav() ([]*NavOut, []string, report.List) {
	r := &navResolver{p: p}
	var out []*NavOut
	for _, g := range p.Nav {
		if n := r.node(g, 1); n != nil {
			out = append(out, n)
		}
	}
	return out, r.listed, r.diags
}

type navResolver struct {
	p      *Project
	listed []string
	diags  report.List
}

func (r *navResolver) add(rule string, line int, msg, fix string) {
	r.diags.Add(rule, "product.yaml", line, msg, fix)
}

func (r *navResolver) node(n *NavNode, depth int) *NavOut {
	if depth > 3 {
		r.add("nav-depth", n.Line, "Navigation is nested more than 3 levels",
			"Move this entry up a level; the top-level group counts as level 1")
		return nil
	}
	switch {
	case n.Group != "" && n.From != "":
		out := &NavOut{Title: n.Group}
		docs, ok := r.generated(n)
		if !ok {
			return nil
		}
		if len(docs) > 0 && depth+1 > 3 {
			r.add("nav-depth", n.Line, fmt.Sprintf("Generated group %q puts its documents at level %d", n.Group, depth+1),
				"Move the generated group up a level; navigation is at most 3 levels deep")
			return nil
		}
		for _, d := range docs {
			r.listed = append(r.listed, d.Name)
			out.Children = append(out.Children, &NavOut{Title: DisplayTitle(d), Doc: d.Name})
		}
		if len(out.Children) == 0 {
			return nil
		}
		return out
	case n.Group != "":
		out := &NavOut{Title: n.Group}
		for _, c := range n.Children {
			if child := r.node(c, depth+1); child != nil {
				out.Children = append(out.Children, child)
			}
		}
		if len(out.Children) == 0 {
			return nil
		}
		return out
	}
	d, ok := r.p.DocBySlug[n.Doc]
	if !ok {
		r.add("nav-missing-doc", n.Line, fmt.Sprintf("Navigation lists %q, which has no document", n.Doc),
			fmt.Sprintf("Create docs/%s.md (docpack new <type> %s), or remove the entry", n.Doc, n.Doc))
		return nil
	}
	r.listed = append(r.listed, d.Name)
	out := &NavOut{Title: DisplayTitle(d), Doc: d.Name}
	for _, c := range n.Children {
		if child := r.node(c, depth+1); child != nil {
			out.Children = append(out.Children, child)
		}
	}
	return out
}

// generated lists the documents of a {group, from} entry.
func (r *navResolver) generated(n *NavNode) ([]*dialect.Document, bool) {
	var docs []*dialect.Document
	switch {
	case n.From == "releases":
		for _, d := range r.p.Docs {
			if d.Front.Type == "release" {
				docs = append(docs, d)
			}
		}
		sort.SliceStable(docs, func(i, j int) bool {
			return CompareVersions(docs[i].Front.Version, docs[j].Front.Version) > 0
		})
	case strings.HasPrefix(n.From, "dashboards/"):
		target := strings.TrimPrefix(n.From, "dashboards/")
		boards, ok := r.p.Dashboards[target]
		if !ok {
			r.add("nav-source", n.Line, fmt.Sprintf("from: %s names a target with no data/dashboards/%s.yaml", n.From, target),
				"Use from: dashboards/<target> with a file that exists in data/dashboards/, or from: releases")
			return nil, false
		}
		order := map[string]int{}
		for i, b := range boards {
			order[b.ID] = i
		}
		for _, d := range r.p.Docs {
			if d.Front.Type == "dashboard" && d.Front.Target == target {
				docs = append(docs, d)
			}
		}
		sort.SliceStable(docs, func(i, j int) bool {
			oi, iok := order[docs[i].Front.DashboardID]
			oj, jok := order[docs[j].Front.DashboardID]
			if iok != jok {
				return iok
			}
			return oi < oj
		})
	default:
		r.add("nav-source", n.Line, fmt.Sprintf("Unknown nav source from: %s", n.From),
			"Use from: releases or from: dashboards/<target>")
		return nil, false
	}
	if n.Limit > 0 && len(docs) > n.Limit {
		docs = docs[:n.Limit]
	}
	return docs, true
}
