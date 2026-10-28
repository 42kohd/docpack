package render

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/42kohd/docpack/internal/compile"
)

var example = filepath.Join("..", "..", "example")

// TestEveryExamplePageRenders renders each document of the example project
// and checks that placeholders are substituted and every block type has a
// template.
func TestEveryExamplePageRenders(t *testing.T) {
	p, diags, err := compile.Load(example)
	if err != nil || len(diags) > 0 {
		t.Fatalf("load: %v %v", err, diags)
	}
	r, err := New(p.Product)
	if err != nil {
		t.Fatal(err)
	}
	nav, _, _ := p.ResolveNav()
	seen := map[string]bool{}
	for _, d := range p.Docs {
		html, err := r.Page(p, d, nav, nil)
		if err != nil {
			t.Fatalf("%s: %v", d.Name, err)
		}
		s := string(html)
		for _, ph := range []string{"{product}", "{binary}", "{version}", "{short}", "{support_email}"} {
			if strings.Contains(s, ph) {
				t.Errorf("%s: placeholder %s not substituted", d.Name, ph)
			}
		}
		if !strings.Contains(s, `aria-current="page"`) {
			t.Errorf("%s: sidebar does not mark the current page", d.Name)
		}
		for _, class := range []string{`class="s2"`, `<p>`, `<ul>`, `class="steps"`, `<dl>`, `class="code"`, `class="command"`,
			`class="callout`, `<div class="table">`, `class="table reference"`, `class="cards"`, `class="image"`} {
			if strings.Contains(s, class) {
				seen[class] = true
			}
		}
	}
	if len(seen) != 12 {
		t.Errorf("the example renders %d of the 12 block types: %v", len(seen), seen)
	}
}

func TestHandler(t *testing.T) {
	srv := httptest.NewServer(Handler(example))
	defer srv.Close()
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	for path, want := range map[string]int{"/": 302, "/install": 200, "/theme.css": 200,
		"/assets/icons/warehouse-a.svg": 200, "/nope": 404} {
		resp, err := client.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != want {
			t.Errorf("GET %s = %d, want %d", path, resp.StatusCode, want)
		}
	}
	// Paths are cleaned (with a redirect) before routing, so nothing outside
	// assets/ is reachable.
	resp, err := http.Get(srv.URL + "/assets/../product.yaml")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("path traversal: final status %d, want 404", resp.StatusCode)
	}
}

func TestThemeHasDarkMode(t *testing.T) {
	if !strings.Contains(string(Theme()), "prefers-color-scheme: dark") {
		t.Error("theme.css has no dark mode")
	}
}
