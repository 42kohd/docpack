package compile

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/42kohd/docpack/internal/report"
	"github.com/42kohd/docpack/internal/schema"
	"github.com/42kohd/docpack/kit"
)

var root = filepath.Join("..", "..")

func diagText(l report.List) string {
	var sb strings.Builder
	l.WriteText(&sb)
	return sb.String()
}

// TestExamplePasses is the same gate CI runs: docpack check example/
// --warnings-as-errors.
func TestExamplePasses(t *testing.T) {
	out := t.TempDir()
	res, err := Check(filepath.Join(root, "example"), Options{WarningsAsErrors: true, OutDir: out, Generator: "docpack test"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Diags) > 0 {
		t.Fatalf("example/ must pass with warnings as errors:\n%s", diagText(res.Diags))
	}
	if filepath.Base(res.ZipPath) != "lf-1.4.0.zip" {
		t.Fatalf("zip path %q", res.ZipPath)
	}
	zr, err := zip.OpenReader(res.ZipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	v, _ := schema.Load()
	names := map[string]bool{}
	var manifest Manifest
	for _, f := range zr.File {
		names[f.Name] = true
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		var val any
		switch {
		case f.Name == "manifest.json":
			json.Unmarshal(b, &manifest)
			json.Unmarshal(b, &val)
			if p := v.Validate(schema.Manifest, val); len(p) > 0 {
				t.Errorf("manifest: %v", p)
			}
		case strings.HasPrefix(f.Name, "documents/"):
			json.Unmarshal(b, &val)
			if p := v.Validate(schema.Document, val); len(p) > 0 {
				t.Errorf("%s: %v", f.Name, p)
			}
		case strings.HasPrefix(f.Name, "data/fields/"):
			json.Unmarshal(b, &val)
			if p := v.Validate(schema.Fields, val); len(p) > 0 {
				t.Errorf("%s: %v", f.Name, p)
			}
		}
	}
	for _, want := range []string{"manifest.json", "product.json", "documents/index.json", "data/flags.json",
		"data/fields/records.json", "data/dashboards/warehouse-a.json", "assets/icons/warehouse-a.svg"} {
		if !names[want] {
			t.Errorf("package lacks %s", want)
		}
	}
	if len(manifest.Documents) != 19 || manifest.SchemaVersion != SchemaVersion || len(manifest.Nav) != 6 {
		t.Errorf("manifest: %d documents, schema %s, %d nav groups", len(manifest.Documents), manifest.SchemaVersion, len(manifest.Nav))
	}
}

func TestPackageIsReproducible(t *testing.T) {
	build := func() []byte {
		res, err := Check(filepath.Join(root, "example"), Options{OutDir: t.TempDir(), Generator: "docpack test"})
		if err != nil || res.ZipPath == "" {
			t.Fatalf("build failed: %v %s", err, diagText(res.Diags))
		}
		b, _ := os.ReadFile(res.ZipPath)
		return b
	}
	if !bytes.Equal(build(), build()) {
		t.Error("two builds of the same input produced different zips")
	}
}

func TestNothingWrittenOnError(t *testing.T) {
	dir := t.TempDir()
	copyTree(t, filepath.Join(root, "example"), dir)
	os.WriteFile(filepath.Join(dir, "docs", "broken.md"), []byte("# no frontmatter\n"), 0o644)
	out := filepath.Join(dir, "dist")
	res, err := Check(dir, Options{OutDir: out})
	if err != nil {
		t.Fatal(err)
	}
	if res.ZipPath != "" {
		t.Error("a zip was written despite errors")
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Error("dist/ was created despite errors")
	}
}

// TestKitExamplesMatchExample keeps kit/examples identical to the example
// project, so they pass the check by construction.
func TestKitExamplesMatchExample(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join(root, "kit", "examples", "*.md"))
	if len(files) != 3 {
		t.Fatalf("want 3 kit examples, got %d", len(files))
	}
	types := map[string]bool{}
	for _, f := range files {
		a, _ := os.ReadFile(f)
		b, err := os.ReadFile(filepath.Join(root, "example", "docs", filepath.Base(f)))
		if err != nil || !bytes.Equal(a, b) {
			t.Errorf("kit/examples/%s differs from example/docs/", filepath.Base(f))
		}
		for _, line := range strings.Split(string(a), "\n") {
			if t, ok := strings.CutPrefix(line, "type: "); ok {
				types[t] = true
				break
			}
		}
	}
	for _, want := range []string{"integration", "howto", "format"} {
		if !types[want] {
			t.Errorf("no kit example of type %s", want)
		}
	}
}

// TestSkeletonsOnlyLackContent: an untouched skeleton fails only because of
// its _fill: markers (and because nothing links to it yet).
func TestSkeletonsOnlyLackContent(t *testing.T) {
	for _, typ := range []string{"landing", "concept", "howto", "integration", "format", "reference", "dashboard", "release"} {
		t.Run(typ, func(t *testing.T) {
			dir := t.TempDir()
			copyTree(t, filepath.Join(root, "testdata", "base"), dir)
			src, err := kit.Skeleton(typ)
			if err != nil {
				t.Fatal(err)
			}
			os.WriteFile(filepath.Join(dir, "docs", typ+".md"), []byte(src), 0o644)
			_, diags, err := Load(dir)
			if err != nil {
				t.Fatal(err)
			}
			fills := 0
			for _, d := range diags {
				switch {
				case d.File != "docs/"+typ+".md":
					t.Errorf("skeleton disturbed another file: %s", d)
				case d.Rule == "fill-left":
					fills++
				case d.Rule != "nav-unreachable":
					t.Errorf("skeleton problem beyond _fill: %s", d)
				}
			}
			if fills == 0 {
				t.Error("skeleton has no _fill: markers")
			}
		})
	}
}

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if rel == "dist" && d.IsDir() {
			return filepath.SkipDir
		}
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
