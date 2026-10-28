// Package schemas embeds the published JSON Schemas so the checker validates
// against exactly the files in this directory.
package schemas

import "embed"

// Version is the schema directory of the current package format.
const Version = "1.0"

// BaseURL is the $id prefix every schema of this version shares; it resolves
// to the files in schemas/1.0/ on the main branch.
const BaseURL = "https://raw.githubusercontent.com/42kohd/docpack/main/schemas/1.0/"

// FS holds every *.schema.json file of the current schema version, keyed by
// its path relative to this directory (for example
// "1.0/data/flags.schema.json"). BaseURL already includes the version.
//
//go:embed 1.0/*.schema.json 1.0/data/*.schema.json
var FS embed.FS
