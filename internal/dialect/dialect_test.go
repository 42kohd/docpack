package dialect

import (
	"strings"
	"testing"
)

func TestSectionID(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"What's new", "whats-new"},
		{"What’s new", "whats-new"},
		{"The --batch-size flag", "the-batch-size-flag"},
		{"Target configuration", "target-configuration"},
		{"  Spaces  and   tabs ", "spaces-and-tabs"},
		{"C++ / Go", "c-go"},
		{"!!!", "section"},
	} {
		if got := SectionID(tc.in); got != tc.want {
			t.Errorf("SectionID(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseDocLink(t *testing.T) {
	for _, tc := range []struct {
		in, slug, section string
		ok                bool
	}{
		{"doc:install", "install", "", true},
		{"doc:install#steps", "install", "steps", true},
		{"doc:Install", "", "", false},
		{"doc:install#", "", "", false},
		{"https://example.com", "", "", false},
	} {
		slug, sec, ok := ParseDocLink(tc.in)
		if slug != tc.slug || sec != tc.section || ok != tc.ok {
			t.Errorf("ParseDocLink(%q) = %q, %q, %v", tc.in, slug, sec, ok)
		}
	}
}

func TestCountCells(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want int
	}{
		{"| a | b |", 2},
		{"a | b", 2},
		{"| a |", 1},
		{"| `a|b` | c |", 2},
		{`| a \| b | c |`, 2},
		{"| a | b | c |", 3},
	} {
		if got := countCells(tc.in); got != tc.want {
			t.Errorf("countCells(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

const fm = "---\ntype: concept\nslug: x\ntitle: X\nsummary: S.\n---\n\n"

func parse(t *testing.T, body string) *Document {
	t.Helper()
	return Parse("docs/x.md", []byte(fm+body))
}

// TestContainerNesting covers the depth counting in the container parser: a
// command closes inside a callout inside a step, and ::: inside fenced code
// does not close anything.
func TestContainerNesting(t *testing.T) {
	d := parse(t, "## A\n\n::: steps\n### One\n\n::: warning Careful\n::: command Run\nwindows: a\nlinux: b\nmacos: c\n:::\n:::\n\n```text\n:::\n```\n\n### Two\n\nDone.\n:::\n\n## Related\n\nEnd.\n")
	if len(d.Diags) > 0 {
		t.Fatalf("unexpected diagnostics: %v", d.Diags)
	}
	if len(d.Body) != 2 {
		t.Fatalf("want 2 sections, got %d", len(d.Body))
	}
	steps := d.Body[0].Body[0]
	if steps.Type != TSteps || len(steps.Steps) != 2 {
		t.Fatalf("want steps with 2 steps, got %+v", steps)
	}
	one := steps.Steps[0].Body
	if len(one) != 2 || one[0].Type != TCallout || one[1].Type != TCode {
		t.Fatalf("step one: want callout then code, got %d blocks", len(one))
	}
	if cmd := one[0].Body[0]; cmd.Type != TCommand || cmd.Variants["macos"].Command != "c" {
		t.Errorf("command not parsed inside callout: %+v", cmd)
	}
	if one[1].Code != ":::" {
		t.Errorf("code = %q, want :::", one[1].Code)
	}
}

func TestRunsMergeAndTrim(t *testing.T) {
	d := parse(t, "## A\n\nOne **two** **three**\nfour `--x` [link](doc:y) \\*.\n")
	runs := d.Body[0].Body[0].Text
	var parts []string
	for _, r := range runs {
		parts = append(parts, r.T)
	}
	got := strings.Join(parts, "|")
	if want := "One |two| |three| four |--x| |link| *."; got != want {
		t.Errorf("runs = %q, want %q", got, want)
	}
	if !runs[1].Bold || !runs[3].Bold || runs[2].Bold {
		t.Errorf("bold flags wrong: %+v", runs)
	}
}

func TestSectionTree(t *testing.T) {
	d := parse(t, "## A\n\n### B\n\n#### C\n\n### D\n\n## A\n")
	if len(d.Body) != 2 || d.Body[1].ID != "a-2" {
		t.Fatalf("want two top sections, the second with id a-2: %+v", d.Body)
	}
	b := d.Body[0].Body
	if len(b) != 2 || b[0].Title != "B" || b[0].Body[0].Title != "C" || b[1].Title != "D" {
		t.Errorf("unexpected tree under A")
	}
	if !d.HasSection("c") {
		t.Error("section index misses nested id")
	}
}

func TestCRLFAndBOM(t *testing.T) {
	src := "\xef\xbb\xbf" + strings.ReplaceAll(fm+"## A\n\nText.\n", "\n", "\r\n")
	d := Parse("docs/x.md", []byte(src))
	if len(d.Diags) > 0 || d.Front.Slug != "x" || len(d.Body) != 1 {
		t.Fatalf("CRLF/BOM file not parsed: %v", d.Diags)
	}
}
