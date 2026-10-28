package main

// Word tables (docs/EDITOR.md, "Tables"), both ways: the .docx import
// (docxbody.go calls tableBlock, docxemit.go tableNode) and the .docx
// export (exportdocx.go calls (d *docxOut).table). The tables of the
// editor's documents are pmtable.go's.
//
// Import:
//   - w:tbl at the body's level is a table; one inside a cell is flattened
//     (its cells' paragraphs in reading order, in that cell; counted in
//     left_out.tables).
//   - Columns: w:tblGrid's gridCol widths (pt) become each cell's
//     colwidth; w:gridSpan is colspan, w:vMerge (restart, then continue)
//     rowspan; gridBefore/gridAfter and short rows are filled with empty
//     cells so every row is as wide as the grid.
//   - Rows marked w:tblHeader at the top are header rows (table_header
//     cells); later ones are ordinary rows.
//   - Table: w:jc center/right is align; a 100% w:tblW is width "full",
//     anything else follows the columns; borders from w:tblBorders, set
//     directly or by the table's style (and the styles it is based on):
//     inside lines on is "all", only edges "outer", none "none"; a table
//     with no table borders whose cells mostly draw their own (w:tcBorders,
//     as Google Docs writes them) is "all".
//   - Cells: w:shd fill is background, w:vAlign is valign (center =
//     middle).
//   - Cell text: the table style's paragraph and run properties come
//     between the document defaults and the paragraph's style, as in Word
//     (Table Grid's single spacing and no space after). Headings in cells
//     are paragraphs (they don't start chapters), pictures in a cell go
//     right after the table, page breaks are dropped. A Normal paragraph
//     carries indentFirst, spaceBefore and spaceAfter only when not 0 (the
//     cell defaults of EDITOR.md).
//
// Export: a w:tbl with fixed layout (the widths as the page view draws
// them: column shares of the table's width, never wider than the text),
// tblW (pct 100% for "full", else dxa), jc, tblBorders for the borders
// setting (single 0.5 pt lines), Word's 5.4 pt cell margins, tblGrid,
// header rows as tblHeader rows, cantSplit rows (the page view doesn't
// break inside a row either), gridSpan, vMerge (restart and continue
// cells), shd and vAlign. A Normal paragraph in a cell gets indent and
// spacing 0 written out where it doesn't set its own. A page break before
// a table is a paragraph holding the break (the import reads it back as
// the page break). Header cells outside the header rows come back as
// plain cells.

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

type irTable struct {
	rows    []irRow
	grid    []float64 // pt per grid column, 0 = unknown
	borders string
	align   string
	full    bool
	empty   irBlock // what an empty cell holds (an empty Normal paragraph)
}

type irRow struct {
	header bool
	cells  []*irCell
}

type irCell struct {
	blocks           []*irBlock
	col              int
	colspan, rowspan int
	vmerge           string // "", "restart", "continue"
	shade, valign    string
	borders          bool // draws borders of its own (tcBorders)
}

// irFlat is the blocks and, after each table, its cells' blocks.
func irFlat(blocks []*irBlock) []*irBlock {
	var out []*irBlock
	for _, b := range blocks {
		out = append(out, b)
		if b.kind == 't' && b.tbl != nil {
			for _, r := range b.tbl.rows {
				for _, c := range r.cells {
					out = append(out, c.blocks...)
				}
			}
		}
	}
	return out
}

// --- table styles ---

// tableStyleID: the table's style, else the file's default table style.
func (p *docxParser) tableStyleID(id string) string {
	if st := p.st.byID[id]; st != nil && st.kind == "table" {
		return id
	}
	for _, st := range p.st.byID {
		if st.kind == "table" && st.isDefault {
			return st.id
		}
	}
	return ""
}

func (p *docxParser) currentTableStyle() string {
	if n := len(p.tblStyles); n > 0 {
		return p.tblStyles[n-1]
	}
	return ""
}

// paraIn: a paragraph style's properties, with the table style's between
// the document defaults and the paragraph style inside a table.
func (p *docxParser) paraIn(styleID string) wPara {
	ts := p.currentTableStyle()
	if ts == "" {
		return p.st.para(styleID)
	}
	out := p.st.defP
	for _, st := range p.st.chain(ts) {
		out.over(st.p)
	}
	for _, st := range p.st.chain(styleID) {
		out.over(st.p)
	}
	return out
}

