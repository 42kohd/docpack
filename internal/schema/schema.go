// Package schema validates YAML data and compiled JSON against the embedded
// schemas in the repository's schemas/<version>/ directory.
package schema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/42kohd/docpack/schemas"
)

// Names of the schemas, as paths relative to schemas/<version>/ without the
// suffix.
const (
	Manifest  = "manifest"
	Document  = "document"
	Product   = "product"
	Flags     = "data/flags"
	Fields    = "data/fields"
	Platforms = "data/platforms"
	Datasets  = "data/datasets"
	Dashboard = "data/dashboards"
	Releases  = "data/releases"
)

// Dir is the versioned schema directory, as it appears in messages.
const Dir = schemas.Version + "/"

// Problem is one validation failure. Pointer is a JSON pointer into the
// instance ("" is the root).
type Problem struct {
	Pointer string
	Message string
}

// Validator holds the compiled schemas.
type Validator struct {
	byName map[string]*jsonschema.Schema
}

var (
	once    sync.Once
	shared  *Validator
	loadErr error
)

// Load compiles every embedded schema once and returns the shared validator.
func Load() (*Validator, error) {
	once.Do(func() { shared, loadErr = load() })
	return shared, loadErr
}

func load() (*Validator, error) {
	c := jsonschema.NewCompiler()
	var names []string
	dir, err := fs.Sub(schemas.FS, schemas.Version)
	if err != nil {
		return nil, err
	}
	err = fs.WalkDir(dir, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".schema.json") {
			return err
		}
		raw, err := fs.ReadFile(dir, path)
		if err != nil {
			return err
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			return fmt.Errorf("schema %s: %w", path, err)
		}
		if err := c.AddResource(schemas.BaseURL+path, doc); err != nil {
			return fmt.Errorf("schema %s: %w", path, err)
		}
		names = append(names, strings.TrimSuffix(path, ".schema.json"))
		return nil
	})
	if err != nil {
		return nil, err
	}
	v := &Validator{byName: map[string]*jsonschema.Schema{}}
	for _, n := range names {
		s, err := c.Compile(schemas.BaseURL + n + ".schema.json")
		if err != nil {
			return nil, fmt.Errorf("schema %s: %w", n, err)
		}
		v.byName[n] = s
	}
	return v, nil
}

// Instance converts any Go value to the JSON form the validator expects.
func Instance(v any) (any, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return jsonschema.UnmarshalJSON(bytes.NewReader(raw))
}

// Validate checks a value against a named schema. It returns the specific
// problems, sorted by location, capped at 20.
func (v *Validator) Validate(name string, value any) []Problem {
	s, ok := v.byName[name]
	if !ok {
		return []Problem{{"", "no schema named " + name}}
	}
	inst, err := Instance(value)
	if err != nil {
		return []Problem{{"", err.Error()}}
	}
	err = s.Validate(inst)
	if err == nil {
		return nil
	}
	ve, ok := err.(*jsonschema.ValidationError)
	if !ok {
		return []Problem{{"", err.Error()}}
	}
	seen := map[Problem]bool{}
	var out []Problem
	var walk func(u jsonschema.OutputUnit)
	walk = func(u jsonschema.OutputUnit) {
		if len(u.Errors) == 0 && u.Error != nil {
			pr := Problem{u.InstanceLocation, u.Error.String()}
			if !seen[pr] && pr.Message != "validation failed" {
				seen[pr] = true
				out = append(out, pr)
			}
		}
		for _, e := range u.Errors {
			walk(e)
		}
	}
	walk(*ve.BasicOutput())
	sort.SliceStable(out, func(i, j int) bool { return out[i].Pointer < out[j].Pointer })
	if len(out) > 20 {
		out = out[:20]
	}
	return out
}
