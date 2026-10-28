package main

// Tables in the EPUB and Online Post exports (export.go; docs/EDITOR.md,
// "Tables"). The .docx export's are in docxtables.go.
//
//   - EPUB: an XHTML table (class tb and tb-all / tb-outer / tb-none for
//     the borders), a colgroup with the columns' shares, its width as a
//     share of the text's, centred or right with auto margins, header
//     rows in a thead, colspan/rowspan, a cell's shading and vertical
//     alignment inline; the cells' blocks as everywhere else.
//   - HTML (AO3): table, thead (the header rows), tbody, tr, th, td with
//     colspan and rowspan only: no classes, no styles (AO3 skins draw
//     tables). A cell of one paragraph holds its text; others their p and
//     lists.
//   - Markdown: a GitHub pipe table when no cell spans rows or columns and
//     every cell is one paragraph (hard breaks as <br>); its first row is
//     the header row (a pipe table must have one). Any other table is
//     written as the HTML table above.
//   - Text: columns lined up with spaces (two between columns; a cell's
//     lines one under the other), a line of dashes under the header rows.

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

// tableShares: each column's share (%) of the table, and the table's share
// of the text's width.
func tableShares(t *pmNode, g *pmTableGrid, textW float64) (cols []float64, width float64) {
	pts := tableColumnsPt(t, g, textW)
	total := 0.0
	for _, w := range pts {
		total += w
	}
	for _, w := range pts {
		cols = append(cols, math.Round(w/total*1000)/10)
	}
	width = 100
	if textW > 0 {
		width = math.Min(100, math.Round(total/textW*1000)/10)
	}
	return cols, width
}

// --- EPUB ---

func epubTableCSS(body float64) string {
	pad := emOf(5.4, body)
	return `table.tb { border-collapse: collapse; table-layout: fixed; margin: 0; text-indent: 0; }
table.tb td, table.tb th { padding: 0 ` + pad + `; vertical-align: top; text-align: inherit; font-weight: inherit; border: 0.5pt solid; }
table.tb-outer td, table.tb-outer th, table.tb-none td, table.tb-none th { border: none; }
table.tb-outer { border: 0.5pt solid; }
table.tb td > p, table.tb th > p { text-indent: 0; margin-top: 0; margin-bottom: 0; }
table.tb ul, table.tb ol { margin: 0; }
table.tb tr { page-break-inside: avoid; break-inside: avoid; }
`
}

func (e *epubOut) table(ctx *epubCtx, n *pmNode) {
	g, err := tableGridOf(n)
	if err != nil {
		for _, row := range n.Content {
			for _, c := range row.Content {
				for _, k := range c.Content {
					e.block(ctx, k)
				}
			}
		}
		return
	}
	s := ctx.s
	cols, width := tableShares(n, g, e.b.textWidthPt())
	style := "width: " + fmtNum(width) + "%"
	switch tableAlign(n) {
	case "center":
		style += "; margin-left: auto; margin-right: auto"
	case "right":
		style += "; margin-left: auto; margin-right: 0"
	}
	fmt.Fprintf(s, `<table class="tb tb-%s" style="%s">`+"\n<colgroup>", tableBorders(n), style)
	for _, w := range cols {
		fmt.Fprintf(s, `<col style="width: %s%%"/>`, fmtNum(w))
	}
	s.WriteString("</colgroup>\n")
	heads := tableHeaderRows(n)
	for r, row := range n.Content {
		if r == 0 && heads > 0 {
			s.WriteString("<thead>\n")
		}
		if r == heads {
			s.WriteString("<tbody>\n")
		}
		s.WriteString("<tr>")
		for _, c := range row.Content {
			tag := "td"
			if c.Type == "table_header" {
				tag = "th"
			}
			s.WriteString("<" + tag + spanAttrs(c))
			var css []string
			if bg := cellBackground(c); bg != "" {
				css = append(css, "background-color: "+bg)
			}
			if va := cellValign(c); va != "top" {
				css = append(css, "vertical-align: "+va)
			}
			if len(css) > 0 {
				s.WriteString(` style="` + strings.Join(css, "; ") + `"`)
			}
			s.WriteString(">\n")
			for _, k := range c.Content {
				e.block(ctx, k)
			}
			s.WriteString("</" + tag + ">")
		}
		s.WriteString("</tr>\n")
		if r == heads-1 {
			s.WriteString("</thead>\n")
		}
	}
	s.WriteString("</tbody>\n</table>\n")
}

func spanAttrs(c *pmNode) string {
	cs, rs := cellSpan(c)
	out := ""
	if cs > 1 {
		out += fmt.Sprintf(` colspan="%d"`, cs)
	}
	if rs > 1 {
		out += fmt.Sprintf(` rowspan="%d"`, rs)
	}
	return out
}

// --- HTML (AO3) ---