// runIn: a run's properties from the styles, the table style's included.
func (p *docxParser) runIn(paraStyle, charStyle string) wRun {
	ts := p.currentTableStyle()
	if ts == "" {
		return p.st.run(paraStyle, charStyle)
	}
	r := p.st.defR
	for _, st := range p.st.chain(ts) {
		r.over(st.r)
	}
	for _, st := range p.st.chain(paraStyle) {
		r.over(st.r)
	}
	if st := p.st.byID[charStyle]; st != nil && st.kind == "character" {
		for _, c := range p.st.chain(charStyle) {
			r.over(c.r)
		}
	}
	return r
}

// borderEdges reads a tblBorders or tcBorders into on (1) / off (-1).
func borderEdges(b *xEl, into map[string]int) {
	if b == nil {
		return
	}
	for _, k := range b.kids {
		name := k.name
		switch name {
		case "start":
			name = "left"
		case "end":
			name = "right"
		}
		switch v := k.attr("val"); v {
		case "", "nil", "none":
			into[name] = -1
		default:
			into[name] = 1
		}
	}
}

// --- reading ---

// tableBlock reads a w:tbl: a table at the body's level, flattened inside
// another table's cell.
func (p *docxParser) tableBlock(e *xEl) {
	if len(p.tblStyles) > 0 {
		p.count("tables")
		p.table(e)
		return
	}
	t, after := p.readTable(e)
	// A table longer than the editor takes goes on as another table.
	for len(t.rows) > maxTableRows {
		head := *t
		head.rows = t.rows[:maxTableRows]
		head.normalise()
		p.emit(&irBlock{kind: 't', tbl: &head, section: p.sect})
		rest := *t
		rest.rows = t.rows[maxTableRows:]
		t = &rest
		t.normalise()
	}
	p.emit(&irBlock{kind: 't', tbl: t, section: p.sect})
	for _, b := range after {
		p.emit(b)
	}
}

// tableRows: a table's rows, through content controls and the like.
func tableRows(e *xEl, out *[]*xEl, dropped *int) {
	for _, k := range e.kids {
		switch {
		case k.is("w", "tr"):
			*out = append(*out, k)
		case k.is("w", "customXml"), k.is("w", "ins"), k.is("w", "moveTo"):
			tableRows(k, out, dropped)
		case k.is("w", "sdt"):
			tableRows(k.child("w", "sdtContent"), out, dropped)
		case k.is("w", "del"), k.is("w", "moveFrom"):
			*dropped++
		}
	}
}

// rowCells: a row's cells, likewise.
func rowCells(e *xEl, out *[]*xEl) {
	for _, k := range e.kids {
		switch {
		case k.is("w", "tc"):
			*out = append(*out, k)
		case k.is("w", "customXml"), k.is("w", "ins"), k.is("w", "moveTo"):
			rowCells(k, out)
		case k.is("w", "sdt"):
			rowCells(k.child("w", "sdtContent"), out)
		}
	}
}

func (p *docxParser) emptyCellPara() *irBlock {
	st := p.paraIn(p.st.defaultPara)
	return &irBlock{kind: 'p', style: p.st.defaultPara, p: concretePara(st), section: p.sect, inCell: true}
}

