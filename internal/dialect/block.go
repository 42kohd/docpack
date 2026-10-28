package dialect

import (
	"encoding/json"
	"fmt"
)

// Run is one span of inline text. Line is the source line, for diagnostics
// only.
type Run struct {
	T      string `json:"t"`
	Bold   bool   `json:"bold,omitempty"`
	Italic bool   `json:"italic,omitempty"`
	Code   bool   `json:"code,omitempty"`
	Link   string `json:"link,omitempty"`
	Flag   bool   `json:"flag,omitempty"`
	Field  bool   `json:"field,omitempty"`
	Line   int    `json:"-"`
}

func (r Run) sameStyle(o Run) bool {
	return r.Bold == o.Bold && r.Italic == o.Italic && r.Code == o.Code &&
		r.Link == o.Link && r.Flag == o.Flag && r.Field == o.Field
}

// Block types, one per construct.
const (
	TSection        = "section"
	TParagraph      = "paragraph"
	TList           = "list"
	TSteps          = "steps"
	TDefinitionList = "definition_list"
	TCode           = "code"
	TCommand        = "command"
	TCallout        = "callout"
	TTable          = "table"
	TReferenceTable = "reference_table"
	TCards          = "cards"
	TImage          = "image"
)

// Block is one node of the body tree. Only the fields of its Type are used.
type Block struct {
	Type string
	Line int

	// section: ID, Title, Level, Body. callout: Kind, Title, Body.
	ID    string
	Title string
	Level int
	Kind  string
	Body  []*Block

	// paragraph
	Text []Run

	// list
	Ordered bool
	Items   []*ListItem

	// steps
	Steps []*Step

	// definition_list
	Defs []*Def

	// code
	Language string
	Code     string

	// command; Caption is also the image caption.
	Caption  string
	Variants map[string]Variant

	// table
	Header [][]Run
	Rows   [][][]Run

	// reference_table
	Source    string
	Filter    map[string]string
	Limit     int
	Columns   []string
	RefRows   [][]string
	Generated bool

	// cards
	Cards []*Card

	// image
	Src string
	Alt string
}

// ListItem is one list entry with an optional nested list.
type ListItem struct {
	Text  []Run       `json:"text"`
	Items []*ListItem `json:"items,omitempty"`
}

// Step is one step of a steps block.
type Step struct {
	Title string   `json:"title"`
	Body  []*Block `json:"body"`
	Line  int      `json:"-"`
}

// Def is one term and its definition.
type Def struct {
	Term       []Run `json:"term"`
	Definition []Run `json:"definition"`
}

// Variant is one OS tab of a command: a command line, or the reason there is
// none.
type Variant struct {
	Command string `json:"command,omitempty"`
	None    string `json:"none,omitempty"`
}

// Card is one item of a cards block.
type Card struct {
	Title   string `json:"title"`
	Doc     string `json:"doc"`
	Section string `json:"section,omitempty"`
	Text    string `json:"text"`
	Line    int    `json:"-"`
}

// OSes are the command variant keys, in display order.
var OSes = []string{"windows", "linux", "macos"}

func blocksOrEmpty(b []*Block) []*Block {
	if b == nil {
		return []*Block{}
	}
	return b
}