func (h *htmlPost) table(n *pmNode) {
	heads := tableHeaderRows(n)
	h.s.WriteString("<table>\n")
	for r, row := range n.Content {
		if r == 0 && heads > 0 {
			h.s.WriteString("<thead>\n")
		}
		if r == heads {
			h.s.WriteString("<tbody>\n")
		}
		h.s.WriteString("<tr>")
		for _, c := range row.Content {
			tag := "td"
			if c.Type == "table_header" {
				tag = "th"
			}
			h.s.WriteString("<" + tag + spanAttrs(c) + ">")
			if len(c.Content) == 1 && c.Content[0].Type == "paragraph" {
				h.s.WriteString(htmlInline(h.b.blockSegs(c.Content[0])))
			} else {
				h.s.WriteString("\n")
				for _, k := range c.Content {
					h.block(k, 1)
				}
			}
			h.s.WriteString("</" + tag + ">")
		}
		h.s.WriteString("</tr>\n")
		if r == heads-1 {
			h.s.WriteString("</thead>\n")
		}
	}
	h.s.WriteString("</tbody>\n</table>\n")
}

// --- Markdown ---

// pipeTable: a GitHub pipe table, or false when the table needs HTML.
func (m *mdPost) pipeTable(n *pmNode) (string, bool) {
	var rows [][]string
	width := -1
	for _, row := range n.Content {
		var cells []string
		for _, c := range row.Content {
			if cs, rs := cellSpan(c); cs != 1 || rs != 1 || len(c.Content) != 1 || c.Content[0].Type != "paragraph" {
				return "", false
			}
			s := mdInline(m.b.blockSegs(c.Content[0]))
			s = strings.ReplaceAll(s, "\\\n", "<br>")
			s = strings.ReplaceAll(s, "\n", " ")
			if strings.TrimSpace(s) == "" {
				s = " "
			}
			cells = append(cells, s)
		}
		if width >= 0 && len(cells) != width || len(cells) == 0 {
			return "", false
		}
		width = len(cells)
		rows = append(rows, cells)
	}
	line := func(cells []string) string { return "| " + strings.Join(cells, " | ") + " |" }
	out := []string{line(rows[0])}
	sep := make([]string, width)
	for i := range sep {
		sep[i] = "---"
	}
	out = append(out, line(sep))
	for _, r := range rows[1:] {
		out = append(out, line(r))
	}
	return strings.Join(out, "\n"), true
}

func (m *mdPost) table(n *pmNode) string {
	if s, ok := m.pipeTable(n); ok {
		return s
	}
	h := &htmlPost{b: m.b}
	h.table(n)
	return strings.TrimSpace(h.s.String())
}

// --- plain text ---

func runeWidth(s string) int { return utf8.RuneCountInString(s) }

// textTable lines a table up in columns. walk gives a block's text.
func textTable(n *pmNode, indent string, walk func(*pmNode, string) string) string {
	g, err := tableGridOf(n)
	if err != nil {
		var parts []string
		for _, row := range n.Content {
			for _, c := range row.Content {
				for _, k := range c.Content {
					if s := walk(k, indent); s != "" {
						parts = append(parts, s)
					}
				}
			}
		}
		return strings.Join(parts, "\n")
	}
	const gap = 2
	lines := make([][]string, len(g.cells))
	for i, c := range g.cells {
		var all []string
		for _, k := range c.node.Content {
			if s := walk(k, ""); s != "" {
				all = append(all, strings.Split(strings.ReplaceAll(s, "\t", " "), "\n")...)
			}
		}
		lines[i] = all
	}
	widths := make([]int, g.cols)
	cellW := func(c pmGridCell, i int) int {
		w := 0
		for _, l := range lines[i] {
			w = max(w, runeWidth(l))
		}
		return w
	}
	for i, c := range g.cells {
		if c.colspan == 1 {
			widths[c.col] = max(widths[c.col], cellW(c, i))
		}
	}
	for i, c := range g.cells {
		if c.colspan > 1 {
			have := gap * (c.colspan - 1)
			for j := c.col; j < c.col+c.colspan; j++ {
				have += widths[j]
			}
			if need := cellW(c, i); need > have {
				widths[c.col+c.colspan-1] += need - have
			}
		}
	}
	spanWidth := func(c pmGridCell) int {
		w := gap * (c.colspan - 1)
		for j := c.col; j < c.col+c.colspan; j++ {
			w += widths[j]
		}
		return w
	}
	heads := tableHeaderRows(n)
	var out []string
	for r := 0; r < g.rows; r++ {
		height := 1
		for i, c := range g.cells {
			if c.row == r {
				height = max(height, len(lines[i]))
			}
		}
		for l := 0; l < height; l++ {
			var b strings.Builder
			for col := 0; col < g.cols; {
				i := g.at[r][col]
				c := g.cells[i]
				text := ""
				if c.row == r && l < len(lines[i]) {
					text = lines[i][l]
				}
				if col > 0 {
					b.WriteString(strings.Repeat(" ", gap))
				}
				b.WriteString(text + strings.Repeat(" ", max(0, spanWidth(c)-runeWidth(text))))
				col += c.colspan
			}
			out = append(out, indent+strings.TrimRight(b.String(), " "))
		}
		if r == heads-1 {
			total := gap * (g.cols - 1)
			for _, w := range widths {
				total += w
			}
			out = append(out, indent+strings.Repeat("-", max(3, total)))
		}
	}
	return strings.Join(out, "\n")
}
