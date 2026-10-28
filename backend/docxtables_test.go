package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// Tables (stage 5): validation, the .docx import, the exports, word counts
// and the chapter index. The .docx round trip is TestExportDocxRoundTrip
// (testBook's third chapter holds two tables).

const tableJSON = `{"type":"doc","content":[
 {"type":"paragraph","content":[{"type":"text","text":"Before."}]},
 {"type":"table","attrs":{"borders":"outer","align":"center","width":"full","wordStyle":"Grid"},"content":[
  {"type":"table_row","content":[
   {"type":"table_header","attrs":{"colwidth":[90]},"content":[{"type":"paragraph","content":[{"type":"text","text":"Stat"}]}]},
   {"type":"table_header","attrs":{"colwidth":[0]},"content":[{"type":"paragraph","content":[{"type":"text","text":"Value"}]}]}]},
  {"type":"table_row","content":[
   {"type":"table_cell","attrs":{"rowspan":2,"background":"#d9d9d9","valign":"middle"},"content":[{"type":"paragraph","content":[{"type":"text","text":"Strength and "},{"type":"text","text":"Mira","marks":[{"type":"story_link","attrs":{"type":"character","id":12}}]}]}]},
   {"type":"table_cell","content":[{"type":"bullet_list","content":[{"type":"list_item","content":[{"type":"paragraph","content":[{"type":"text","text":"twelve"}]}]}]}]}]},
  {"type":"table_row","content":[
   {"type":"table_cell","content":[{"type":"paragraph"}]}]}
 ]}]}`