func (p *docxParser) readTable(e *xEl) (*irTable, []*irBlock) {
	tblPr := e.child("w", "tblPr")
	style := p.tableStyleID(tblPr.child("w", "tblStyle").attr("val"))
	t := &irTable{align: "left", empty: *p.emptyCellPara()}
	for _, gc := range e.child("w", "tblGrid").childrenNamed("w", "gridCol") {
		w, _ := twipsAttr(gc, "w")
		t.grid = append(t.grid, math.Max(0, w))
	}
	switch tblPr.child("w", "jc").attr("val") {
	case "center":
		t.align = "center"
	case "right", "end":
		t.align = "right"
	}
	if w := tblPr.child("w", "tblW"); w != nil && w.attr("type") == "pct" {
		s := w.attr("w")
		pct := 0.0
		if strings.HasSuffix(s, "%") {
			pct, _ = strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64)
		} else if v, err := strconv.ParseFloat(s, 64); err == nil {
			pct = v / 50
		}
		t.full = pct >= 99
	}
	edges := map[string]int{}
	if style != "" {
		for _, st := range p.st.chain(style) {
			borderEdges(st.tblPr.child("w", "tblBorders"), edges)
		}
	}
	borderEdges(tblPr.child("w", "tblBorders"), edges)

	var rowEls []*xEl
	dropped := 0
	tableRows(e, &rowEls, &dropped)
	if dropped > 0 {
		p.leftOut["tracked_changes"] += dropped
	}

	var after []*irBlock
	p.tblStyles = append(p.tblStyles, style)
	open := map[int]*irCell{} // grid column → the cell a vMerge continues
	for _, re := range rowEls {
		trPr := re.child("w", "trPr")
		row := irRow{header: trPr.child("w", "tblHeader") != nil && wOn(trPr.child("w", "tblHeader"))}
		col := 0
		if gb, ok := intAttr(trPr.child("w", "gridBefore"), "val"); ok && gb > 0 {
			row.cells = append(row.cells, &irCell{blocks: []*irBlock{p.emptyCellPara()}, col: 0, colspan: min(gb, maxTableCols), rowspan: 1})
			col = gb
		}
		var tcs []*xEl
		rowCells(re, &tcs)
		continued := map[int]bool{}
		for _, tc := range tcs {
			tcPr := tc.child("w", "tcPr")
			c := &irCell{col: col, colspan: 1, rowspan: 1}
			if n, ok := intAttr(tcPr.child("w", "gridSpan"), "val"); ok && n > 1 {
				c.colspan = min(n, maxTableCols)
			}
			if vm := tcPr.child("w", "vMerge"); vm != nil {
				c.vmerge = "continue"
				if vm.attr("val") == "restart" {
					c.vmerge = "restart"
				}
			}
			if f := hexColor(tcPr.child("w", "shd").attr("fill")); f != "" {
				c.shade = f
			}
			switch tcPr.child("w", "vAlign").attr("val") {
			case "center":
				c.valign = "middle"
			case "bottom":
				c.valign = "bottom"
			}
			ce := map[string]int{}
			borderEdges(tcPr.child("w", "tcBorders"), ce)
			for _, v := range ce {
				if v > 0 {
					c.borders = true
				}
			}
			// The cell's text.
			var blocks []*irBlock
			saved := p.out
			p.out = &blocks
			p.walkBlocks(tc.kids)
			p.out = saved
			for _, b := range blocks {
				switch b.kind {
				case 'i':
					after = append(after, b)
				case 'p':
					b.level = 0
					b.inCell = true
					c.blocks = append(c.blocks, b)
				}
			}
			col += c.colspan
			// A continued vertical merge joins the cell above.
			if c.vmerge == "continue" {
				if o := open[c.col]; o != nil && o.colspan == c.colspan {
					o.rowspan++
					continued[c.col] = true
					if hasContent(c.blocks) {
						o.blocks = append(o.blocks, c.blocks...)
					}
					continue
				}
			}
			if len(c.blocks) == 0 {
				c.blocks = []*irBlock{p.emptyCellPara()}
			}
			row.cells = append(row.cells, c)
		}
		// A merge goes on only while each row continues it.
		next := map[int]*irCell{}
		for gc, o := range open {
			if continued[gc] {
				next[gc] = o
			}
		}
		for _, c := range row.cells {
			next[c.col] = c
		}
		open = next
		t.rows = append(t.rows, row)
	}
	p.tblStyles = p.tblStyles[:len(p.tblStyles)-1]
	t.normalise()

	// Borders: inside lines mean all, edges alone the outer border.
	switch {
	case edges["insideH"] > 0 || edges["insideV"] > 0:
		t.borders = "all"
	case edges["top"] > 0 || edges["bottom"] > 0 || edges["left"] > 0 || edges["right"] > 0:
		t.borders = "outer"
	default:
		t.borders = "none"
	}
	if t.borders != "all" {
		n, with := 0, 0
		for _, r := range t.rows {
			for _, c := range r.cells {
				n++
				if c.borders {
					with++
				}
			}
		}
		if n > 0 && with*2 > n {
			t.borders = "all"
		}
	}
	if len(t.rows) == 0 {
		t.rows = []irRow{{cells: []*irCell{{blocks: []*irBlock{p.emptyCellPara()}, colspan: 1, rowspan: 1}}}}
		t.normalise()
	}
	return t, after
}