// MarshalJSON writes the block in the package format (SPEC part B3).
func (b *Block) MarshalJSON() ([]byte, error) {
	switch b.Type {
	case TSection:
		return json.Marshal(struct {
			Type  string   `json:"type"`
			ID    string   `json:"id"`
			Title string   `json:"title"`
			Level int      `json:"level"`
			Body  []*Block `json:"body"`
		}{b.Type, b.ID, b.Title, b.Level, blocksOrEmpty(b.Body)})
	case TParagraph:
		return json.Marshal(struct {
			Type string `json:"type"`
			Text []Run  `json:"text"`
		}{b.Type, b.Text})
	case TList:
		return json.Marshal(struct {
			Type    string      `json:"type"`
			Ordered bool        `json:"ordered"`
			Items   []*ListItem `json:"items"`
		}{b.Type, b.Ordered, b.Items})
	case TSteps:
		for _, s := range b.Steps {
			s.Body = blocksOrEmpty(s.Body)
		}
		return json.Marshal(struct {
			Type  string  `json:"type"`
			Steps []*Step `json:"steps"`
		}{b.Type, b.Steps})
	case TDefinitionList:
		return json.Marshal(struct {
			Type  string `json:"type"`
			Items []*Def `json:"items"`
		}{b.Type, b.Defs})
	case TCode:
		return json.Marshal(struct {
			Type     string `json:"type"`
			Language string `json:"language"`
			Title    string `json:"title,omitempty"`
			Code     string `json:"code"`
		}{b.Type, b.Language, b.Title, b.Code})
	case TCommand:
		return json.Marshal(struct {
			Type     string             `json:"type"`
			Caption  string             `json:"caption"`
			Variants map[string]Variant `json:"variants"`
		}{b.Type, b.Caption, b.Variants})
	case TCallout:
		return json.Marshal(struct {
			Type  string   `json:"type"`
			Kind  string   `json:"kind"`
			Title string   `json:"title"`
			Body  []*Block `json:"body"`
		}{b.Type, b.Kind, b.Title, blocksOrEmpty(b.Body)})
	case TTable:
		rows := b.Rows
		if rows == nil {
			rows = [][][]Run{}
		}
		for _, row := range rows {
			for i, cell := range row {
				if cell == nil {
					row[i] = []Run{}
				}
			}
		}
		return json.Marshal(struct {
			Type   string    `json:"type"`
			Header [][]Run   `json:"header"`
			Rows   [][][]Run `json:"rows"`
		}{b.Type, b.Header, rows})
	case TReferenceTable:
		return json.Marshal(struct {
			Type    string            `json:"type"`
			Source  string            `json:"source"`
			Filter  map[string]string `json:"filter,omitempty"`
			Limit   int               `json:"limit,omitempty"`
			Columns []string          `json:"columns"`
			Rows    [][]string        `json:"rows"`
		}{b.Type, b.Source, b.Filter, b.Limit, b.Columns, b.RefRows})
	case TCards:
		return json.Marshal(struct {
			Type  string  `json:"type"`
			Items []*Card `json:"items"`
		}{b.Type, b.Cards})
	case TImage:
		return json.Marshal(struct {
			Type    string `json:"type"`
			Src     string `json:"src"`
			Alt     string `json:"alt"`
			Caption string `json:"caption,omitempty"`
		}{b.Type, b.Src, b.Alt, b.Caption})
	}
	return nil, fmt.Errorf("dialect: cannot marshal block type %q", b.Type)
}

// Walk calls fn for every block in the tree, depth first, parents before
// children. Blocks inside steps and callouts are included.
func Walk(blocks []*Block, fn func(*Block)) {
	for _, b := range blocks {
		fn(b)
		Walk(b.Body, fn)
		for _, s := range b.Steps {
			Walk(s.Body, fn)
		}
	}
}

// RunSlices returns pointers to every run slice held by b itself (not by its
// children), so callers can inspect or rewrite inline text in one place.
func (b *Block) RunSlices() [][]Run {
	var out [][]Run
	switch b.Type {
	case TParagraph:
		out = append(out, b.Text)
	case TList:
		var items func([]*ListItem)
		items = func(li []*ListItem) {
			for _, it := range li {
				out = append(out, it.Text)
				items(it.Items)
			}
		}
		items(b.Items)
	case TDefinitionList:
		for _, d := range b.Defs {
			out = append(out, d.Term, d.Definition)
		}
	case TTable:
		out = append(out, b.Header...)
		for _, row := range b.Rows {
			out = append(out, row...)
		}
	}
	return out
}
