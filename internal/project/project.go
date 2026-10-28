// Package project loads a docpack project directory: product.yaml, the data
// files, every document and the asset list.
package project

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/42kohd/docpack/internal/dialect"
	"github.com/42kohd/docpack/internal/report"
	"github.com/42kohd/docpack/internal/schema"
)

// Product is product.yaml.
type Product struct {
	Name         string `yaml:"name" json:"name"`
	Short        string `yaml:"short" json:"short"`
	Binary       string `yaml:"binary" json:"binary"`
	Version      string `yaml:"version" json:"version"`
	SupportEmail string `yaml:"support_email" json:"support_email"`
}

// Flag is one entry of data/flags.yaml.
type Flag struct {
	Name        string   `yaml:"name"`
	Group       string   `yaml:"group"`
	Type        string   `yaml:"type"`
	Default     any      `yaml:"default"`
	Env         string   `yaml:"env"`
	Description string   `yaml:"description"`
	Platforms   []string `yaml:"platforms"`
	Status      string   `yaml:"status"`
	ReplacedBy  string   `yaml:"replaced_by"`
	Since       string   `yaml:"since"`
}

// Field is one entry of data/fields/<dataset>.yaml.
type Field struct {
	Name        string   `yaml:"name"`
	Type        string   `yaml:"type"`
	Required    bool     `yaml:"required"`
	Description string   `yaml:"description"`
	Example     any      `yaml:"example"`
	Levels      []string `yaml:"levels"`
}

// Platform is one entry of data/platforms.yaml.
type Platform struct {
	OS           string          `yaml:"os"`
	Version      string          `yaml:"version"`
	Arch         []string        `yaml:"arch"`
	Capabilities map[string]bool `yaml:"capabilities"`
}

// Dataset is one entry of data/datasets.yaml.
type Dataset struct {
	ID           string  `yaml:"id"`
	Description  string  `yaml:"description"`
	AvgSizeKB    float64 `yaml:"avg_size_kb"`
	TypicalCount int     `yaml:"typical_count"`
}

// Panel is one panel of a dashboard.
type Panel struct {
	Title  string   `yaml:"title"`
	Query  string   `yaml:"query"`
	Fields []string `yaml:"fields"`
}

// Dashboard is one entry of data/dashboards/<target>.yaml.
type Dashboard struct {
	ID         string  `yaml:"id"`
	Title      string  `yaml:"title"`
	Purpose    string  `yaml:"purpose"`
	Panels     []Panel `yaml:"panels"`
	Screenshot string  `yaml:"screenshot"`
}

// ReleaseItem is one change in a release.
type ReleaseItem struct {
	ID   string `yaml:"id"`
	Type string `yaml:"type"`
	Text string `yaml:"text"`
}

// Advisory is one notice a reader must act on.
type Advisory struct {
	ID   string `yaml:"id"`
	Text string `yaml:"text"`
}

// Release is one entry of data/releases.yaml.
type Release struct {
	Version      string        `yaml:"version"`
	Date         string        `yaml:"date"`
	Kind         string        `yaml:"kind"`
	Items        []ReleaseItem `yaml:"items"`
	Advisories   []Advisory    `yaml:"advisories"`
	UpgradeNotes []string      `yaml:"upgrade_notes"`
}

// NavNode is one entry of product.yaml nav, as written.
type NavNode struct {
	Doc      string
	Group    string
	From     string
	Limit    int
	Children []*NavNode
	Line     int
}

