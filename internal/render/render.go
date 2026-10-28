// Package render turns a checked project into HTML pages for docpack serve:
// one html/template per block type, a sidebar from nav, an on-page table of
// contents from sections, and one theme.css.
package render

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/42kohd/docpack/internal/compile"
	"github.com/42kohd/docpack/internal/dialect"
	"github.com/42kohd/docpack/internal/project"
	"github.com/42kohd/docpack/internal/report"
)

//go:embed templates/*.html templates/blocks/*.html templates/theme.css
var files embed.FS

// Page is the data of one rendered page.
type Page struct {
	Product project.Product
	Doc     compile.Document
	Nav     []*NavView
	TOC     []TOCEntry
	Diags   report.List
	Errors  int
}

// NavView is a navigation node with the current page marked.
type NavView struct {
	Title    string
	Doc      string
	Active   bool
	Children []*NavView
}

// TOCEntry is one line of the on-page table of contents.
type TOCEntry struct {
	ID, Title string
	Level     int
}

var osNames = map[string]string{"windows": "Windows", "linux": "Linux", "macos": "macOS"}

// Renderer holds the parsed templates.
type Renderer struct {
	tmpl *template.Template
	sub  *strings.Replacer
	seq  atomic.Int64
}

// New parses the templates; sub replaces placeholders in rendered text.
func New(p project.Product) (*Renderer, error) {
	r := &Renderer{sub: strings.NewReplacer(
		"{product}", p.Name, "{short}", p.Short, "{binary}", p.Binary,
		"{version}", p.Version, "{support_email}", p.SupportEmail)}
	funcs := template.FuncMap{
		"sub":    func(s string) string { return r.sub.Replace(s) },
		"oses":   func() []string { return dialect.OSes },
		"osname": func(os string) string { return osNames[os] },
		"uid":    func() string { return fmt.Sprintf("cmd%d", r.seq.Add(1)) },
		"run":    r.run,
		"cellcode": func(source string, col int) bool {
			if source == "panels" {
				return col == 1
			}
			return col == 0 && source != "releases"
		},
		"heading": func(b *dialect.Block) template.HTML {
			return template.HTML(fmt.Sprintf(`<h%d id="%s">%s<a class="anchor" href="#%s" aria-label="Link to this section">#</a></h%d>`,
				b.Level, b.ID, template.HTMLEscapeString(r.sub.Replace(b.Title)), b.ID, b.Level))
		},
		"render": func(b *dialect.Block) (template.HTML, error) {
			var buf bytes.Buffer
			if err := r.tmpl.ExecuteTemplate(&buf, "block/"+b.Type, b); err != nil {
				return "", err
			}
			return template.HTML(buf.String()), nil
		},
	}
	t, err := template.New("docpack").Funcs(funcs).ParseFS(files, "templates/*.html", "templates/blocks/*.html")
	if err != nil {
		return nil, err
	}
	r.tmpl = t
	return r, nil
}

// run renders one text run.
func (r *Renderer) run(x dialect.Run) template.HTML {
	s := template.HTMLEscapeString(r.sub.Replace(x.T))
	if x.Code {
		class := ""
		switch {
		case x.Flag:
			class = ` class="flag"`
		case x.Field:
			class = ` class="field"`
		}
		s = "<code" + class + ">" + s + "</code>"
	}
	if x.Italic {
		s = "<em>" + s + "</em>"
	}
	if x.Bold {
		s = "<strong>" + s + "</strong>"
	}
	if x.Link != "" {
		href := x.Link
		if slug, sec, ok := dialect.ParseDocLink(x.Link); ok {
			href = "/" + slug
			if sec != "" {
				href += "#" + sec
			}
		}
		s = `<a href="` + template.HTMLEscapeString(href) + `">` + s + "</a>"
	}
	return template.HTML(s)
}

// Page renders one document.
func (r *Renderer) Page(p *project.Project, d *dialect.Document, nav []*project.NavOut, diags report.List) ([]byte, error) {
	page := Page{Product: p.Product, Doc: compile.DocumentJSON(d), Nav: navView(nav, d.Name), Diags: diags}
	page.Errors, _ = diags.Counts()
	dialect.Walk(d.Body, func(b *dialect.Block) {
		if b.Type == dialect.TSection && b.Level <= 3 {
			page.TOC = append(page.TOC, TOCEntry{b.ID, b.Title, b.Level})
		}
	})
	var buf bytes.Buffer
	if err := r.tmpl.ExecuteTemplate(&buf, "layout", page); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func navView(nodes []*project.NavOut, current string) []*NavView {
	var out []*NavView
	for _, n := range nodes {
		out = append(out, &NavView{Title: n.Title, Doc: n.Doc, Active: n.Doc == current, Children: navView(n.Children, current)})
	}
	return out
}

// Theme returns theme.css.
func Theme() []byte {
	b, _ := files.ReadFile("templates/theme.css")
	return b
}

// Handler serves a project directory. Every request re-reads the project, so
// edits show on reload; check problems are listed at the top of each page.
func Handler(dir string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/theme.css", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Write(Theme())
	})
	mux.HandleFunc("/assets/", func(w http.ResponseWriter, req *http.Request) {
		rel := path.Clean(strings.TrimPrefix(req.URL.Path, "/"))
		if !strings.HasPrefix(rel, "assets/") {
			http.NotFound(w, req)
			return
		}
		http.ServeFile(w, req, filepath.Join(dir, filepath.FromSlash(rel)))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		p, diags, err := compile.Load(dir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		nav, _, _ := p.ResolveNav()
		slug := strings.Trim(req.URL.Path, "/")
		if slug == "" {
			if first := firstDoc(nav); first != "" {
				http.Redirect(w, req, "/"+first, http.StatusFound)
				return
			}
			if len(p.Docs) > 0 {
				http.Redirect(w, req, "/"+p.Docs[0].Name, http.StatusFound)
				return
			}
			http.Error(w, "no documents in docs/", http.StatusNotFound)
			return
		}
		d, ok := p.DocBySlug[slug]
		if !ok {
			http.NotFound(w, req)
			return
		}
		r, err := New(p.Product)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		html, err := r.Page(p, d, nav, diags)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(html)
	})
	return mux
}

func firstDoc(nodes []*project.NavOut) string {
	for _, n := range nodes {
		if n.Doc != "" {
			return n.Doc
		}
		if d := firstDoc(n.Children); d != "" {
			return d
		}
	}
	return ""
}
