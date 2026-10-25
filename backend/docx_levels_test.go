package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// Character styles and nested lists in the .docx import (docs/EDITOR.md).

const levelsNumbering = `<w:abstractNum w:abstractNumId="0">` +
	`<w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="bullet"/><w:lvlText w:val="•"/></w:lvl>` +
	`<w:lvl w:ilvl="1"><w:start w:val="1"/><w:numFmt w:val="bullet"/><w:lvlText w:val="o"/></w:lvl>` +
	`<w:lvl w:ilvl="2"><w:start w:val="1"/><w:numFmt w:val="bullet"/><w:lvlText w:val="-"/></w:lvl>` +
	`<w:lvl w:ilvl="3"><w:start w:val="1"/><w:numFmt w:val="bullet"/><w:lvlText w:val="-"/></w:lvl></w:abstractNum>` +
	`<w:abstractNum w:abstractNumId="1">` +
	`<w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="decimal"/><w:lvlText w:val="%1."/></w:lvl>` +
	`<w:lvl w:ilvl="1"><w:start w:val="1"/><w:numFmt w:val="lowerLetter"/><w:lvlText w:val="%2)"/></w:lvl>` +
	`<w:lvl w:ilvl="2"><w:start w:val="1"/><w:numFmt w:val="lowerRoman"/><w:lvlText w:val="%3."/></w:lvl></w:abstractNum>` +
	`<w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num>` +
	`<w:num w:numId="2"><w:abstractNumId w:val="1"/></w:num>`

// nestedListShape writes a list as text: [item [nested…] item]; "#" in front of
// an ordered list, with its start when it isn't 1.
func nestedListShape(n *pmNode) string {
	var b strings.Builder
	if n.Type == "ordered_list" {
		b.WriteString("#")
		if s, _ := pmInt(n.Attrs["start"]); s != 1 {
			b.WriteString(string(rune('0' + s)))
		}
	}
	b.WriteString("[")
	for i, it := range n.Content {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(nodeText(it.Content[0]))
		for _, c := range it.Content[1:] {
			b.WriteString(" " + nestedListShape(c))
		}
	}
	b.WriteString("]")
	return b.String()
}

func TestDocxNestedLists(t *testing.T) {
	item := func(id, lvl int, text string) string { return wp(numPr(id, lvl), wr("", text)) }
	body := item(2, 0, "a") + item(2, 1, "a1") + item(2, 2, "a1i") + item(2, 2, "a1ii") + item(2, 1, "a2") + item(2, 0, "b") +
		wp("", wr("", "text between")) +
		item(1, 1, "deep first") + item(1, 0, "then back") + item(1, 2, "skips a level") + item(1, 1, "and up") +
		wp("", wr("", "more text")) +
		item(2, 0, "x") + item(1, 1, "bullets under numbers") + item(2, 1, "letters again")
	imp := importDocx(t, docxSpec{body: body, numbering: levelsNumbering})
	bs := imp.Chapters[0].Doc.Content
	if got := types(bs); got != "ordered_list,paragraph,bullet_list,paragraph,ordered_list" {
		t.Fatalf("blocks: %s", got)
	}
	if got := nestedListShape(bs[0]); got != "#[a #[a1 #[a1i a1ii] a2] b]" {
		t.Errorf("numbered: %s", got)
	}
	// A list that starts one level in opens there; coming back out to level 0
	// is a new item of the same list, and a skipped level still nests.
	if got := nestedListShape(bs[2]); got != "[deep first then back [skips a level] [and up]]" {
		t.Errorf("bullets: %s", got)
	}
	// Another kind of list inside an item. (The count of the first list
	// goes on: "x" is item 3 of numId 2.)
	if got := nestedListShape(bs[4]); got != "#3[x [bullets under numbers] #[letters again]]" {
		t.Errorf("mixed: %s", got)
	}
}

func TestDocxNestedListsStopAtNineLevels(t *testing.T) {
	var abs strings.Builder
	abs.WriteString(`<w:abstractNum w:abstractNumId="0">`)
	for i := 0; i < 10; i++ {
		abs.WriteString(`<w:lvl w:ilvl="` + string(rune('0'+i)) + `"><w:start w:val="1"/><w:numFmt w:val="bullet"/><w:lvlText w:val="-"/></w:lvl>`)
	}
	abs.WriteString(`</w:abstractNum><w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num>`)
	var body strings.Builder
	for i := 0; i < 10; i++ {
		body.WriteString(wp(numPr(1, i), wr("", "level")))
	}
	imp := importDocx(t, docxSpec{body: body.String(), numbering: abs.String()})
	levels := 1
	n := imp.Chapters[0].Doc.Content[0]
	for len(n.Content[len(n.Content)-1].Content) > 1 {
		item := n.Content[len(n.Content)-1]
		n = item.Content[1]
		levels++
	}
	if levels != 9 {
		t.Errorf("%d levels, want 9", levels)
	}
	// The tenth Word level (ilvl 9 does not exist; it was clamped to 8) is
	// an item of the ninth list. The document passes the server's checks.
	b, _ := json.Marshal(imp.Chapters[0].Doc)
	if _, err := parsePMDoc(b); err != nil {
		t.Errorf("nine levels: %v", err)
	}
}