func TestTableValidation(t *testing.T) {
	doc, err := parsePMDoc([]byte(tableJSON))
	if err != nil {
		t.Fatal(err)
	}
	g, err := tableGridOf(doc.Content[1])
	if err != nil || g.rows != 3 || g.cols != 2 || g.at[2][0] != g.at[1][0] {
		t.Fatalf("grid %+v %v", g, err)
	}
	if ws := g.colWidths(); ws[0] != 90 || ws[1] != 0 {
		t.Errorf("column widths %v", ws)
	}
	if tableHeaderRows(doc.Content[1]) != 1 {
		t.Error("one header row")
	}
	// Round trip keeps unknown attrs.
	out, _ := json.Marshal(doc)
	if !strings.Contains(string(out), `"wordStyle":"Grid"`) {
		t.Errorf("unknown attr lost: %s", out)
	}
	// Words in cells count; story links in cells are mentions.
	if n := docWords(doc); n != 7 {
		t.Errorf("docWords = %d, want 7", n)
	}
	if got := fmt.Sprint(docLinkCounts(doc)); got != fmt.Sprint([]pmLinkCount{{pmStoryLink{"character", 12}, 1}}) {
		t.Errorf("docLinkCounts = %s", got)
	}

	cell := func(attrs, content string) string {
		if content == "" {
			content = `[{"type":"paragraph"}]`
		}
		return `{"type":"table_cell"` + attrs + `,"content":` + content + `}`
	}
	tbl := func(attrs string, rows ...string) string {
		var rs []string
		for _, r := range rows {
			rs = append(rs, `{"type":"table_row","content":[`+r+`]}`)
		}
		return `{"type":"doc","content":[{"type":"table"` + attrs + `,"content":[` + strings.Join(rs, ",") + `]}]}`
	}
	c := cell("", "")
	bad := map[string]string{
		"borders":         tbl(`,"attrs":{"borders":"dotted"}`, c),
		"align":           tbl(`,"attrs":{"align":"middle"}`, c),
		"width":           tbl(`,"attrs":{"width":2}`, c),
		"width text":      tbl(`,"attrs":{"width":"half"}`, c),
		"no rows":         `{"type":"doc","content":[{"type":"table","content":[]}]}`,
		"ragged":          tbl("", c+","+c, c),
		"rowspan too far": tbl("", cell(`,"attrs":{"rowspan":2}`, "")),
		"overlap":         tbl("", cell(`,"attrs":{"rowspan":2}`, "")+","+c, c+","+c),
		"colspan 0":       tbl("", cell(`,"attrs":{"colspan":0}`, "")),
		"colwidth length": tbl("", cell(`,"attrs":{"colspan":2,"colwidth":[10]}`, "")),
		"colwidth value":  tbl("", cell(`,"attrs":{"colwidth":[-3]}`, "")),
		"background":      tbl("", cell(`,"attrs":{"background":"red"}`, "")),
		"valign":          tbl("", cell(`,"attrs":{"valign":"baseline"}`, "")),
		"heading in cell": tbl("", cell("", `[{"type":"heading","attrs":{"level":1}}]`)),
		"empty cell":      tbl("", cell("", "[]")),
		"nested":          tbl("", cell("", `[`+strings.TrimSuffix(strings.TrimPrefix(tbl("", c), `{"type":"doc","content":[`), `]}`)+`]`)),
		"row in doc":      `{"type":"doc","content":[{"type":"table_row","content":[]}]}`,
		"in a list":       `{"type":"doc","content":[{"type":"bullet_list","content":[{"type":"list_item","content":[{"type":"paragraph"},` + strings.TrimSuffix(strings.TrimPrefix(tbl("", c), `{"type":"doc","content":[`), `]}`) + `]}]}]}`,
		"cell in table":   `{"type":"doc","content":[{"type":"table","content":[` + c + `]}]}`,
		"64 columns":      tbl("", strings.TrimSuffix(strings.Repeat(c+",", 64), ",")),
	}
	for name, src := range bad {
		if _, err := parsePMDoc([]byte(src)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	good := map[string]string{
		"plain":           tbl("", c),
		"63 columns":      tbl("", strings.TrimSuffix(strings.Repeat(c+",", 63), ",")),
		"covered row":     tbl("", cell(`,"attrs":{"rowspan":2}`, ""), ""),
		"spans":           tbl(`,"attrs":{"borders":"none","width":120.5}`, cell(`,"attrs":{"colspan":2,"colwidth":[0,30.5]}`, ""), c+","+c),
		"header and list": tbl("", `{"type":"table_header","content":[{"type":"ordered_list","attrs":{"start":1},"content":[{"type":"list_item","content":[{"type":"paragraph"}]}]}]}`),
	}
	for name, src := range good {
		if _, err := parsePMDoc([]byte(src)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// A Word table: Table Grid style (borders, single spacing, no space
// after), a header row, gridSpan, vMerge, shading, vertical alignment,
// centred, 100% wide, a heading and a nested table in cells, gridBefore.
func TestDocxTableImport(t *testing.T) {
	styles := `<w:docDefaults><w:pPrDefault><w:pPr><w:spacing w:after="160" w:line="259" w:lineRule="auto"/></w:pPr></w:pPrDefault>` +
		`<w:rPrDefault><w:rPr><w:sz w:val="22"/></w:rPr></w:rPrDefault></w:docDefaults>` +
		`<w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/><w:pPr><w:ind w:firstLine="284"/><w:jc w:val="both"/></w:pPr></w:style>` +
		`<w:style w:type="paragraph" w:styleId="Heading1"><w:name w:val="heading 1"/><w:basedOn w:val="Normal"/><w:pPr><w:outlineLvl w:val="0"/></w:pPr></w:style>` +
		`<w:style w:type="table" w:default="1" w:styleId="TableNormal"><w:name w:val="Normal Table"/></w:style>` +
		`<w:style w:type="table" w:styleId="TableGrid"><w:name w:val="Table Grid"/><w:basedOn w:val="TableNormal"/>` +
		`<w:pPr><w:spacing w:after="0" w:line="240" w:lineRule="auto"/></w:pPr>` +
		`<w:tblPr><w:tblBorders><w:top w:val="single" w:sz="4"/><w:left w:val="single" w:sz="4"/><w:bottom w:val="single" w:sz="4"/>` +
		`<w:right w:val="single" w:sz="4"/><w:insideH w:val="single" w:sz="4"/><w:insideV w:val="single" w:sz="4"/></w:tblBorders></w:tblPr></w:style>`
	tc := func(tcPr string, content ...string) string {
		if len(content) == 0 {
			content = []string{wp("")}
		}
		return `<w:tc><w:tcPr>` + tcPr + `</w:tcPr>` + strings.Join(content, "") + `</w:tc>`
	}
	nested := `<w:tbl><w:tblPr/><w:tblGrid><w:gridCol w:w="1000"/></w:tblGrid><w:tr>` + tc("", wp("", wr("", "inner"))) + `</w:tr></w:tbl>`
	body := wp(`<w:pStyle w:val="Heading1"/>`, wr("", "Chapter")) +
		`<w:tbl><w:tblPr><w:tblStyle w:val="TableGrid"/><w:tblW w:w="5000" w:type="pct"/><w:jc w:val="center"/></w:tblPr>` +
		`<w:tblGrid><w:gridCol w:w="2000"/><w:gridCol w:w="3000"/><w:gridCol w:w="1500"/></w:tblGrid>` +
		`<w:tr><w:trPr><w:tblHeader/></w:trPr>` + tc(`<w:gridSpan w:val="2"/>`, wp("", wr("<w:b/>", "Name"))) + tc("", wp("", wr("", "Lv"))) + `</w:tr>` +
		`<w:tr>` + tc(`<w:vMerge w:val="restart"/><w:shd w:val="clear" w:fill="D9D9D9"/><w:vAlign w:val="center"/>`, wp("", wr("", "Mira"))) +
		tc("", wp(`<w:pStyle w:val="Heading1"/>`, wr("", "Not a chapter"))) + tc("", wp(`<w:spacing w:after="120"/>`, wr("", "12"))) + `</w:tr>` +
		`<w:tr>` + tc(`<w:vMerge/>`) + tc("", wp("", wr("", "around")), nested) + tc("") + `</w:tr>` +
		`<w:tr><w:trPr><w:tblHeader/></w:trPr>` + tc("", wp("", wr("", "late header"))) + tc(`<w:gridSpan w:val="2"/>`) + `</w:tr>` +
		`<w:tr><w:trPr><w:gridBefore w:val="1"/></w:trPr>` + tc("", wp("", wr("", "shifted"))) + `</w:tr>` +
		`</w:tbl>` + wp("", wr("", "After."))
	imp := importDocx(t, docxSpec{body: body, styles: styles})
	if len(imp.Chapters) != 1 {
		t.Fatalf("%d chapters (a heading in a cell must not start one)", len(imp.Chapters))
	}
	doc := imp.Chapters[0].Doc
	if got := types(doc.Content); got != "heading,table,paragraph" {
		t.Fatalf("blocks %s", got)
	}
	tb := doc.Content[1]
	if tableBorders(tb) != "all" || tableAlign(tb) != "center" {
		t.Errorf("table attrs %v", tb.Attrs)
	}
	if full, _ := tableWidth(tb); !full {
		t.Errorf("a 100%% table is full width: %v", tb.Attrs)
	}
	g, err := tableGridOf(tb)
	if err != nil {
		t.Fatal(err)
	}
	if g.rows != 5 || g.cols != 3 {
		t.Fatalf("grid %dx%d", g.rows, g.cols)
	}
	if ws := g.colWidths(); fmt.Sprint(ws) != "[100 150 75]" {
		t.Errorf("column widths %v", ws)
	}
	at := func(r, c int) pmGridCell { return g.cells[g.at[r][c]] }
	name := at(0, 0)
	if name.colspan != 2 || name.node.Type != "table_header" || at(0, 2).node.Type != "table_header" {
		t.Errorf("header row: %+v", name)
	}
	if late := at(3, 0); late.node.Type != "table_cell" {
		t.Error("a header row after ordinary rows is an ordinary row")
	}
	mira := at(1, 0)
	if mira.rowspan != 2 || at(2, 0).node != mira.node || cellBackground(mira.node) != "#d9d9d9" || cellValign(mira.node) != "middle" {
		t.Errorf("merged cell %+v %v", mira, mira.node.Attrs)
	}
	heading := at(1, 1).node.Content[0]
	if heading.Type != "paragraph" || nodeText(heading) != "Not a chapter" {
		t.Errorf("a heading in a cell: %s %q", heading.Type, nodeText(heading))
	}
	// Cell paragraphs: Table Grid's spacing (0 after: the cell default,
	// not written), Normal's first-line indent as Word shows it in a cell
	// (written: a cell's default is 0); a paragraph's own space after kept.
	first := mira.node.Content[0]
	for _, k := range []string{"spaceAfter", "spaceBefore"} {
		if _, ok := first.Attrs[k]; ok {
			t.Errorf("cell paragraph carries %s: %v", k, first.Attrs)
		}
	}
	if v, _ := attrNum(first, "indentFirst"); v != 14.2 {
		t.Errorf("Normal's indent in a cell: %v", first.Attrs)
	}
	if lh, _ := attrNum(first, "lineHeight"); lh != 1 {
		t.Errorf("Table Grid's single spacing: %v", first.Attrs)
	}
	if sa, _ := attrNum(at(1, 2).node.Content[0], "spaceAfter"); sa != 6 {
		t.Errorf("own space after: %v", at(1, 2).node.Content[0].Attrs)
	}
	// The nested table is flattened into its cell, and counted.
	around := at(2, 1).node
	if docText(around) != "around\ninner" || imp.LeftOut["tables"] != 1 {
		t.Errorf("nested: %q, left out %v", docText(around), imp.LeftOut)
	}
	// gridBefore: an empty cell first; a short row filled to the grid.
	if docText(at(4, 0).node) != "" || docText(at(4, 1).node) != "shifted" || at(4, 2).node == at(4, 1).node {
		t.Errorf("row with gridBefore: %q %q", docText(at(4, 0).node), docText(at(4, 1).node))
	}
	// Normal's own formatting outside the table is untouched by cells.
	normal := styleByID(imp, "normal")
	if v, _ := normal.Para["spaceAfter"].(float64); v != 8 {
		t.Errorf("normal's space after %v (cells must not shape it)", normal.Para)
	}
	if imp.Chapters[0].Words != docWords(doc) || docWords(doc) != 14 {
		t.Errorf("words %d / %d", imp.Chapters[0].Words, docWords(doc))
	}
}

func TestDocxTableBorders(t *testing.T) {
	cases := map[string]struct{ tblPr, tcPr, want string }{
		"none":         {``, ``, "none"},
		"outer":        {`<w:tblBorders><w:top w:val="single"/><w:bottom w:val="double"/><w:start w:val="single"/><w:end w:val="single"/><w:insideH w:val="nil"/></w:tblBorders>`, ``, "outer"},
		"all":          {`<w:tblBorders><w:insideH w:val="single"/></w:tblBorders>`, ``, "all"},
		"cell borders": {``, `<w:tcBorders><w:top w:val="single"/><w:bottom w:val="single"/></w:tcBorders>`, "all"},
		"cells none":   {``, `<w:tcBorders><w:top w:val="nil"/></w:tcBorders>`, "none"},
	}
	for name, c := range cases {
		body := `<w:tbl><w:tblPr>` + c.tblPr + `</w:tblPr><w:tblGrid><w:gridCol w:w="1000"/><w:gridCol w:w="1000"/></w:tblGrid><w:tr>` +
			`<w:tc><w:tcPr>` + c.tcPr + `</w:tcPr>` + wp("", wr("", "a")) + `</w:tc><w:tc><w:tcPr>` + c.tcPr + `</w:tcPr>` + wp("", wr("", "b")) + `</w:tc></w:tr></w:tbl>`
		imp := importDocx(t, docxSpec{body: body})
		tb := allBlocks(imp)[0]
		if tb.Type != "table" || tableBorders(tb) != c.want {
			t.Errorf("%s: %s borders %s, want %s", name, tb.Type, tableBorders(tb), c.want)
		}
	}
}

func TestTableExports(t *testing.T) {
	b, _ := testBook(t)
	// .docx: the Word table's parts.
	data, err := exportDocx(b)
	if err != nil {
		t.Fatal(err)
	}
	files, _ := unzip(t, data)
	doc := string(files["word/document.xml"])
	for _, want := range []string{`<w:tblHeader/>`, `<w:gridSpan w:val="2"/>`, `<w:vMerge w:val="restart"/>`, `<w:vMerge/>`,
		`<w:shd w:val="clear" w:color="auto" w:fill="D9D9D9"/>`, `<w:vAlign w:val="center"/>`, `<w:jc w:val="center"/>`,
		`<w:gridCol w:w="1600"/><w:gridCol w:w="2400"/>`, `<w:insideH w:val="nil"/>`, `<w:tblLayout w:type="fixed"/>`} {
		if !strings.Contains(doc, want) {
			t.Errorf("document.xml lacks %s", want)
		}
	}
	// Normal cell paragraphs get the cell defaults written out.
	if !strings.Contains(doc, `<w:spacing w:before="0" w:after="0"/><w:ind w:firstLine="0"/></w:pPr><w:r><w:t xml:space="preserve">Strength`) {
		i := strings.Index(doc, ">Strength")
		t.Errorf("cell defaults not written: %s", doc[max(0, i-300):i])
	}

	// EPUB.
	ep, err := exportEPUB(b)
	if err != nil {
		t.Fatal(err)
	}
	ef, _ := unzip(t, ep)
	x := string(ef["OEBPS/ch004.xhtml"])
	wellFormed(t, "ch004.xhtml", ef["OEBPS/ch004.xhtml"])
	for _, want := range []string{`<table class="tb tb-outer" style="width: 77.5%; margin-left: auto; margin-right: auto">`,
		`<col style="width: 40%"/><col style="width: 60%"/>`, "<thead>\n<tr><th>", `<td rowspan="2">`, `<td colspan="2">`,
		`style="background-color: #d9d9d9; vertical-align: middle"`, `<table class="tb tb-none" style="width: 100%">`} {
		if !strings.Contains(x, want) {
			t.Errorf("EPUB chapter lacks %s\n%s", want, x)
		}
	}
	if css := string(ef["OEBPS/style.css"]); !strings.Contains(css, "table.tb td > p, table.tb th > p { text-indent: 0") {
		t.Error("EPUB CSS lacks the table rules")
	}

	// HTML for AO3.
	h, _ := exportHTML(b)
	html := string(h)
	for _, want := range []string{"<table>\n<thead>\n<tr><th><strong>Stat</strong></th><th>Value</th></tr>\n</thead>\n<tbody>",
		`<td rowspan="2">` + "\n<p>12</p>\n<ul>\n<li>raw</li>\n</ul>\n</td>", `<td colspan="2">Skills: Sword, Bow</td>`,
		"<tr><td>Level</td><td>12<br>(max)</td></tr>\n</tbody>\n</table>"} {
		if !strings.Contains(html, want) {
			t.Errorf("HTML lacks %q", want)
		}
	}

	// Markdown: a pipe table for the simple one, HTML for the other.
	md, _ := exportMarkdown(b)
	mds := string(md)
	if !strings.Contains(mds, "| Name | Mira \\| Vey |\n| --- | --- |\n| Level | 12<br>(max) |") {
		t.Errorf("Markdown pipe table:\n%s", mds)
	}
	if !strings.Contains(mds, "<table>\n<thead>") {
		t.Error("Markdown: the stat block should be an HTML table")
	}

	// Text: columns lined up.
	tx, _ := exportText(b)
	txt := string(tx)
	for _, want := range []string{"Stat      Value\n------------------\nStrength  12\n          • raw\nAgility\nSkills: Sword, Bow",
		"Name   Mira | Vey\nLevel  12\n       (max)"} {
		if !strings.Contains(txt, want) {
			t.Errorf("text lacks %q in\n%s", want, txt[strings.Index(txt, "Short."):])
		}
	}
}
