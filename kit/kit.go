// Package kit embeds the document skeletons that docpack new writes.
package kit

import (
	"embed"
	"fmt"
)

//go:embed skeletons/*.md
var skeletons embed.FS

// Skeleton returns the skeleton document for a type.
func Skeleton(docType string) (string, error) {
	b, err := skeletons.ReadFile("skeletons/" + docType + ".md")
	if err != nil {
		return "", fmt.Errorf("no skeleton for type %q", docType)
	}
	return string(b), nil
}
