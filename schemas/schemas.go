// Package schemas embeds the published JSON Schemas so the checker validates
// against exactly the files in this directory.
package schemas

import "embed"

// BaseURL is the $id prefix every schema in this directory shares.
const BaseURL = "https://raw.githubusercontent.com/42kohd/docpack/main/schemas/1.0/"

// FS holds every *.schema.json file, keyed by its path relative to this
// directory (for example "data/flags.schema.json").
//
//go:embed *.schema.json data/*.schema.json
var FS embed.FS
