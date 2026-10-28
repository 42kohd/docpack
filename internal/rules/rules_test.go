package rules_test

import (
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/42kohd/docpack/internal/compile"
	"github.com/42kohd/docpack/internal/report"
)

var update = flag.Bool("update", false, "rewrite the golden files in testdata/")

var testdata = filepath.Join("..", "..", "testdata")

// copyDir copies a directory tree.
func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func golden(t *testing.T, path string, got []string) {
	t.Helper()
	text := strings.Join(got, "\n")
	if text != "" {
		text += "\n"
	}
	if *update {
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing golden file %s (run go test ./internal/rules -update): %v", path, err)
	}
	if string(want) != text {
		t.Errorf("diagnostics differ from %s\n--- want\n%s--- got\n%s", path, want, text)
	}
}

func load(t *testing.T, dir string) report.List {
	t.Helper()
	_, diags, err := compile.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	return diags
}

func TestBaseProjectIsClean(t *testing.T) {
	if diags := load(t, filepath.Join(testdata, "base")); len(diags) > 0 {
		var sb strings.Builder
		diags.WriteText(&sb)
		t.Fatalf("testdata/base must pass cleanly:\n%s", sb.String())
	}
}

// TestDocumentRules checks each testdata/docs/<case>.md inside a copy of the
// base project and compares that file's diagnostics with <case>.golden.
// nav-unreachable is left out: fixtures are never in nav, and the rule has
// its own project fixture.
func TestDocumentRules(t *testing.T) {
	cases, _ := filepath.Glob(filepath.Join(testdata, "docs", "*.md"))
	if len(cases) == 0 {
		t.Fatal("no fixtures in testdata/docs")
	}
	for _, c := range cases {
		name := strings.TrimSuffix(filepath.Base(c), ".md")
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			copyDir(t, filepath.Join(testdata, "base"), dir)
			src, err := os.ReadFile(c)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "docs", name+".md"), src, 0o644); err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, d := range load(t, dir) {
				if d.File == "docs/"+name+".md" && d.Rule != "nav-unreachable" {
					got = append(got, d.String())
				}
			}
			golden(t, strings.TrimSuffix(c, ".md")+".golden", got)
		})
	}
}

// TestProjectRules overlays each testdata/projects/<case>/ on the base
// project (deleting the paths in remove.txt) and compares every diagnostic
// with expected.golden.
func TestProjectRules(t *testing.T) {
	dirs, _ := os.ReadDir(filepath.Join(testdata, "projects"))
	for _, e := range dirs {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fixture := filepath.Join(testdata, "projects", name)
			dir := t.TempDir()
			copyDir(t, filepath.Join(testdata, "base"), dir)
			copyDir(t, fixture, dir)
			os.Remove(filepath.Join(dir, "expected.golden"))
			if rm, err := os.ReadFile(filepath.Join(fixture, "remove.txt")); err == nil {
				os.Remove(filepath.Join(dir, "remove.txt"))
				for _, p := range strings.Fields(string(rm)) {
					os.RemoveAll(filepath.Join(dir, filepath.FromSlash(p)))
				}
			}
			var got []string
			for _, d := range load(t, dir) {
				got = append(got, d.String())
			}
			golden(t, filepath.Join(fixture, "expected.golden"), got)
		})
	}
}

var ruleColumn = regexp.MustCompile(`(?m)^\S+:\d+  ([a-z0-9-]+)  `)

func goldenFiles(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	docs, _ := filepath.Glob(filepath.Join(testdata, "docs", "*.golden"))
	projects, _ := filepath.Glob(filepath.Join(testdata, "projects", "*", "expected.golden"))
	for _, f := range append(docs, projects...) {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		out[f] = string(b)
	}
	return out
}

// untestable rules cannot be triggered from a fixture.
var untestable = map[string]bool{
	"schema-document": true, // fires only if the checker itself is wrong
}

func TestEveryRuleHasAFixture(t *testing.T) {
	seen := map[string]bool{}
	for _, text := range goldenFiles(t) {
		for _, m := range ruleColumn.FindAllStringSubmatch(text, -1) {
			seen[m[1]] = true
		}
	}
	var missing []string
	for id := range report.Rules {
		if !seen[id] && !untestable[id] {
			missing = append(missing, id)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("rules with no golden fixture: %s", strings.Join(missing, ", "))
	}
}

// TestFixturesTestTheirRule makes sure a fixture named after a rule actually
// produces that rule, and that good-* fixtures produce nothing.
func TestFixturesTestTheirRule(t *testing.T) {
	for path, text := range goldenFiles(t) {
		name := strings.TrimSuffix(filepath.Base(path), ".golden")
		if name == "expected" {
			name = filepath.Base(filepath.Dir(path))
		}
		if strings.HasPrefix(name, "good-") {
			if text != "" {
				t.Errorf("%s: a good fixture must produce no diagnostics, got:\n%s", path, text)
			}
			continue
		}
		if _, ok := report.Rules[name]; !ok {
			t.Errorf("%s: fixture is not named after a rule id", path)
			continue
		}
		if !strings.Contains(text, "  "+name+"  ") {
			t.Errorf("%s: fixture does not produce its own rule %s", path, name)
		}
	}
}

var specRow = regexp.MustCompile("(?m)^\\| `([a-z0-9-]+)` \\| (error|warning) \\|")

// TestRulesMatchSpec keeps the registry and SPEC.md part C2 identical.
func TestRulesMatchSpec(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("..", "..", "SPEC.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, table, ok := strings.Cut(string(spec), "## C2. Rule ids")
	if !ok {
		t.Fatal("SPEC.md has no C2 section")
	}
	inSpec := map[string]string{}
	for _, m := range specRow.FindAllStringSubmatch(table, -1) {
		inSpec[m[1]] = m[2]
	}
	for id, r := range report.Rules {
		sev, ok := inSpec[id]
		if !ok {
			t.Errorf("rule %s is registered but not in SPEC.md C2", id)
			continue
		}
		if sev != r.Severity.String() {
			t.Errorf("rule %s: SPEC says %s, registry says %s", id, sev, r.Severity)
		}
	}
	for id := range inSpec {
		if _, ok := report.Rules[id]; !ok {
			t.Errorf("rule %s is in SPEC.md C2 but not registered", id)
		}
	}
}
