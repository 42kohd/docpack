package kit

import (
	"fmt"
	"regexp"
	"strings"
)

var shortRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// emptyList is the body of a data file that starts with no entries.
const emptyList = "# %s See SPEC.md section A6 for the fields of each entry.\n[]\n"

// Init returns the files of a new project, keyed by slash-separated path.
// Every value the author must supply is a _fill: marker, so docpack check
// reports exactly those until they are replaced.
func Init(short string) (map[string]string, error) {
	if !shortRe.MatchString(short) {
		return nil, fmt.Errorf("short name %q must be lowercase letters and digits joined by -", short)
	}
	landing, err := Skeleton("landing")
	if err != nil {
		return nil, err
	}
	landing = strings.Replace(landing, "\nslug: landing\n", "\nslug: index\n", 1)
	product := `# Product facts. Replace every _fill: value, then run docpack check.
name: "_fill: product name, as written in prose"
short: ` + short + `
binary: "_fill: executable name, for example mytool"
version: "_fill: version being documented, MAJOR.MINOR.PATCH"
support_email: "_fill: support address, for example support@example.com"
nav:
  - group: Get started
    items:
      - index
  - group: Releases
    from: releases
`
	return map[string]string{
		"product.yaml":             product,
		"data/flags.yaml":          fmt.Sprintf(emptyList, "Command-line flags."),
		"data/platforms.yaml":      fmt.Sprintf(emptyList, "Supported platforms."),
		"data/datasets.yaml":       fmt.Sprintf(emptyList, "Output datasets; give each one data/fields/<id>.yaml."),
		"data/releases.yaml":       fmt.Sprintf(emptyList, "Releases, newest first."),
		"data/fields/.gitkeep":     "",
		"data/dashboards/.gitkeep": "",
		"docs/index.md":            landing,
		"assets/.gitkeep":          "",
		".gitignore":               "dist/\n",
	}, nil
}
