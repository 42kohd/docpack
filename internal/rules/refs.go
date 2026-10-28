package rules

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/42kohd/docpack/internal/dialect"
	"github.com/42kohd/docpack/internal/project"
)

// field finds a field by name in any dataset, in dataset-id order.
func (c *checker) field(name string) (dataset string, f project.Field, ok bool) {
	for _, ds := range c.datasetIDs() {
		for _, x := range c.p.Fields[ds] {
			if x.Name == name {
				return ds, x, true
			}
		}
	}
	return "", project.Field{}, false
}

func (c *checker) datasetIDs() []string {
	ids := make([]string, 0, len(c.p.Fields))
	for id := range c.p.Fields {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (c *checker) flag(name string) (project.Flag, bool) {
	for _, f := range c.p.Flags {
		if f.Name == name {
			return f, true
		}
	}
	return project.Flag{}, false
}

func scalar(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case bool:
		return strconv.FormatBool(x)
	case int:
		return strconv.Itoa(x)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	}
	return fmt.Sprint(v)
}

// references resolves every author-written reference table in a document.
func (c *checker) references(d *dialect.Document) {
	dialect.Walk(d.Body, func(b *dialect.Block) {
		if b.Type != dialect.TReferenceTable || b.Generated {
			return
		}
		for _, v := range b.Filter {
			if strings.Contains(v, "_fill:") {
				return // reported by fill-left
			}
		}
		c.resolve(b)
		if len(b.RefRows) == 0 {
			c.docAdd(d, "ref-empty", b.Line, fmt.Sprintf("{{ %s%s }} selects no rows", b.Source, filterText(b.Filter)), c.refFix(b))
		}
	})
}

func filterText(f map[string]string) string {
	var parts []string
	for k, v := range f {
		parts = append(parts, fmt.Sprintf(" %s=%q", k, v))
	}
	sort.Strings(parts)
	return strings.Join(parts, "")
}

func (c *checker) refFix(b *dialect.Block) string {
	switch b.Source {
	case "flags":
		var groups []string
		seen := map[string]bool{}
		for _, f := range c.p.Flags {
			if !seen[f.Group] {
				seen[f.Group] = true
				groups = append(groups, f.Group)
			}
		}
		if len(groups) == 0 {
			return "Add flags to data/flags.yaml"
		}
		return "Use a group from data/flags.yaml: " + strings.Join(groups, ", ")
	case "fields":
		if len(c.p.Fields) == 0 {
			return "Add data/fields/<dataset>.yaml"
		}
		return "Use a dataset with a file in data/fields/: " + strings.Join(c.datasetIDs(), ", ")
	}
	return fmt.Sprintf("Add entries to data/%s.yaml, or remove the reference", b.Source)
}

// resolve fills Columns and RefRows from the data files (SPEC B3.10).
func (c *checker) resolve(b *dialect.Block) {
	switch b.Source {
	case "flags":
		b.Columns = []string{"Flag", "Type", "Default", "Env", "Description", "Platforms", "Status"}
		for _, f := range c.p.Flags {
			if g, ok := b.Filter["group"]; ok && f.Group != g {
				continue
			}
			b.RefRows = append(b.RefRows, []string{f.Name, f.Type, scalar(f.Default), f.Env, f.Description,
				strings.Join(f.Platforms, ", "), f.Status})
		}
	case "fields":
		b.Columns = []string{"Field", "Type", "Required", "Description", "Example"}
		for _, f := range c.p.Fields[b.Filter["dataset"]] {
			req := "no"
			if f.Required {
				req = "yes"
			}
			b.RefRows = append(b.RefRows, []string{f.Name, f.Type, req, f.Description, scalar(f.Example)})
		}
	case "platforms":
		b.Columns = []string{"Platform", "Version", "Arch", "Capabilities"}
		for _, pl := range c.p.Platforms {
			var caps []string
			for k, v := range pl.Capabilities {
				if v {
					caps = append(caps, k)
				}
			}
			sort.Strings(caps)
			b.RefRows = append(b.RefRows, []string{pl.OS, pl.Version, strings.Join(pl.Arch, ", "), strings.Join(caps, ", ")})
		}
	case "datasets":
		b.Columns = []string{"Dataset", "Description", "Avg size (KB)", "Typical count"}
		for _, ds := range c.p.Datasets {
			b.RefRows = append(b.RefRows, []string{ds.ID, ds.Description, scalar(ds.AvgSizeKB), strconv.Itoa(ds.TypicalCount)})
		}
	case "releases":
		b.Columns = []string{"Version", "Date", "Kind", "Changes"}
		rels := append([]project.Release{}, c.p.Releases...)
		sort.SliceStable(rels, func(i, j int) bool { return project.CompareVersions(rels[i].Version, rels[j].Version) > 0 })
		for i, r := range rels {
			if b.Limit > 0 && i >= b.Limit {
				break
			}
			b.RefRows = append(b.RefRows, []string{r.Version, r.Date, r.Kind, strconv.Itoa(len(r.Items))})
		}
	}
}

// data checks ids and cross-references inside the data files.
func (c *checker) data() {
	p := c.p
	add := func(file, ptr, rule, msg, fix string) {
		p.Diags.Add(rule, file, p.Line(file, ptr), msg, fix)
	}
	dup := func(file string, ids []string, what string) {
		seen := map[string]bool{}
		for i, id := range ids {
			if seen[id] {
				add(file, fmt.Sprintf("/%d", i), "data-duplicate", fmt.Sprintf("Duplicate %s %q", what, id),
					"Remove or rename one of the entries")
			}
			seen[id] = true
		}
	}
	var names, osIDs, dsIDs, versions []string
	for _, f := range p.Flags {
		names = append(names, f.Name)
	}
	for _, pl := range p.Platforms {
		osIDs = append(osIDs, pl.OS)
	}
	for _, ds := range p.Datasets {
		dsIDs = append(dsIDs, ds.ID)
	}
	for _, r := range p.Releases {
		versions = append(versions, r.Version)
	}
	dup("data/flags.yaml", names, "flag")
	dup("data/platforms.yaml", osIDs, "platform")
	dup("data/datasets.yaml", dsIDs, "dataset")
	dup("data/releases.yaml", versions, "release")

	for i, f := range p.Flags {
		if f.ReplacedBy != "" && !contains(names, f.ReplacedBy) {
			add("data/flags.yaml", fmt.Sprintf("/%d/replaced_by", i), "data-ref",
				fmt.Sprintf("%s is replaced by %s, which is not a flag", f.Name, f.ReplacedBy),
				"Add the replacement flag, or correct replaced_by")
		}
		if len(p.Platforms) == 0 {
			continue
		}
		for j, os := range f.Platforms {
			if !contains(osIDs, os) {
				add("data/flags.yaml", fmt.Sprintf("/%d/platforms/%d", i, j), "data-ref",
					fmt.Sprintf("%s lists platform %q, which is not in data/platforms.yaml", f.Name, os),
					"Use an os id from data/platforms.yaml: "+strings.Join(osIDs, ", "))
			}
		}
	}
	for _, ds := range c.datasetIDs() {
		file := "data/fields/" + ds + ".yaml"
		var fn []string
		for _, f := range p.Fields[ds] {
			fn = append(fn, f.Name)
		}
		dup(file, fn, "field")
		if !contains(dsIDs, ds) && !p.Broken["data/datasets.yaml"] {
			add(file, "", "data-ref", fmt.Sprintf("data/fields/%s.yaml has no dataset %q in data/datasets.yaml", ds, ds),
				"Add the dataset to data/datasets.yaml, or rename the file to an existing dataset id")
		}
	}
	targets := make([]string, 0, len(p.Dashboards))
	for t := range p.Dashboards {
		targets = append(targets, t)
	}
	sort.Strings(targets)
	fieldsBroken := false
	for f := range p.Broken {
		fieldsBroken = fieldsBroken || strings.HasPrefix(f, "data/fields/")
	}
	for _, t := range targets {
		file := "data/dashboards/" + t + ".yaml"
		var ids []string
		for i, b := range p.Dashboards[t] {
			ids = append(ids, b.ID)
			if b.Screenshot != "" {
				c.referenced[b.Screenshot] = true
				if !p.HasAsset(b.Screenshot) {
					add(file, fmt.Sprintf("/%d/screenshot", i), "data-ref", fmt.Sprintf("Screenshot %s does not exist", b.Screenshot),
						"Add the image under assets/, or remove the screenshot key")
				}
			}
			for j, pn := range b.Panels {
				for k, f := range pn.Fields {
					if _, _, ok := c.field(f); !ok && !fieldsBroken {
						add(file, fmt.Sprintf("/%d/panels/%d/fields/%d", i, j, k), "data-ref",
							fmt.Sprintf("Panel %q uses field %q, which no dataset defines", pn.Title, f),
							"Add the field to data/fields/<dataset>.yaml, or correct the name")
					}
				}
			}
		}
		dup(file, ids, "dashboard")
	}
}
