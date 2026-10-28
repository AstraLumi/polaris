package main

// Tables in the editor's documents (docs/EDITOR.md, stage 5; the schema is
// frontend/src/editor/tableSchema.js): a `table` of `table_row`s of
// `table_cell`s and `table_header`s, as prosemirror-tables makes them.
// Tables sit at the top level of a doc only (not in lists, not in cells).
//
//	table         attrs borders ("all" when missing, "outer", "none"),
//	              align (null = left, "center", "right"), width (null:
//	              from the columns; "full"; pt)
//	table_row     no attrs of its own
//	table_cell /  attrs colspan, rowspan (1 when missing), colwidth (null
//	table_header  or one width in pt per column it spans, 0 = unknown),
//	              background (#rrggbb or null), valign (null = top,
//	              "middle", "bottom"); content: paragraphs and lists
//
// A table's leading rows of header cells are its header rows (they repeat
// on each page). Normal paragraphs in cells have no first-line indent and
// no space before or after unless they set their own.
//
// API:
//
//	(v *pmValidator) table(n, depth) error   validation (validatePMDoc)
//	tableGridOf(n) (*pmTableGrid, error)     where each cell sits
//	tableHeaderRows(n) int                   the header rows' count
//	tableBorders(n), tableAlign(n), tableWidth(n), cellBackground(c),
//	cellValign(c), cellSpan(c)               attrs read with their defaults

import (
	"errors"
	"fmt"
	"strings"
)

const (
	maxTableCols = 63 // Word's limit
	maxTableRows = 1000
)

// pmGridCell is a cell and where it sits in its table's grid.
type pmGridCell struct {
	node             *pmNode
	row, col         int
	colspan, rowspan int
	header           bool
}

// pmTableGrid is a table's cells on its grid: at[r][c] is the index in
// cells of the cell covering row r, column c.
type pmTableGrid struct {
	rows, cols int
	cells      []pmGridCell
	at         [][]int
}

func isTableCell(n *pmNode) bool {
	return n != nil && (n.Type == "table_cell" || n.Type == "table_header")
}

// cellSpan reads a cell's colspan and rowspan (1 when missing).
func cellSpan(c *pmNode) (colspan, rowspan int) {
	colspan, rowspan = 1, 1
	if v, ok := pmInt(c.Attrs["colspan"]); ok && v > 0 && v <= maxTableCols {
		colspan = int(v)
	}
	if v, ok := pmInt(c.Attrs["rowspan"]); ok && v > 0 && v <= maxTableRows {
		rowspan = int(v)
	}
	return
}

// tableGridOf places a table's cells. Errors: rows of different widths,
// cells overlapping, a cell spanning past the last row, too many columns.
func tableGridOf(t *pmNode) (*pmTableGrid, error) {
	g := &pmTableGrid{rows: len(t.Content)}
	g.at = make([][]int, g.rows)
	width := -1
	for r, row := range t.Content {
		if row == nil {
			return nil, errors.New("table: empty row")
		}
		if g.at[r] == nil {
			g.at[r] = []int{}
		}
		col := 0
		for _, c := range row.Content {
			if !isTableCell(c) {
				return nil, errors.New("table_row: may only hold table_cell and table_header")
			}
			for col < len(g.at[r]) && g.at[r][col] >= 0 {
				col++
			}
			cs, rs := cellSpan(c)
			if r+rs > g.rows {
				return nil, errors.New("table: a cell spans past the last row")
			}
			idx := len(g.cells)
			g.cells = append(g.cells, pmGridCell{node: c, row: r, col: col, colspan: cs, rowspan: rs, header: c.Type == "table_header"})
			for dr := 0; dr < rs; dr++ {
				line := g.at[r+dr]
				for len(line) < col+cs {
					line = append(line, -1)
				}
				for dc := 0; dc < cs; dc++ {
					if line[col+dc] >= 0 {
						return nil, errors.New("table: cells overlap")
					}
					line[col+dc] = idx
				}
				g.at[r+dr] = line
			}
			col += cs
			if col > maxTableCols {
				return nil, fmt.Errorf("table: more than %d columns", maxTableCols)
			}
		}
	}
	for r := range g.at {
		w := len(g.at[r])
		for _, i := range g.at[r] {
			if i < 0 {
				return nil, errors.New("table: a row has a gap")
			}
		}
		if width < 0 {
			width = w
		} else if w != width {
			return nil, errors.New("table: rows of different widths")
		}
	}
	if width <= 0 {
		return nil, errors.New("table: no cells")
	}
	g.cols = width
	return g, nil
}

// colWidths: each column's width in pt (0 when no cell says).
func (g *pmTableGrid) colWidths() []float64 {
	out := make([]float64, g.cols)
	for _, c := range g.cells {
		ws, _ := c.node.Attrs["colwidth"].([]any)
		for j := 0; j < c.colspan && j < len(ws); j++ {
			if w, ok := pmNum(ws[j]); ok && w > 0 && out[c.col+j] == 0 {
				out[c.col+j] = w
			}
		}
	}
	return out
}

// tableHeaderRows: how many leading rows are all header cells (a table
// that is nothing but header rows has none to repeat).
func tableHeaderRows(t *pmNode) int {
	n := 0
	for _, row := range t.Content {
		all := row != nil && len(row.Content) > 0
		for _, c := range row.Content {
			if c == nil || c.Type != "table_header" {
				all = false
			}
		}
		if !all {
			break
		}
		n++
	}
	if n == len(t.Content) {
		return 0
	}
	return n
}