// normalise makes the table a grid the editor takes: rowspans end at the
// last row, at most maxTableCols columns, every row as wide as the
// widest (gaps filled with empty cells), a width for every grid column.
func (t *irTable) normalise() {
	n := len(t.rows)
	width := len(t.grid)
	covered := make([]map[int]bool, n)
	for r := range covered {
		covered[r] = map[int]bool{}
	}
	for r := range t.rows {
		row := &t.rows[r]
		var kept []*irCell
		for _, c := range row.cells {
			if c.col >= maxTableCols {
				// Past Word's widest: its text goes in the cell before.
				if len(kept) > 0 && hasContent(c.blocks) {
					kept[len(kept)-1].blocks = append(kept[len(kept)-1].blocks, c.blocks...)
				}
				continue
			}
			c.colspan = max(1, min(c.colspan, maxTableCols-c.col))
			c.rowspan = max(1, min(c.rowspan, n-r))
			for dr := 1; dr < c.rowspan; dr++ {
				for dc := 0; dc < c.colspan; dc++ {
					covered[r+dr][c.col+dc] = true
				}
			}
			width = max(width, c.col+c.colspan)
			kept = append(kept, c)
		}
		row.cells = kept
	}
	width = min(width, maxTableCols)
	for len(t.grid) < width {
		t.grid = append(t.grid, 0)
	}
	t.grid = t.grid[:width]
	// Fill the gaps: positions no cell starts at nor covers.
	for r := range t.rows {
		row := &t.rows[r]
		var out []*irCell
		pos := 0
		fill := func(to int) {
			for pos < to {
				if covered[r][pos] {
					pos++
					continue
				}
				e := t.empty
				e.kind, e.inCell = 'p', true
				out = append(out, &irCell{blocks: []*irBlock{&e}, col: pos, colspan: 1, rowspan: 1})
				pos++
			}
		}
		for _, c := range row.cells {
			if c.col < pos {
				// Overlapping what came before (a broken file): moved after it.
				c.col = pos
				if c.col+c.colspan > width {
					c.colspan = max(1, width-c.col)
				}
				if c.col >= width {
					continue
				}
			}
			fill(c.col)
			out = append(out, c)
			pos = c.col + c.colspan
		}
		fill(width)
		row.cells = out
	}
}

// --- emitting ---

func (p *docxParser) tableNode(t *irTable, set *styleSet, keep map[string]bool) *pmNode {
	attrs := map[string]any{"borders": t.borders}
	if t.align == "center" || t.align == "right" {
		attrs["align"] = t.align
	}
	if t.full {
		attrs["width"] = "full"
	}
	n := &pmNode{Type: "table", Attrs: attrs}
	heading := true
	for _, r := range t.rows {
		heading = heading && r.header
		row := &pmNode{Type: "table_row"}
		for _, c := range r.cells {
			ca := map[string]any{}
			if c.colspan > 1 {
				ca["colspan"] = c.colspan
			}
			if c.rowspan > 1 {
				ca["rowspan"] = c.rowspan
			}
			ws := make([]any, c.colspan)
			known := false
			for j := range ws {
				ws[j] = 0
				if k := c.col + j; k < len(t.grid) && t.grid[k] > 0 {
					ws[j] = r2(math.Min(t.grid[k], 5000))
					known = true
				}
			}
			if known {
				ca["colwidth"] = ws
			}
			if c.shade != "" {
				ca["background"] = c.shade
			}
			if c.valign != "" {
				ca["valign"] = c.valign
			}
			cell := &pmNode{Type: "table_cell", Content: p.cellNodes(c.blocks, set, keep)}
			if heading {
				cell.Type = "table_header"
			}
			if len(ca) > 0 {
				cell.Attrs = ca
			}
			row.Content = append(row.Content, cell)
		}
		n.Content = append(n.Content, row)
	}
	return n
}

