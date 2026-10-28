// Package compile runs the whole check pipeline and turns a checked project
// into the package of SPEC part B: JSON documents, a manifest and a zip.
package compile

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/42kohd/docpack/internal/dialect"
	"github.com/42kohd/docpack/internal/project"
	"github.com/42kohd/docpack/internal/report"
	"github.com/42kohd/docpack/internal/rules"
	"github.com/42kohd/docpack/internal/schema"
)

// SchemaVersion is the package format version (SPEC B5).
const SchemaVersion = "1.0"

// Options control a check.
type Options struct {
	WarningsAsErrors bool
	OutDir           string // default <dir>/dist
	Generator        string // written to the manifest, for example "docpack 0.1.0"
	NoWrite          bool   // check only; do not write the zip
}

// Result is the outcome of a check.
type Result struct {
	Project *project.Project
	Diags   report.List
	ZipPath string // set when the package was written
	Files   int    // entries in the package
}

// Blocked reports whether the diagnostics stop the package.
func (r *Result) Blocked(o Options) bool { return r.Diags.Blocking(o.WarningsAsErrors) }

// Load parses and checks a project without compiling it. The returned
// project's block tree is resolved and ready to render.
func Load(dir string) (*project.Project, report.List, error) {
	v, err := schema.Load()
	if err != nil {
		return nil, nil, err
	}
	p, err := project.Load(dir, v)
	if err != nil {
		return nil, nil, err
	}
	rules.Check(p)
	return p, p.AllDiags(), nil
}

// Check runs the pipeline of SPEC C1: load, check, compile, validate, and on
// zero blocking diagnostics write dist/<short>-<version>.zip.
func Check(dir string, o Options) (*Result, error) {
	p, diags, err := Load(dir)
	if err != nil {
		return nil, err
	}
	res := &Result{Project: p, Diags: diags}
	if res.Blocked(o) {
		return res, nil
	}
	files, problems, err := Package(p, o.Generator)
	if err != nil {
		return nil, err
	}
	if len(problems) > 0 {
		res.Diags = append(res.Diags, problems...)
		res.Diags.Sort()
		return res, nil
	}
	res.Files = len(files)
	if o.NoWrite {
		return res, nil
	}
	out := o.OutDir
	if out == "" {
		out = filepath.Join(dir, "dist")
	}
	zipPath := filepath.Join(out, fmt.Sprintf("%s-%s.zip", p.Product.Short, p.Product.Version))
	if err := writeZip(zipPath, files); err != nil {
		return nil, err
	}
	res.ZipPath = zipPath
	return res, nil
}

// File is one package entry.
type File struct {
	Name string
	Data []byte
}

// Document is the JSON shape of documents/<slug>.json.
type Document struct {
	Type        string           `json:"type"`
	Slug        string           `json:"slug"`
	Title       string           `json:"title"`
	Summary     string           `json:"summary"`
	Category    string           `json:"category,omitempty"`
	Icon        string           `json:"icon,omitempty"`
	Version     string           `json:"version,omitempty"`
	Date        string           `json:"date,omitempty"`
	Target      string           `json:"target,omitempty"`
	DashboardID string           `json:"dashboard_id,omitempty"`
	Task        string           `json:"task,omitempty"`
	Since       string           `json:"since,omitempty"`
	Status      string           `json:"status,omitempty"`
	NavTitle    string           `json:"nav_title,omitempty"`
	Aliases     []string         `json:"aliases,omitempty"`
	Related     []string         `json:"related,omitempty"`
	Body        []*dialect.Block `json:"body"`
}

// DocumentJSON converts a parsed document to its package form.
func DocumentJSON(d *dialect.Document) Document {
	f := d.Front
	body := d.Body
	if body == nil {
		body = []*dialect.Block{}
	}
	return Document{f.Type, f.Slug, f.Title, f.Summary, f.Category, f.Icon, f.Version, f.Date, f.Target,
		f.DashboardID, f.Task, f.Since, f.Status, f.NavTitle, f.Aliases, f.Related, body}
}

// Manifest is manifest.json.
type Manifest struct {
	SchemaVersion string             `json:"schema_version"`
	Product       string             `json:"product"`
	Version       string             `json:"version"`
	Generator     string             `json:"generator"`
	Nav           []*project.NavOut  `json:"nav"`
	Documents     []ManifestDocument `json:"documents"`
	Assets        []string           `json:"assets"`
}

// ManifestDocument is one entry of the manifest's document list.
type ManifestDocument struct {
	Slug  string `json:"slug"`
	Type  string `json:"type"`
	Title string `json:"title"`
	Hash  string `json:"hash"`
}

func marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Package builds every package entry and validates each against its schema.
// Validation failures are returned as schema-document diagnostics.
func Package(p *project.Project, generator string) ([]File, report.List, error) {
	v, err := schema.Load()
	if err != nil {
		return nil, nil, err
	}
	var files []File
	var problems report.List
	check := func(name, file string, value any) {
		for _, pr := range v.Validate(name, value) {
			problems.Add("schema-document", file, 0, fmt.Sprintf("%s fails %s.schema.json at %q: %s", file, name, pr.Pointer, pr.Message),
				"This is a checker bug; please report it with the document attached")
		}
	}
	m := Manifest{SchemaVersion: SchemaVersion, Product: p.Product.Short, Version: p.Product.Version,
		Generator: generator, Assets: append([]string{}, p.Assets...)}
	for _, d := range p.Docs {
		dj := DocumentJSON(d)
		raw, err := marshal(dj)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", d.Path, err)
		}
		name := "documents/" + d.Name + ".json"
		check(schema.Document, d.Path, json.RawMessage(raw))
		sum := sha256.Sum256(raw)
		m.Documents = append(m.Documents, ManifestDocument{d.Name, d.Front.Type, d.Front.Title, "sha256:" + hex.EncodeToString(sum[:])})
		files = append(files, File{name, raw})
	}
	nav, _, _ := p.ResolveNav()
	if nav == nil {
		nav = []*project.NavOut{}
	}
	m.Nav = nav
	raw, err := marshal(m)
	if err != nil {
		return nil, nil, err
	}
	check(schema.Manifest, "manifest.json", json.RawMessage(raw))
	files = append(files, File{"manifest.json", raw})

	raw, err = marshal(p.Product)
	if err != nil {
		return nil, nil, err
	}
	check(schema.Product, "product.json", json.RawMessage(raw))
	files = append(files, File{"product.json", raw})

	for name, inst := range p.Data {
		raw, err := marshal(inst)
		if err != nil {
			return nil, nil, err
		}
		files = append(files, File{name, raw})
	}
	for _, a := range p.Assets {
		raw, err := os.ReadFile(filepath.Join(p.Dir, filepath.FromSlash(a)))
		if err != nil {
			return nil, nil, err
		}
		files = append(files, File{a, raw})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	return files, problems, nil
}

// zipTime is the fixed timestamp of every entry, so builds are reproducible.
var zipTime = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)

func writeZip(path string, files []File) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".docpack-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	zw := zip.NewWriter(tmp)
	for _, f := range files {
		method := zip.Deflate
		if isCompressed(f.Name) {
			method = zip.Store
		}
		w, err := zw.CreateHeader(&zip.FileHeader{Name: f.Name, Method: method, Modified: zipTime})
		if err != nil {
			tmp.Close()
			return err
		}
		if _, err := io.Copy(w, bytes.NewReader(f.Data)); err != nil {
			tmp.Close()
			return err
		}
	}
	if err := zw.Close(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func isCompressed(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".woff2":
		return true
	}
	return false
}