func tableBorders(t *pmNode) string {
	switch s, _ := t.Attrs["borders"].(string); s {
	case "outer", "none":
		return s
	}
	return "all"
}

func tableAlign(t *pmNode) string {
	switch s, _ := t.Attrs["align"].(string); s {
	case "center", "right":
		return s
	}
	return "left"
}

// tableWidth: full (the text's width), or pt (> 0), or neither (from the
// columns).
func tableWidth(t *pmNode) (full bool, pt float64) {
	if s, _ := t.Attrs["width"].(string); s == "full" {
		return true, 0
	}
	if w, ok := pmNum(t.Attrs["width"]); ok && w > 0 {
		return false, w
	}
	return false, 0
}

func cellBackground(c *pmNode) string {
	if s, _ := c.Attrs["background"].(string); pmColorRe.MatchString(s) {
		return strings.ToLower(s)
	}
	return ""
}

func cellValign(c *pmNode) string {
	switch s, _ := c.Attrs["valign"].(string); s {
	case "middle", "bottom":
		return s
	}
	return "top"
}

// --- validation ---

func (v *pmValidator) table(t *pmNode, depth int) error {
	if err := v.count(); err != nil {
		return err
	}
	if len(t.Marks) > 0 || t.Text != "" {
		return errors.New("table: has no marks or text")
	}
	a := t.Attrs
	if a == nil {
		a = map[string]any{}
	}
	if b, ok := a["borders"]; ok && b != nil {
		if s, _ := b.(string); s != "all" && s != "outer" && s != "none" {
			return errors.New("table: attr borders must be all, outer or none")
		}
	}
	if al, err := optString(a, "align", 10); err != nil || (al != "" && al != "left" && al != "center" && al != "right") {
		return errors.New("table: attr align: unknown value")
	}
	if w, ok := a["width"]; ok && w != nil {
		if s, isStr := w.(string); isStr {
			if s != "full" {
				return errors.New(`table: attr width must be "full", a number of pt or null`)
			}
		} else if f, ok := pmNum(w); !ok || f < 10 || f > 5000 {
			return errors.New("table: attr width: expected a number from 10 to 5000")
		}
	}
	if len(t.Content) == 0 {
		return errors.New("table: needs at least one row")
	}
	if len(t.Content) > maxTableRows {
		return fmt.Errorf("table: more than %d rows", maxTableRows)
	}
	for _, row := range t.Content {
		if row == nil || row.Type != "table_row" {
			return errors.New("table: may only hold table_row")
		}
		if err := v.count(); err != nil {
			return err
		}
		if len(row.Marks) > 0 || row.Text != "" {
			return errors.New("table_row: has no marks or text")
		}
		// A row may be empty when cells above span all of it (merging can
		// leave one, as prosemirror-tables allows).
		for _, c := range row.Content {
			if !isTableCell(c) {
				return errors.New("table_row: may only hold table_cell and table_header")
			}
			if err := v.cell(c, depth+2); err != nil {
				return err
			}
		}
	}
	_, err := tableGridOf(t)
	return err
}

func (v *pmValidator) cell(c *pmNode, depth int) error {
	if err := v.count(); err != nil {
		return err
	}
	if len(c.Marks) > 0 || c.Text != "" {
		return fmt.Errorf("%s: has no marks or text", c.Type)
	}
	a := c.Attrs
	if a == nil {
		a = map[string]any{}
	}
	wrap := func(msg string) error { return fmt.Errorf("%s: %s", c.Type, msg) }
	colspan := 1
	if x, ok := a["colspan"]; ok && x != nil {
		n, ok := pmInt(x)
		if !ok || n < 1 || n > maxTableCols {
			return wrap(fmt.Sprintf("attr colspan: expected 1 to %d", maxTableCols))
		}
		colspan = int(n)
	}
	if x, ok := a["rowspan"]; ok && x != nil {
		if n, ok := pmInt(x); !ok || n < 1 || n > maxTableRows {
			return wrap(fmt.Sprintf("attr rowspan: expected 1 to %d", maxTableRows))
		}
	}
	if x, ok := a["colwidth"]; ok && x != nil {
		ws, isList := x.([]any)
		if !isList || len(ws) != colspan {
			return wrap("attr colwidth: expected one width per column the cell spans")
		}
		for _, w := range ws {
			if w == nil {
				continue
			}
			if f, ok := pmNum(w); !ok || f < 0 || f > 5000 {
				return wrap("attr colwidth: expected widths from 0 to 5000 pt")
			}
		}
	}
	if x, ok := a["background"]; ok && x != nil {
		if s, isStr := x.(string); !isStr || !pmColorRe.MatchString(s) {
			return wrap("attr background: expected #rrggbb")
		}
	}
	if va, err := optString(a, "valign", 10); err != nil || (va != "" && va != "top" && va != "middle" && va != "bottom") {
		return wrap("attr valign: unknown value")
	}
	if len(c.Content) == 0 {
		return wrap("needs at least one paragraph")
	}
	for _, b := range c.Content {
		if b == nil || (b.Type != "paragraph" && b.Type != "bullet_list" && b.Type != "ordered_list") {
			return wrap("may only hold paragraphs and lists")
		}
		if err := v.block(b, c.Type, depth+2); err != nil {
			return err
		}
	}
	return nil
}
