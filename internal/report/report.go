// Package report holds diagnostics and their text and JSON formatting.
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// Severity says whether a diagnostic blocks the package.
type Severity int

const (
	Error Severity = iota
	Warning
)

func (s Severity) String() string {
	if s == Warning {
		return "warning"
	}
	return "error"
}

// Rule describes one rule id.
type Rule struct {
	Severity Severity
	Summary  string
}

// Diag is one diagnostic. File is relative to the project root, with forward
// slashes. Line 0 means the whole file.
type Diag struct {
	File     string   `json:"file"`
	Line     int      `json:"line"`
	Rule     string   `json:"rule"`
	Message  string   `json:"message"`
	Fix      string   `json:"fix"`
	Severity Severity `json:"-"`
}

// New builds a diagnostic for a registered rule. It panics on an unknown rule
// id, so a typo fails the first test that reaches it.
func New(rule, file string, line int, message, fix string) Diag {
	r, ok := Rules[rule]
	if !ok {
		panic("report: unregistered rule " + rule)
	}
	return Diag{File: file, Line: line, Rule: rule, Message: message, Fix: fix, Severity: r.Severity}
}

// List collects diagnostics.
type List []Diag

// Add appends a diagnostic built with New.
func (l *List) Add(rule, file string, line int, message, fix string) {
	*l = append(*l, New(rule, file, line, message, fix))
}

// Sort orders diagnostics by file, line, rule and message.
func (l List) Sort() {
	sort.SliceStable(l, func(i, j int) bool {
		a, b := l[i], l[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Rule != b.Rule {
			return a.Rule < b.Rule
		}
		return a.Message < b.Message
	})
}

// Counts returns the number of errors and warnings.
func (l List) Counts() (errors, warnings int) {
	for _, d := range l {
		if d.Severity == Warning {
			warnings++
		} else {
			errors++
		}
	}
	return
}

// Blocking reports whether the list stops the package from being written.
func (l List) Blocking(warningsAsErrors bool) bool {
	e, w := l.Counts()
	return e > 0 || (warningsAsErrors && w > 0)
}

// Line formats one diagnostic as
// "file:line  rule  message. fix".
func (d Diag) String() string {
	msg := strings.TrimSuffix(d.Message, ".")
	if d.Severity == Warning {
		msg = "warning: " + msg
	}
	return fmt.Sprintf("%s:%d  %s  %s. %s", d.File, d.Line, d.Rule, msg, d.Fix)
}

// WriteText writes one diagnostic per line.
func (l List) WriteText(w io.Writer) error {
	for _, d := range l {
		if _, err := fmt.Fprintln(w, d.String()); err != nil {
			return err
		}
	}
	return nil
}

type jsonDiag struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Rule     string `json:"rule"`
	Message  string `json:"message"`
	Fix      string `json:"fix"`
	Severity string `json:"severity"`
}

// WriteJSON writes the list as a JSON array; an empty list is "[]".
func (l List) WriteJSON(w io.Writer) error {
	out := make([]jsonDiag, 0, len(l))
	for _, d := range l {
		out = append(out, jsonDiag{d.File, d.Line, d.Rule, strings.TrimSuffix(d.Message, "."), d.Fix, d.Severity.String()})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
