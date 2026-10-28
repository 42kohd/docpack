package schema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var jsonFence = regexp.MustCompile("(?s)```json\n(.*?)```")

// TestSpecExamplesValidate validates every block example in SPEC.md B3.1 to
// B3.12, and the document example in B3, against the published schemas.
func TestSpecExamplesValidate(t *testing.T) {
	v, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	spec, err := os.ReadFile(filepath.Join("..", "..", "SPEC.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, partB3, _ := strings.Cut(string(spec), "## B3. Documents")
	partB3, _, _ = strings.Cut(partB3, "## B4.")
	sections := regexp.MustCompile(`(?m)^### `).Split(partB3, -1)
	checked := 0
	for i, sec := range sections {
		m := jsonFence.FindStringSubmatch(sec)
		if m == nil {
			continue
		}
		var val any
		if err := json.Unmarshal([]byte(m[1]), &val); err != nil {
			t.Errorf("SPEC B3 example %d is not JSON: %v", i, err)
			continue
		}
		name := Document
		switch {
		case strings.HasPrefix(sec, "B3.0"):
			for _, run := range val.([]any) {
				if p := v.Validate("text-run", run); len(p) > 0 {
					t.Errorf("B3.0 run %v: %v", run, p)
				}
			}
			checked++
			continue
		case strings.HasPrefix(sec, "B3."):
			name = "block"
		}
		if p := v.Validate(name, val); len(p) > 0 {
			t.Errorf("SPEC example %q fails %s: %v", firstLine(sec), name, p)
		}
		checked++
	}
	if checked != 14 {
		t.Errorf("checked %d SPEC examples, want 14 (document, runs and twelve blocks)", checked)
	}
}

func firstLine(s string) string {
	l, _, _ := strings.Cut(s, "\n")
	return l
}

func TestRejectsUnknownBlockFields(t *testing.T) {
	v, _ := Load()
	bad := map[string]any{"type": "paragraph", "text": []any{map[string]any{"t": "x"}}, "style": "red"}
	if len(v.Validate("block", bad)) == 0 {
		t.Error("block with an unknown property must fail")
	}
	cmd := map[string]any{"type": "command", "caption": "c", "variants": map[string]any{
		"windows": map[string]any{"command": "a"}, "linux": map[string]any{"none": "n"}}}
	if len(v.Validate("block", cmd)) == 0 {
		t.Error("command without macos must fail")
	}
}