const charStyles = `<w:docDefaults><w:rPrDefault><w:rPr><w:rFonts w:ascii="Georgia" w:hAnsi="Georgia"/><w:sz w:val="22"/></w:rPr></w:rPrDefault></w:docDefaults>` +
	`<w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/></w:style>` +
	`<w:style w:type="character" w:default="1" w:styleId="DefaultParagraphFont"><w:name w:val="Default Paragraph Font"/></w:style>` +
	`<w:style w:type="character" w:styleId="Loud"><w:name w:val="Loud"/><w:rPr><w:b/><w:color w:val="C00000"/></w:rPr></w:style>` +
	`<w:style w:type="character" w:styleId="LouderStill"><w:name w:val="Louder Still"/><w:basedOn w:val="Loud"/><w:rPr><w:rFonts w:ascii="Impact" w:hAnsi="Impact"/><w:sz w:val="32"/></w:rPr></w:style>`

func TestDocxCharacterStylesAreMarks(t *testing.T) {
	body := wp("", wr("", "plain "), wr(`<w:rStyle w:val="Loud"/>`, "loud"), wr("", " "),
		// Direct formatting on top of the style: italic, and a size the style doesn't set.
		wr(`<w:rStyle w:val="Loud"/><w:i/><w:sz w:val="40"/>`, "loud and italic"), wr("", " "),
		// A colour of its own beats the style's.
		wr(`<w:rStyle w:val="Loud"/><w:color w:val="0000FF"/>`, "blue"), wr("", " "),
		// A style based on another: both give their look.
		wr(`<w:rStyle w:val="LouderStill"/>`, "louder"), wr("", " "),
		// Direct formatting alone.
		wr(`<w:b/>`, "bold"))
	imp := importDocx(t, docxSpec{body: body, styles: charStyles})
	marks := func(s string) map[string]map[string]any { return markMap(textNode(t, imp, s)) }

	if m := marks("plain"); len(m) != 0 {
		t.Errorf("plain: %v", m)
	}
	if m := marks("loud"); len(m) != 1 || m["char_style"]["id"] != "word-loud" {
		t.Errorf("loud: %v", m)
	}
	if m := marks("loud and italic"); len(m) != 3 || m["char_style"]["id"] != "word-loud" || m["italic"] == nil || m["size"]["pt"] != 20.0 || m["bold"] != nil || m["color"] != nil {
		t.Errorf("loud and italic: %v", m)
	}
	if m := marks("blue"); len(m) != 2 || m["char_style"]["id"] != "word-loud" || m["color"]["color"] != "#0000ff" {
		t.Errorf("blue: %v", m)
	}
	if m := marks("louder"); len(m) != 1 || m["char_style"]["id"] != "word-louderstill" {
		t.Errorf("louder: %v", m)
	}
	if m := marks("bold"); len(m) != 1 || m["bold"] == nil {
		t.Errorf("bold: %v", m)
	}
	// The style mark comes first, so a reader draws the direct marks on top of it.
	if ms := textNode(t, imp, "loud and italic").Marks; ms[0].Type != "char_style" {
		t.Errorf("mark order: %v", ms)
	}

	loud, louder := styleByID(imp, "word-loud"), styleByID(imp, "word-louderstill")
	js := func(v any) string { b, _ := json.Marshal(v); return string(b) }
	if loud == nil || loud.Kind != "character" || js(loud.Text) != `{"bold":true,"color":"#c00000"}` {
		t.Errorf("loud style: %+v", loud)
	}
	if louder == nil || louder.BasedOn == nil || *louder.BasedOn != "word-loud" || js(louder.Text) != `{"font":"Impact","size":16}` {
		t.Errorf("louder style: %+v", louder)
	}
}

func TestDocxCharacterStyleOnTopOfAParagraphStyle(t *testing.T) {
	// Text bold in its paragraph style, and in a character style that asks
	// for bold again, carries just the character style.
	styles := charStyles +
		`<w:style w:type="paragraph" w:styleId="Shouty"><w:name w:val="Shouty"/><w:basedOn w:val="Normal"/><w:rPr><w:b/></w:rPr></w:style>`
	body := wp(`<w:pStyle w:val="Shouty"/>`, wr("", "all "), wr(`<w:rStyle w:val="Loud"/>`, "loud"))
	imp := importDocx(t, docxSpec{body: body, styles: styles})
	if m := markMap(textNode(t, imp, "loud")); len(m) != 1 || m["char_style"]["id"] != "word-loud" {
		t.Errorf("loud: %v", m)
	}
	if m := markMap(textNode(t, imp, "all")); len(m) != 0 {
		t.Errorf("all: %v", m)
	}
}
