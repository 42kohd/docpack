package dialect

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/42kohd/docpack/internal/report"
)

// Types lists the document types in SPEC order.
var Types = []string{"landing", "concept", "howto", "integration", "format", "reference", "dashboard", "release"}

// typeKeys are the keys each type requires beyond the common four.
var typeKeys = map[string][]string{
	"integration": {"category", "icon"},
	"release":     {"version", "date"},
	"dashboard":   {"target", "dashboard_id"},
	"howto":       {"task"},
}

var commonRequired = []string{"type", "slug", "title", "summary"}
var commonOptional = []string{"since", "status", "nav_title", "aliases", "related"}

// keyOwner maps a type-specific key to the type that owns it.
var keyOwner = func() map[string]string {
	m := map[string]string{}
	for t, keys := range typeKeys {
		for _, k := range keys {
			m[k] = t
		}
	}
	return m
}()

var (
	slugRe    = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	versionRe = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)
	sinceRe   = regexp.MustCompile(`^[0-9]+\.[0-9]+(\.[0-9]+)?$`)
	dateRe    = regexp.MustCompile(`^[0-9]{4}-(0[1-9]|1[0-2])-(0[1-9]|[12][0-9]|3[01])$`)
)

// Frontmatter is the parsed header of a document.
type Frontmatter struct {
	Type, Slug, Title, Summary string
	Category, Icon             string
	Version, Date              string
	Target, DashboardID        string
	Task                       string
	Since, Status, NavTitle    string
	Aliases, Related           []string
	Keys                       []string // keys present, in file order
	lines                      map[string]int
}

// Line returns the source line of a key, or 1 when the key is absent.
func (f *Frontmatter) Line(key string) int {
	if l, ok := f.lines[key]; ok {
		return l
	}
	return 1
}

// Has reports whether the key was written.
func (f *Frontmatter) Has(key string) bool {
	_, ok := f.lines[key]
	return ok
}

func (f *Frontmatter) setString(key, v string) {
	switch key {
	case "type":
		f.Type = v
	case "slug":
		f.Slug = v
	case "title":
		f.Title = v
	case "summary":
		f.Summary = v
	case "category":
		f.Category = v
	case "icon":
		f.Icon = v
	case "version":
		f.Version = v
	case "date":
		f.Date = v
	case "target":
		f.Target = v
	case "dashboard_id":
		f.DashboardID = v
	case "task":
		f.Task = v
	case "since":
		f.Since = v
	case "status":
		f.Status = v
	case "nav_title":
		f.NavTitle = v
	}
}

func (f *Frontmatter) value(key string) string {
	switch key {
	case "version":
		return f.Version
	case "date":
		return f.Date
	case "since":
		return f.Since
	case "status":
		return f.Status
	case "target":
		return f.Target
	case "icon":
		return f.Icon
	}
	return ""
}

func isListKey(k string) bool { return k == "aliases" || k == "related" }

// parseFrontmatter reads the YAML between the --- lines. offset is the file
// line of the opening ---, so YAML line n is file line n+offset.
func parseFrontmatter(path, name string, src []byte, offset int, diags *report.List) Frontmatter {
	f := Frontmatter{lines: map[string]int{}}
	var root yaml.Node
	if err := yaml.Unmarshal(src, &root); err != nil {
		diags.Add("fm-parse", path, offset+1, "Frontmatter is not valid YAML: "+yamlErr(err),
			`Write one "key: value" per line and quote values that contain ": " or start with a special character`)
		return f
	}
	if len(root.Content) == 0 || root.Content[0].Kind != yaml.MappingNode {
		diags.Add("fm-parse", path, offset+1, "Frontmatter is empty or not a mapping",
			"Add type, slug, title and summary as key: value lines between the --- lines")
		return f
	}
	m := root.Content[0]
	for i := 0; i+1 < len(m.Content); i += 2 {
		k, v := m.Content[i], m.Content[i+1]
		key := k.Value
		line := k.Line + offset
		if _, dup := f.lines[key]; dup {
			diags.Add("fm-invalid", path, line, fmt.Sprintf("Key %q appears twice", key), "Keep one of them")
			continue
		}
		f.lines[key] = line
		f.Keys = append(f.Keys, key)
		if isListKey(key) {
			if v.Kind != yaml.SequenceNode {
				diags.Add("fm-invalid", path, line, fmt.Sprintf("%s must be a list of slugs", key),
					fmt.Sprintf("Write it as %s: [slug-one, slug-two]", key))
				continue
			}
			var vals []string
			for _, it := range v.Content {
				if it.Kind != yaml.ScalarNode || !slugRe.MatchString(it.Value) {
					diags.Add("fm-invalid", path, it.Line+offset, fmt.Sprintf("%s entry %q is not a slug", key, it.Value),
						"Use the target document's slug: lowercase words joined by -")
					continue
				}
				vals = append(vals, it.Value)
			}
			if key == "aliases" {
				f.Aliases = vals
			} else {
				f.Related = vals
			}
			continue
		}
		if v.Kind != yaml.ScalarNode || v.Tag == "!!null" || strings.TrimSpace(v.Value) == "" {
			diags.Add("fm-invalid", path, line, fmt.Sprintf("%s must be a non-empty text value", key),
				fmt.Sprintf("Write it as %s: <text> on one line", key))
			continue
		}
		f.setString(key, strings.TrimSpace(v.Value))
	}
	validateFrontmatter(path, name, &f, offset, diags)
	return f
}

