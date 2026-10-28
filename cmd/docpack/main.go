// Command docpack checks product documentation written in the docpack
// dialect and compiles it into a validated package.
//
//	docpack check [dir]          check, and on success write dist/<short>-<version>.zip
//	docpack pack [dir]           the same, quiet: prints only the zip path
//	docpack new <type> <slug>    write a skeleton to docs/<slug>.md
//	docpack serve [dir]          render the project at http://127.0.0.1:8080
//	docpack version              print the version
package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/42kohd/docpack/internal/compile"
	"github.com/42kohd/docpack/internal/dialect"
	"github.com/42kohd/docpack/internal/render"
	"github.com/42kohd/docpack/kit"
)

var version = "0.1.0-dev" // set by goreleaser from the tag

const usage = `docpack checks product documentation and compiles it into a package.

Usage:
  docpack check [dir] [--format text|json] [--warnings-as-errors] [--out dir]
  docpack pack  [dir] [--warnings-as-errors] [--out dir]
  docpack new   <type> <slug> [--dir dir]
  docpack serve [dir] [--addr host:port]
  docpack version

dir defaults to the current directory. Types: landing, concept, howto,
integration, format, reference, dashboard, release.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// opts holds parsed command-line options.
type opts struct {
	args             []string
	format           string
	warningsAsErrors bool
	out              string
	dir              string
	addr             string
}

// parse accepts flags before or after positional arguments.
func parse(args []string) (opts, error) {
	o := opts{format: "text", addr: "127.0.0.1:8080"}
	valued := map[string]*string{"--format": &o.format, "--out": &o.out, "--dir": &o.dir, "--addr": &o.addr}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") || a == "-" {
			o.args = append(o.args, a)
			continue
		}
		name, val, hasVal := strings.Cut(a, "=")
		name = "--" + strings.TrimLeft(name, "-")
		if name == "--warnings-as-errors" {
			o.warningsAsErrors = true
			continue
		}
		if name == "--help" || name == "--h" {
			return o, errHelp
		}
		dst, ok := valued[name]
		if !ok {
			return o, fmt.Errorf("unknown flag %s", a)
		}
		if !hasVal {
			if i+1 >= len(args) {
				return o, fmt.Errorf("%s needs a value", name)
			}
			i++
			val = args[i]
		}
		*dst = val
	}
	if o.format != "text" && o.format != "json" {
		return o, fmt.Errorf("--format must be text or json")
	}
	return o, nil
}

var errHelp = errors.New("help")

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	cmd := args[0]
	o, err := parse(args[1:])
	if err == errHelp || cmd == "help" || cmd == "--help" || cmd == "-h" {
		fmt.Fprint(stdout, usage)
		return 0
	}
	if err != nil {
		fmt.Fprintf(stderr, "docpack %s: %v\n\n%s", cmd, err, usage)
		return 2
	}
	switch cmd {
	case "check":
		return check(o, false, stdout, stderr)
	case "pack":
		return check(o, true, stdout, stderr)
	case "new":
		return newDoc(o, stdout, stderr)
	case "serve":
		return serve(o, stdout, stderr)
	case "version":
		fmt.Fprintln(stdout, "docpack", version)
		return 0
	}
	fmt.Fprintf(stderr, "docpack: unknown command %q\n\n%s", cmd, usage)
	return 2
}

func dirArg(o opts) (string, error) {
	switch len(o.args) {
	case 0:
		return ".", nil
	case 1:
		return o.args[0], nil
	}
	return "", fmt.Errorf("expected at most one directory, got %d arguments", len(o.args))
}

func check(o opts, quiet bool, stdout, stderr io.Writer) int {
	dir, err := dirArg(o)
	if err != nil {
		fmt.Fprintln(stderr, "docpack:", err)
		return 2
	}
	res, err := compile.Check(dir, compile.Options{
		WarningsAsErrors: o.warningsAsErrors,
		OutDir:           o.out,
		Generator:        "docpack " + version,
	})
	if err != nil {
		fmt.Fprintln(stderr, "docpack:", err)
		return 2
	}
	diagOut := stdout
	if quiet {
		diagOut = stderr
	}
	if o.format == "json" {
		res.Diags.WriteJSON(diagOut)
	} else {
		res.Diags.WriteText(diagOut)
	}
	errs, warns := res.Diags.Counts()
	if res.ZipPath == "" {
		if !quiet {
			fmt.Fprintf(stderr, "docpack: %d error%s, %d warning%s; nothing written\n", errs, plural(errs), warns, plural(warns))
		}
		return 1
	}
	if quiet {
		fmt.Fprintln(stdout, res.ZipPath)
		return 0
	}
	p := res.Project
	summary := fmt.Sprintf("ok  %s %s: %d documents, %d warning%s -> %s (%d files)",
		p.Product.Short, p.Product.Version, len(p.Docs), warns, plural(warns), res.ZipPath, res.Files)
	if o.format == "json" {
		fmt.Fprintln(stderr, summary)
	} else {
		fmt.Fprintln(stdout, summary)
	}
	return 0
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

var slugRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func newDoc(o opts, stdout, stderr io.Writer) int {
	if len(o.args) != 2 {
		fmt.Fprintln(stderr, "docpack new: usage: docpack new <type> <slug>")
		return 2
	}
	typ, slug := o.args[0], o.args[1]
	known := false
	for _, t := range dialect.Types {
		known = known || t == typ
	}
	if !known {
		fmt.Fprintf(stderr, "docpack new: unknown type %q; use one of %s\n", typ, strings.Join(dialect.Types, ", "))
		return 2
	}
	if !slugRe.MatchString(slug) {
		fmt.Fprintf(stderr, "docpack new: slug %q must be lowercase words joined by -\n", slug)
		return 2
	}
	src, err := kit.Skeleton(typ)
	if err != nil {
		fmt.Fprintln(stderr, "docpack new:", err)
		return 2
	}
	src = strings.Replace(src, "\nslug: "+typ+"\n", "\nslug: "+slug+"\n", 1)
	dir := o.dir
	if dir == "" {
		dir = "."
	}
	path := filepath.Join(dir, "docs", slug+".md")
	if _, err := os.Stat(path); err == nil {
		fmt.Fprintf(stderr, "docpack new: %s already exists\n", path)
		return 1
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintln(stderr, "docpack new:", err)
		return 2
	}
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		fmt.Fprintln(stderr, "docpack new:", err)
		return 2
	}
	fmt.Fprintf(stdout, "wrote %s; replace every _fill: line, add %s to nav in product.yaml, then run docpack check\n", path, slug)
	return 0
}

func serve(o opts, stdout, stderr io.Writer) int {
	dir, err := dirArg(o)
	if err != nil {
		fmt.Fprintln(stderr, "docpack:", err)
		return 2
	}
	ln, err := net.Listen("tcp", o.addr)
	if err != nil {
		fmt.Fprintln(stderr, "docpack serve:", err)
		return 2
	}
	fmt.Fprintf(stdout, "serving %s at http://%s (Ctrl+C to stop)\n", dir, ln.Addr())
	if err := http.Serve(ln, render.Handler(dir)); err != nil {
		fmt.Fprintln(stderr, "docpack serve:", err)
		return 2
	}
	return 0
}
