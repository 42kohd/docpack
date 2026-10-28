// Package rules runs the structural checks that need the whole project:
// section templates, cross-document links, data references, literals,
// assets and navigation (SPEC A2–A7). It also resolves reference tables and
// generated sections in place, so the block tree is ready to compile.
package rules

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/42kohd/docpack/internal/dialect"
	"github.com/42kohd/docpack/internal/project"
)

// Check runs every rule over a loaded project, appending to p.Diags and the
// documents' own diagnostics.
func Check(p *project.Project) {
	c := &checker{p: p, referenced: map[string]bool{}}
	c.data()
	for _, d := range p.Docs {
		c.frontmatter(d)
		c.sections(d)
		c.generate(d)
		d.Reindex()
	}
	for _, d := range p.Docs {
		c.references(d)
		c.inline(d)
		c.images(d)
		c.text(d)
	}
	c.fillData()
	c.assets()
	c.nav()
}

type checker struct {
	p          *project.Project
	referenced map[string]bool // asset paths referenced anywhere
}

func (c *checker) docAdd(d *dialect.Document, rule string, line int, msg, fix string) {
	d.Diags.Add(rule, d.Path, line, msg, fix)
}

func (c *checker) frontmatter(d *dialect.Document) {
	f := &d.Front
	if utf8.RuneCountInString(f.Summary) > 160 {
		c.docAdd(d, "summary-long", f.Line("summary"), fmt.Sprintf("Summary is %d characters", utf8.RuneCountInString(f.Summary)),
			"Shorten it to one sentence under 160 characters; move detail into the first section")
	}
	if f.Icon != "" && strings.HasPrefix(f.Icon, "assets/") && !strings.Contains(f.Icon, "_fill:") {
		c.referenced[f.Icon] = true
		if !c.p.HasAsset(f.Icon) {
			c.docAdd(d, "icon-missing", f.Line("icon"), fmt.Sprintf("Icon %s does not exist", f.Icon),
				"Add the file under assets/, or point icon: at an existing file")
		}
	}
	for _, r := range f.Related {
		if _, ok := c.p.DocBySlug[r]; !ok {
			c.docAdd(d, "related-missing", f.Line("related"), fmt.Sprintf("related lists %q, which has no document", r),
				"Use the slug of an existing document, or remove the entry")
		}
	}
	for _, a := range f.Aliases {
		if other, ok := c.p.DocBySlug[a]; ok {
			c.docAdd(d, "alias-duplicate", f.Line("aliases"), fmt.Sprintf("Alias %q is the slug of %s", a, other.Path),
				"Remove the alias; an alias must be a former slug that no document uses now")
			continue
		}
		for _, o := range c.p.Docs {
			if o != d && contains(o.Front.Aliases, a) && o.Name < d.Name {
				c.docAdd(d, "alias-duplicate", f.Line("aliases"), fmt.Sprintf("Alias %q is also an alias of %s", a, o.Path),
					"Keep the alias on one document only")
			}
		}
	}
	switch f.Type {
	case "dashboard":
		if f.Target == "" || f.DashboardID == "" || strings.Contains(f.Target+f.DashboardID, "_fill:") {
			return
		}
		if _, ok := c.dashboard(f.Target, f.DashboardID); !ok {
			c.docAdd(d, "dashboard-missing", f.Line("dashboard_id"),
				fmt.Sprintf("No dashboard %q in data/dashboards/%s.yaml", f.DashboardID, f.Target),
				fmt.Sprintf("Add it to data/dashboards/%s.yaml, or use one of: %s", f.Target, c.dashboardIDs(f.Target)))
		}
	case "release":
		if f.Version == "" || strings.Contains(f.Version, "_fill:") {
			return
		}
		if _, ok := c.release(f.Version); !ok {
			c.docAdd(d, "release-missing", f.Line("version"), fmt.Sprintf("Version %s is not in data/releases.yaml", f.Version),
				"Add the release to data/releases.yaml first, or correct the version")
		}
	}
}

func (c *checker) dashboard(target, id string) (project.Dashboard, bool) {
	for _, b := range c.p.Dashboards[target] {
		if b.ID == id {
			return b, true
		}
	}
	return project.Dashboard{}, false
}

func (c *checker) dashboardIDs(target string) string {
	var ids []string
	for _, b := range c.p.Dashboards[target] {
		ids = append(ids, b.ID)
	}
	if len(ids) == 0 {
		return "(none: the file is missing or empty)"
	}
	return strings.Join(ids, ", ")
}

func (c *checker) release(version string) (project.Release, bool) {
	for _, r := range c.p.Releases {
		if r.Version == version {
			return r, true
		}
	}
	return project.Release{}, false
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
