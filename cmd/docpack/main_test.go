package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runCLI(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := run(args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestCheckExample(t *testing.T) {
	out := t.TempDir()
	code, stdout, stderr := runCLI("check", "../../example", "--warnings-as-errors", "--out", out)
	if code != 0 {
		t.Fatalf("exit %d\n%s%s", code, stdout, stderr)
	}
	if !strings.HasPrefix(stdout, "ok  lf 1.4.0: 19 documents, 0 warnings -> ") {
		t.Errorf("summary: %q", stdout)
	}
	code, stdout, _ = runCLI("check", "--format=json", "../../example", "--out", out)
	var diags []any
	if code != 0 || json.Unmarshal([]byte(stdout), &diags) != nil || len(diags) != 0 {
		t.Errorf("json output: exit %d, %q", code, stdout)
	}
	code, stdout, _ = runCLI("pack", "../../example", "--out", out)
	if code != 0 || strings.TrimSpace(stdout) != filepath.Join(out, "lf-1.4.0.zip") {
		t.Errorf("pack: exit %d, %q", code, stdout)
	}
}

func TestNewThenCheckFails(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "product.yaml"), []byte("name: Demo\nshort: dm\nbinary: demo\nversion: 1.0.0\nsupport_email: a@example.com\nnav:\n  - group: Start\n    items: [first-steps]\n"), 0o644)
	code, stdout, stderr := runCLI("new", "howto", "first-steps", "--dir", dir)
	if code != 0 {
		t.Fatalf("new: exit %d %s", code, stderr)
	}
	if !strings.Contains(stdout, "first-steps.md") {
		t.Errorf("new output: %q", stdout)
	}
	src, _ := os.ReadFile(filepath.Join(dir, "docs", "first-steps.md"))
	if !strings.Contains(string(src), "\nslug: first-steps\n") {
		t.Error("new did not set the slug")
	}
	if code, _, _ := runCLI("new", "howto", "first-steps", "--dir", dir); code != 1 {
		t.Error("new must refuse to overwrite")
	}
	code, stdout, _ = runCLI("check", dir, "--format", "json")
	if code != 1 {
		t.Fatalf("check of a fresh skeleton: exit %d", code)
	}
	var diags []map[string]any
	json.Unmarshal([]byte(stdout), &diags)
	for _, d := range diags {
		if d["rule"] != "fill-left" {
			t.Errorf("unexpected diagnostic for a fresh skeleton: %v", d)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "dist")); !os.IsNotExist(err) {
		t.Error("dist/ written despite errors")
	}
}

func TestUsageErrors(t *testing.T) {
	for _, args := range [][]string{{}, {"frobnicate"}, {"check", "--format", "xml"}, {"check", "--bogus"}, {"new", "essay", "x"}, {"new", "howto", "Bad Slug"}} {
		if code, _, _ := runCLI(args...); code != 2 {
			t.Errorf("%v: exit %d, want 2", args, code)
		}
	}
	if code, stdout, _ := runCLI("version"); code != 0 || !strings.HasPrefix(stdout, "docpack ") {
		t.Errorf("version: %d %q", code, stdout)
	}
}