// cellNodes: a cell's paragraphs and lists, Normal paragraphs carrying
// indentFirst, spaceBefore and spaceAfter only when not 0 (the cell
// defaults).
func (p *docxParser) cellNodes(blocks []*irBlock, set *styleSet, keep map[string]bool) []*pmNode {
	var paras []*irBlock
	var bs []*irBlock
	for _, b := range blocks {
		if b.kind != 'p' {
			continue
		}
		if b.style == "" {
			b.style = p.st.defaultPara
			b.p = concretePara(p.st.para(b.style))
		}
		bs = append(bs, b)
		if b.list == nil {
			paras = append(paras, b)
		}
	}
	var out []*pmNode
	i := 0
	for _, n := range p.blockNodes(bs, set, keep) {
		switch n.Type {
		case "paragraph":
			if i < len(paras) {
				cellDefaults(n, paras[i])
			}
			i++
			out = append(out, n)
		case "heading": // can't be (levels are cleared), but never a heading in a cell
			n.Type = "paragraph"
			delete(n.Attrs, "level")
			i++
			out = append(out, n)
		case "bullet_list", "ordered_list":
			out = append(out, n)
		}
	}
	if len(out) == 0 {
		out = []*pmNode{{Type: "paragraph"}}
	}
	return out
}

func cellDefaults(n *pmNode, b *irBlock) {
	if _, styled := n.Attrs["style"]; styled {
		return
	}
	if n.Attrs == nil {
		n.Attrs = map[string]any{}
	}
	for _, x := range []struct {
		k string
		v float64
	}{{"indentFirst", b.p.indFirst}, {"spaceBefore", b.p.before}, {"spaceAfter", b.p.after}} {
		if r2(x.v) == 0 {
			delete(n.Attrs, x.k)
		} else {
			n.Attrs[x.k] = r2(x.v)
		}
	}
	if len(n.Attrs) == 0 {
		n.Attrs = nil
	}
}

// --- export ---

// cellDefaultsOut: a Normal paragraph in a cell with its indent and
// spacing written out as the cell defaults (0) where it doesn't set them.
func (d *docxOut) cellDefaultsOut(n *pmNode) *pmNode {
	if n == nil || n.Type != "paragraph" || d.b.paraStyleOf(n) != "normal" {
		return n
	}
	c := *n
	c.Attrs = map[string]any{}
	for k, v := range n.Attrs {
		c.Attrs[k] = v
	}
	for _, k := range []string{"indentFirst", "spaceBefore", "spaceAfter"} {
		if _, ok := pmNum(c.Attrs[k]); !ok {
			c.Attrs[k] = 0
		}
	}
	return &c
}

// tableColumnsPt: a table's column widths in pt as the page view draws
// them (shares of the table's width, never wider than the text).
func tableColumnsPt(t *pmNode, g *pmTableGrid, textW float64) []float64 {
	ws := g.colWidths()
	sum, known := 0.0, 0
	for _, w := range ws {
		if w > 0 {
			sum += w
			known++
		}
	}
	out := make([]float64, len(ws))
	if known == 0 {
		for i := range out {
			out[i] = 1
		}
	} else {
		avg := sum / float64(known)
		for i, w := range ws {
			out[i] = w
			if w <= 0 {
				out[i] = avg
			}
		}
	}
	total := 0.0
	for _, w := range out {
		total += w
	}
	full, fixed := tableWidth(t)
	target := total
	switch {
	case full || known == 0 && fixed == 0:
		target = textW
	case fixed > 0:
		target = fixed
	case known < len(ws):
		target = textW
	}
	if textW > 0 {
		target = math.Min(target, textW)
	}
	for i := range out {
		out[i] = out[i] / total * target
	}
	return out
}

const docxBorder = `w:val="single" w:sz="4" w:space="0" w:color="auto"`