func validateFrontmatter(path, name string, f *Frontmatter, offset int, diags *report.List) {
	missing := func(key, hint string) {
		diags.Add("fm-missing", path, offset, fmt.Sprintf("Frontmatter has no %s", key),
			fmt.Sprintf("Add %s: %s", key, hint))
	}
	if !f.Has("type") {
		missing("type", "one of "+strings.Join(Types, ", "))
	} else if !contains(Types, f.Type) {
		diags.Add("fm-invalid", path, f.Line("type"), fmt.Sprintf("Unknown type %q", f.Type),
			"Use one of "+strings.Join(Types, ", "))
	}
	if !f.Has("slug") {
		missing("slug", name)
	} else if !slugRe.MatchString(f.Slug) {
		diags.Add("fm-invalid", path, f.Line("slug"), fmt.Sprintf("Slug %q is not lowercase words joined by -", f.Slug),
			fmt.Sprintf("Set slug: %s and name the file %s.md", name, name))
	} else if f.Slug != name {
		diags.Add("slug-mismatch", path, f.Line("slug"), fmt.Sprintf("Slug %q does not match the filename %q", f.Slug, name+".md"),
			fmt.Sprintf("Set slug: %s, or rename the file to %s.md", name, f.Slug))
	}
	if !f.Has("title") {
		missing("title", "<the page title>")
	}
	if !f.Has("summary") {
		missing("summary", "<one sentence saying what the page is for>")
	}
	known := contains(Types, f.Type)
	if known {
		for _, k := range typeKeys[f.Type] {
			if !f.Has(k) {
				missing(k, keyHint(k))
			}
		}
	}
	for _, k := range f.Keys {
		if contains(commonRequired, k) || contains(commonOptional, k) {
			continue
		}
		owner, isTypeKey := keyOwner[k]
		switch {
		case isTypeKey && known && owner != f.Type:
			diags.Add("fm-unknown-key", path, f.Line(k), fmt.Sprintf("Key %q is only allowed in %s documents", k, owner),
				fmt.Sprintf("Remove the %s: line", k))
		case !isTypeKey:
			diags.Add("fm-unknown-key", path, f.Line(k), fmt.Sprintf("Unknown frontmatter key %q", k),
				"Remove it; allowed keys are type, slug, title, summary, since, status, nav_title, aliases, related and the keys of this type")
		}
	}
	check := func(key string, ok bool, msg, fix string) {
		if f.Has(key) && !ok && !strings.Contains(f.value(key), "_fill:") {
			diags.Add("fm-invalid", path, f.Line(key), msg, fix)
		}
	}
	check("version", versionRe.MatchString(f.Version), fmt.Sprintf("version %q is not MAJOR.MINOR.PATCH", f.Version), "Write it as version: 1.2.3")
	check("date", dateRe.MatchString(f.Date), fmt.Sprintf("date %q is not YYYY-MM-DD", f.Date), "Write it as date: 2026-01-31")
	check("since", sinceRe.MatchString(f.Since), fmt.Sprintf("since %q is not a version", f.Since), "Write it as since: 1.2 or since: 1.2.0")
	check("status", f.Status == "ga" || f.Status == "prerelease", fmt.Sprintf("status %q is not ga or prerelease", f.Status), "Use status: ga or status: prerelease")
	check("target", slugRe.MatchString(f.Target), fmt.Sprintf("target %q is not lowercase words joined by -", f.Target), "Use the forwarder target id, the name of a file in data/dashboards/")
	check("icon", strings.HasPrefix(f.Icon, "assets/"), fmt.Sprintf("icon %q is not under assets/", f.Icon), "Put the icon in assets/ and write icon: assets/<path>")
}

func keyHint(k string) string {
	switch k {
	case "category":
		return "<integration group, for example Forwarders>"
	case "icon":
		return "assets/icons/<name>.svg"
	case "version":
		return "<MAJOR.MINOR.PATCH from data/releases.yaml>"
	case "date":
		return "<YYYY-MM-DD>"
	case "target":
		return "<forwarder target, a file name in data/dashboards/>"
	case "dashboard_id":
		return "<a dashboard id from data/dashboards/<target>.yaml>"
	case "task":
		return "<the task as a verb phrase>"
	}
	return "<value>"
}

func yamlErr(err error) string {
	s := err.Error()
	s = strings.TrimPrefix(s, "yaml: ")
	return s
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