// Project is a loaded project directory.
type Project struct {
	Dir     string
	Product Product
	Nav     []*NavNode
	HasNav  bool

	Flags      []Flag
	Platforms  []Platform
	Datasets   []Dataset
	Fields     map[string][]Field     // dataset id → fields
	Dashboards map[string][]Dashboard // target → dashboards
	Releases   []Release

	// Data maps each package path ("data/flags.json") to the converted YAML.
	Data map[string]any
	// DataSource maps the same package path to its YAML file ("data/flags.yaml").
	DataSource map[string]string

	Docs      []*dialect.Document
	DocBySlug map[string]*dialect.Document
	Assets    []string // project-relative paths under assets/

	Diags report.List
	// Broken lists the YAML files that failed to parse or validate, so rules
	// that depend on them stay quiet instead of cascading.
	Broken map[string]bool

	nodes map[string]*yaml.Node // YAML file → root content node, for line lookups
}

// Load reads a project. Problems are recorded in Diags; the returned error is
// only for an unreadable directory.
func Load(dir string, v *schema.Validator) (*Project, error) {
	st, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}
	p := &Project{
		Dir:        dir,
		Fields:     map[string][]Field{},
		Dashboards: map[string][]Dashboard{},
		Data:       map[string]any{},
		DataSource: map[string]string{},
		DocBySlug:  map[string]*dialect.Document{},
		nodes:      map[string]*yaml.Node{},
		Broken:     map[string]bool{},
	}
	p.loadProduct(v)
	p.loadList(v, "data/flags.yaml", schema.Flags, &p.Flags)
	p.loadList(v, "data/platforms.yaml", schema.Platforms, &p.Platforms)
	p.loadList(v, "data/datasets.yaml", schema.Datasets, &p.Datasets)
	p.loadList(v, "data/releases.yaml", schema.Releases, &p.Releases)
	for _, f := range p.glob("data/fields", ".yaml") {
		var fields []Field
		if p.loadList(v, f, schema.Fields, &fields) {
			p.Fields[strings.TrimSuffix(path.Base(f), ".yaml")] = fields
		}
	}
	for _, f := range p.glob("data/dashboards", ".yaml") {
		var ds []Dashboard
		if p.loadList(v, f, schema.Dashboard, &ds) {
			p.Dashboards[strings.TrimSuffix(path.Base(f), ".yaml")] = ds
		}
	}
	p.loadDocs()
	p.loadAssets()
	return p, nil
}

func (p *Project) abs(rel string) string { return filepath.Join(p.Dir, filepath.FromSlash(rel)) }

// glob lists files with a suffix directly inside a project-relative directory.
func (p *Project) glob(dir, suffix string) []string {
	entries, err := os.ReadDir(p.abs(dir))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), suffix) && !strings.HasPrefix(e.Name(), ".") {
			out = append(out, dir+"/"+e.Name())
		}
	}
	sort.Strings(out)
	return out
}

var yamlLineRe = regexp.MustCompile(`line ([0-9]+)`)

// readYAML parses a YAML file into its root content node. A missing file
// returns nil without a diagnostic.
func (p *Project) readYAML(rel string) (*yaml.Node, bool) {
	raw, err := os.ReadFile(p.abs(rel))
	if err != nil {
		return nil, false
	}
	var root yaml.Node
	if err := yaml.Unmarshal(raw, &root); err != nil {
		line := 0
		if m := yamlLineRe.FindStringSubmatch(err.Error()); m != nil {
			line, _ = strconv.Atoi(m[1])
		}
		p.Broken[rel] = true
		p.Diags.Add("yaml-parse", rel, line, "Not valid YAML: "+strings.TrimPrefix(err.Error(), "yaml: "),
			"Fix the indentation or quoting on that line; quote values that contain \": \" or start with a special character")
		return nil, true
	}
	if len(root.Content) == 0 {
		return &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Line: 1}, true
	}
	datesAsText(root.Content[0])
	p.nodes[rel] = root.Content[0]
	return root.Content[0], true
}

// datesAsText makes YAML timestamps plain strings, so "date: 2026-01-31"
// keeps the text the author wrote instead of becoming a time value.
func datesAsText(n *yaml.Node) {
	if n.Kind == yaml.ScalarNode && n.ShortTag() == "!!timestamp" {
		n.Tag = "!!str"
	}
	for _, c := range n.Content {
		datesAsText(c)
	}
}