func (d *docxOut) table(n *pmNode) {
	g, err := tableGridOf(n)
	if err != nil { // not a valid table: its text, cell by cell
		for _, row := range n.Content {
			for _, c := range row.Content {
				if c != nil {
					d.blocks(c.Content, nil)
				}
			}
		}
		return
	}
	d.flush("")
	if d.breakNext && d.started {
		d.doc.WriteString(`<w:p><w:r><w:br w:type="page"/></w:r></w:p>`)
	}
	d.breakNext = false
	d.started = true
	cols := tableColumnsPt(n, g, d.textW)
	s := &d.doc
	s.WriteString(`<w:tbl><w:tblPr>`)
	if full, _ := tableWidth(n); full {
		s.WriteString(`<w:tblW w:w="5000" w:type="pct"/>`)
	} else {
		total := 0.0
		for _, w := range cols {
			total += w
		}
		fmt.Fprintf(s, `<w:tblW w:w="%d" w:type="dxa"/>`, twips(total))
	}
	if al := tableAlign(n); al != "left" {
		fmt.Fprintf(s, `<w:jc w:val="%s"/>`, al)
	}
	edge := func(name string, on bool) string {
		if on {
			return `<w:` + name + ` ` + docxBorder + `/>`
		}
		return `<w:` + name + ` w:val="nil"/>`
	}
	b := tableBorders(n)
	outer, inner := b != "none", b == "all"
	s.WriteString(`<w:tblBorders>` + edge("top", outer) + edge("left", outer) + edge("bottom", outer) + edge("right", outer) +
		edge("insideH", inner) + edge("insideV", inner) + `</w:tblBorders>`)
	s.WriteString(`<w:tblLayout w:type="fixed"/><w:tblCellMar><w:left w:w="108" w:type="dxa"/><w:right w:w="108" w:type="dxa"/></w:tblCellMar>`)
	s.WriteString(`<w:tblLook w:val="0000" w:firstRow="0" w:lastRow="0" w:firstColumn="0" w:lastColumn="0" w:noHBand="1" w:noVBand="1"/></w:tblPr><w:tblGrid>`)
	for _, w := range cols {
		fmt.Fprintf(s, `<w:gridCol w:w="%d"/>`, twips(w))
	}
	s.WriteString(`</w:tblGrid>`)
	heads := tableHeaderRows(n)
	spanW := func(col, span int) int {
		w := 0.0
		for j := col; j < col+span && j < len(cols); j++ {
			w += cols[j]
		}
		return twips(w)
	}
	for r := 0; r < g.rows; r++ {
		s.WriteString(`<w:tr><w:trPr><w:cantSplit/>`)
		if r < heads {
			s.WriteString(`<w:tblHeader/>`)
		}
		s.WriteString(`</w:trPr>`)
		for c := 0; c < g.cols; {
			cell := g.cells[g.at[r][c]]
			fmt.Fprintf(s, `<w:tc><w:tcPr><w:tcW w:w="%d" w:type="dxa"/>`, spanW(cell.col, cell.colspan))
			if cell.colspan > 1 {
				fmt.Fprintf(s, `<w:gridSpan w:val="%d"/>`, cell.colspan)
			}
			if cell.row != r { // under a cell spanning rows
				s.WriteString(`<w:vMerge/></w:tcPr><w:p/></w:tc>`)
				c += cell.colspan
				continue
			}
			if cell.rowspan > 1 {
				s.WriteString(`<w:vMerge w:val="restart"/>`)
			}
			if bg := cellBackground(cell.node); bg != "" {
				fmt.Fprintf(s, `<w:shd w:val="clear" w:color="auto" w:fill="%s"/>`, hexVal(bg))
			}
			switch cellValign(cell.node) {
			case "middle":
				s.WriteString(`<w:vAlign w:val="center"/>`)
			case "bottom":
				s.WriteString(`<w:vAlign w:val="bottom"/>`)
			}
			s.WriteString(`</w:tcPr>`)
			start := s.Len()
			for _, k := range cell.node.Content {
				d.block(d.cellDefaultsOut(k), nil)
			}
			d.flush("")
			if s.Len() == start { // a cell holds at least a paragraph
				s.WriteString(`<w:p/>`)
			}
			s.WriteString(`</w:tc>`)
			c += cell.colspan
		}
		s.WriteString(`</w:tr>`)
	}
	s.WriteString(`</w:tbl>`)
}
