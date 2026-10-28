package report

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestFormatting(t *testing.T) {
	var l List
	l.Add("flag-unknown", "docs/b.md", 3, "Flag --x is not in data/flags.yaml.", "Check the spelling")
	l.Add("cmd-missing-variant", "docs/a.md", 9, "Command \"Run\" has no macos: line", "Add macos: <command>")
	l.Sort()
	var buf bytes.Buffer
	l.WriteText(&buf)
	want := "docs/a.md:9  cmd-missing-variant  Command \"Run\" has no macos: line. Add macos: <command>\n" +
		"docs/b.md:3  flag-unknown  warning: Flag --x is not in data/flags.yaml. Check the spelling\n"
	if buf.String() != want {
		t.Errorf("text:\n%s\nwant:\n%s", buf.String(), want)
	}
	buf.Reset()
	l.WriteJSON(&buf)
	var out []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || out[1]["severity"] != "warning" || out[0]["line"] != float64(9) || out[1]["message"] != "Flag --x is not in data/flags.yaml" {
		t.Errorf("json: %s", buf.String())
	}
	if e, w := l.Counts(); e != 1 || w != 1 {
		t.Errorf("counts = %d, %d", e, w)
	}
	if !l[1:].Blocking(true) || l[1:].Blocking(false) {
		t.Error("a warning blocks only with --warnings-as-errors")
	}
}

func TestEmptyJSONIsArray(t *testing.T) {
	var buf bytes.Buffer
	List{}.WriteJSON(&buf)
	if buf.String() != "[]\n" {
		t.Errorf("got %q", buf.String())
	}
}

func TestUnknownRulePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("New with an unregistered rule must panic")
		}
	}()
	New("no-such-rule", "f", 1, "m", "f")
}