// validate converts a YAML node to JSON form, checks it against a schema and
// reports problems. It returns the converted value and whether it is valid.
func (p *Project) validate(v *schema.Validator, rel, name string, node *yaml.Node) (any, bool) {
	var val any
	if err := node.Decode(&val); err != nil {
		p.Diags.Add("yaml-parse", rel, node.Line, "Cannot read the file: "+err.Error(), "Check the file against its schema in schemas/")
		return nil, false
	}
	if val == nil {
		val = []any{}
	}
	var problems []schema.Problem
	for _, pr := range v.Validate(name, val) {
		// A value still holding a _fill: marker is reported once, as
		// fill-left, instead of also failing its pattern.
		if s, ok := valueAt(val, pr.Pointer).(string); ok && strings.Contains(s, "_fill:") {
			continue
		}
		problems = append(problems, pr)
	}
	if len(problems) > 0 {
		p.Broken[rel] = true
	}
	for _, pr := range problems {
		p.Diags.Add("data-schema", rel, p.Line(rel, pr.Pointer), describe(pr),
			fmt.Sprintf("Correct the value to match schemas/%s%s.schema.json", schema.Dir, name))
	}
	inst, _ := schema.Instance(val)
	return inst, len(problems) == 0
}

// valueAt follows a JSON pointer into a decoded YAML value.
func valueAt(v any, pointer string) any {
	for _, tok := range strings.Split(strings.TrimPrefix(pointer, "/"), "/") {
		if tok == "" {
			continue
		}
		tok = strings.ReplaceAll(strings.ReplaceAll(tok, "~1", "/"), "~0", "~")
		switch x := v.(type) {
		case map[string]any:
			v = x[tok]
		case []any:
			i, err := strconv.Atoi(tok)
			if err != nil || i >= len(x) {
				return nil
			}
			v = x[i]
		default:
			return nil
		}
	}
	return v
}

func describe(pr schema.Problem) string {
	where := strings.TrimPrefix(pr.Pointer, "/")
	if where == "" {
		return pr.Message
	}
	return fmt.Sprintf("At %s: %s", strings.ReplaceAll(where, "/", "."), pr.Message)
}

// loadList loads a data file that is a YAML list into out. It returns false
// when the file is absent or invalid.
func (p *Project) loadList(v *schema.Validator, rel, name string, out any) bool {
	node, ok := p.readYAML(rel)
	if node == nil {
		return false
	}
	_ = ok
	inst, valid := p.validate(v, rel, name, node)
	if !valid {
		return false
	}
	if err := node.Decode(out); err != nil && node.Kind == yaml.SequenceNode {
		p.Diags.Add("data-schema", rel, node.Line, err.Error(), "Check the file against its schema in schemas/")
		return false
	}
	pkg := strings.TrimSuffix(rel, ".yaml") + ".json"
	p.Data[pkg] = inst
	p.DataSource[pkg] = rel
	return true
}

func (p *Project) loadProduct(v *schema.Validator) {
	const rel = "product.yaml"
	node, exists := p.readYAML(rel)
	if !exists {
		p.Diags.Add("data-missing", rel, 0, "The project has no product.yaml",
			"Create product.yaml with name, short, binary, version, support_email and nav")
		return
	}
	if node == nil {
		return
	}
	if node.Kind != yaml.MappingNode {
		p.Diags.Add("data-schema", rel, node.Line, "product.yaml is not a mapping",
			"Write one key: value per line: name, short, binary, version, support_email, nav")
		return
	}
	if _, valid := p.validate(v, rel, schema.Product, node); !valid {
		return
	}
	_ = node.Decode(&p.Product)
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == "nav" {
			p.HasNav = true
			for _, g := range node.Content[i+1].Content {
				p.Nav = append(p.Nav, parseNav(g))
			}
		}
	}
}

func parseNav(n *yaml.Node) *NavNode {
	out := &NavNode{Line: n.Line}
	if n.Kind == yaml.ScalarNode {
		out.Doc = n.Value
		return out
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i].Value, n.Content[i+1]
		switch k {
		case "doc":
			out.Doc = v.Value
		case "group":
			out.Group = v.Value
		case "from":
			out.From = v.Value
		case "limit":
			out.Limit, _ = strconv.Atoi(v.Value)
		case "items", "children":
			for _, c := range v.Content {
				out.Children = append(out.Children, parseNav(c))
			}
		}
	}
	return out
}

func (p *Project) loadDocs() {
	root := p.abs("docs")
	_ = filepath.WalkDir(root, func(fp string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		rel, _ := filepath.Rel(p.Dir, fp)
		rel = filepath.ToSlash(rel)
		if filepath.Dir(fp) != root {
			p.Diags.Add("doc-nested", rel, 0, "Document is in a subdirectory of docs/",
				fmt.Sprintf("Move it to docs/%s; documents are flat and grouped by nav in product.yaml", d.Name()))
			return nil
		}
		src, err := os.ReadFile(fp)
		if err != nil {
			return nil
		}
		doc := dialect.Parse(rel, src)
		p.Docs = append(p.Docs, doc)
		p.DocBySlug[doc.Name] = doc
		return nil
	})
	sort.Slice(p.Docs, func(i, j int) bool { return p.Docs[i].Name < p.Docs[j].Name })
	if len(p.Docs) == 0 {
		p.Diags.Add("data-missing", "docs", 0, "The project has no documents in docs/",
			"Create one with: docpack new landing index")
	}
}

func (p *Project) loadAssets() {
	_ = filepath.WalkDir(p.abs("assets"), func(fp string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		rel, _ := filepath.Rel(p.Dir, fp)
		p.Assets = append(p.Assets, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(p.Assets)
}

// HasAsset reports whether a project-relative asset path exists.
func (p *Project) HasAsset(rel string) bool {
	i := sort.SearchStrings(p.Assets, rel)
	return i < len(p.Assets) && p.Assets[i] == rel
}

// Line maps a JSON pointer into a YAML file to a source line (0 if unknown).
func (p *Project) Line(rel, pointer string) int {
	n := p.nodes[rel]
	if n == nil {
		return 0
	}
	line := n.Line
	for _, tok := range strings.Split(strings.TrimPrefix(pointer, "/"), "/") {
		if tok == "" {
			continue
		}
		tok = strings.ReplaceAll(strings.ReplaceAll(tok, "~1", "/"), "~0", "~")
		var next *yaml.Node
		switch n.Kind {
		case yaml.SequenceNode:
			if i, err := strconv.Atoi(tok); err == nil && i < len(n.Content) {
				next = n.Content[i]
			}
		case yaml.MappingNode:
			for i := 0; i+1 < len(n.Content); i += 2 {
				if n.Content[i].Value == tok {
					line = n.Content[i].Line
					next = n.Content[i+1]
				}
			}
		}
		if next == nil {
			return line
		}
		n = next
		line = n.Line
	}
	return line
}

// AllDiags returns project and document diagnostics, sorted.
func (p *Project) AllDiags() report.List {
	out := append(report.List{}, p.Diags...)
	for _, d := range p.Docs {
		out = append(out, d.Diags...)
	}
	out.Sort()
	dedup := out[:0]
	for i, d := range out {
		if i > 0 && d == out[i-1] {
			continue
		}
		dedup = append(dedup, d)
	}
	return dedup
}

// DisplayTitle is the navigation title of a document.
func DisplayTitle(d *dialect.Document) string {
	if d.Front.NavTitle != "" {
		return d.Front.NavTitle
	}
	if d.Front.Title != "" {
		return d.Front.Title
	}
	return d.Name
}

// CompareVersions compares MAJOR.MINOR.PATCH strings numerically.
func CompareVersions(a, b string) int {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}
